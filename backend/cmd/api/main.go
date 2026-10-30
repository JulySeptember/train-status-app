// @title			Train Status API
// @version		1.0
// @description	Tokyo railway status, timetable and journey API (ODPT open data)
// @BasePath		/
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"

	_ "train-status-app/backend/docs"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/awssetup"
	"train-status-app/backend/internal/ai/gemini"
	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/auth/dynamostore"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/config"
	"train-status-app/backend/internal/favorite"
	"train-status-app/backend/internal/handler"
	"train-status-app/backend/internal/middleware"
	"train-status-app/backend/internal/notify"
	"train-status-app/backend/internal/odptkey"
	"train-status-app/backend/internal/router"
	"train-status-app/backend/internal/service"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func main() {
	// 起動時に読み込む時刻表などはほとんどが最後まで残るので、読み込み中の GC は回収できるものが少ない。
	// 読み込みが終わるまで GC を止めて、コールドスタートを短くする（#65）
	gcPercent := debug.SetGCPercent(-1)

	cfg := config.Load()

	c := client.New(odptSources(cfg.ODPTOperators), odptKeys(cfg, isLambda()))

	// 通知用の Lambda（EventBridge から5分ごとに呼ばれる）は、運行情報と路線名しか使わないので、
	// 時刻表を読み込まず、経路探索のエンジンなども作らない
	notifyMode := cfg.AppMode == "notify"

	// assets/extra に他社のデータがあれば読み込む。本番のビルド（make backend-build）は S3 から取って埋め込む。
	// 無ければ（CI のテストなど）都営だけで動く
	assetOpts := []assets.Option{assets.WithExtra()}
	if notifyMode {
		assetOpts = append(assetOpts, assets.WithoutTimetables())
	}

	loader, err := assets.New(assetOpts...)
	if err != nil {
		log.Fatal(err)
	}

	if loader.HasExtra() {
		log.Printf("loaded other operators' assets from assets/extra")
	}

	var svc *service.Service
	if notifyMode {
		svc = service.NewConditions(c, loader)
	} else {
		svc = service.New(c, loader)
	}

	debug.SetGCPercent(gcPercent)

	onLambda := isLambda()

	store := newUserStore(cfg, onLambda)
	keys := vapidKeys(cfg, onLambda)

	// 手元では1回だけ確かめて終わる
	if notifyMode {
		runNotifier(store, keys, svc, onLambda)
		return
	}

	h := handler.New(svc, newChat(cfg, svc, onLambda))

	if cfg.GoogleClientID == "" || store == nil {
		log.Printf("login is disabled (GOOGLE_CLIENT_ID or USERS_TABLE is not set)")
	} else {
		// Lambda は CloudFront の HTTPS の後ろで動く
		h.WithAuth(auth.NewService(auth.NewGoogleVerifier(cfg.GoogleClientID), store), cfg.GoogleClientID, onLambda)
		h.WithFavorites(favorite.NewService(store, svc))
		h.WithNotify(notify.NewService(store, svc, keys, pushSubject))
	}

	r := router.New(h)

	var app http.Handler = r

	// Recovery を内側に置き、panic したリクエストも Logging がログに出す
	app = middleware.Recovery(app)
	app = middleware.Logging(app)

	// AWS Lambda
	if onLambda {
		adapter := httpadapter.NewV2(app)
		lambda.Start(adapter.ProxyWithContext)
		return
	}

	// LocalのみCORSを有効化
	app = middleware.CORS(app)

	addr := ":" + cfg.Port

	log.Printf("Server started on %s", addr)

	if err := http.ListenAndServe(addr, app); err != nil {
		log.Fatal(err)
	}
}

// newChat は AI エージェントを組み立てる。使えないときは nil を返し、/api/chat は 503 になる。
// Lambda では API キーを SSM から読み、利用上限を DynamoDB で数える（/api/chat の初回に準備する）。
// 手元では環境変数の API キーを使い、利用上限をメモリ上で数える。
func newChat(cfg config.Config, svc *service.Service, onLambda bool) handler.ChatService {

	limits := ai.DefaultLimits()
	if cfg.AILimitPerIPPerMinute > 0 {
		limits.PerIPPerMinute = cfg.AILimitPerIPPerMinute
	}
	if cfg.AILimitPerIPPerDay > 0 {
		limits.PerIPPerDay = cfg.AILimitPerIPPerDay
	}
	if cfg.AILimitCallsPerDay > 0 {
		limits.CallsPerDay = cfg.AILimitCallsPerDay
	}

	model := ai.ResolveModel(cfg.AIModel)
	tools := ai.NewTools(svc)

	if onLambda {
		if cfg.AIKeyParameter == "" || cfg.AIUsageTable == "" {
			log.Printf("AI agent is disabled (AI_API_KEY_PARAMETER or AI_USAGE_TABLE is not set)")
			return nil
		}
		return awssetup.New(awssetup.Config{
			KeyParameter: cfg.AIKeyParameter,
			UsageTable:   cfg.AIUsageTable,
			Model:        model,
			Limits:       limits,
		}, tools)
	}

	if cfg.GeminiAPIKey == "" {
		log.Printf("AI agent is disabled (GEMINI_API_KEY is not set)")
		return nil
	}

	var opts []gemini.Option
	if cfg.GeminiBaseURL != "" {
		opts = append(opts, gemini.WithBaseURL(cfg.GeminiBaseURL))
	}

	log.Printf("AI agent is enabled (model %s)", model)

	return ai.NewService(
		gemini.New(cfg.GeminiAPIKey, opts...),
		model,
		tools,
		ai.NewMemoryLimiter(limits),
		ai.DefaultConfig(),
	)
}

// pushSubject は、push サービスが送り手に連絡するための URL（VAPID の sub）
const pushSubject = "https://github.com/JulySeptember/train-status-app"

// userStore は、利用者・セッション・お気に入り・通知の保存先
type userStore interface {
	auth.Store
	favorite.Store
	notify.Store
}

// newUserStore は、利用者の保存先を作る。Lambda では DynamoDB に、手元ではメモリ上に保存する（再起動で消える）。
// Lambda でテーブルが設定されていなければ nil を返し、ログイン・お気に入り・通知の API は 503 になる
func newUserStore(cfg config.Config, onLambda bool) userStore {

	if cfg.UsersTable == "" {
		if onLambda {
			return nil
		}
		log.Printf("users and sessions are kept in memory")
		return auth.NewMemoryStore()
	}

	// 設定を読むだけで、AWS には接続しない
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Printf("users table is disabled (load aws config: %v)", err)
		return nil
	}

	return dynamostore.New(dynamodb.NewFromConfig(awsCfg), cfg.UsersTable)
}

// vapidKeys は、通知に使う VAPID の鍵を返す。Lambda では初めて使うときに SSM から、手元では環境変数から読む。
// 設定が無ければ nil を返し、購読と通知は使えない
func vapidKeys(cfg config.Config, onLambda bool) notify.Keys {

	if !onLambda {
		if cfg.VAPIDPrivateKey == "" {
			log.Printf("push notification is disabled (VAPID_PRIVATE_KEY is not set)")
			return nil
		}
		return notify.NewKeyLoader(func(context.Context) (string, error) { return cfg.VAPIDPrivateKey, nil })
	}

	if cfg.VAPIDKeyParameter == "" {
		log.Printf("push notification is disabled (VAPID_KEY_PARAMETER is not set)")
		return nil
	}

	return notify.NewKeyLoader(func(ctx context.Context) (string, error) {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return "", err
		}
		out, err := ssm.NewFromConfig(awsCfg).GetParameter(ctx, &ssm.GetParameterInput{
			Name:           aws.String(cfg.VAPIDKeyParameter),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			return "", fmt.Errorf("read vapid key: %w", err)
		}
		return aws.ToString(out.Parameter.Value), nil
	})
}

// runNotifier は、運行情報を確かめて通知を送る。Lambda では EventBridge から呼ばれるたびに、手元では1回だけ行う
func runNotifier(store userStore, keys notify.Keys, svc *service.Service, onLambda bool) {

	if store == nil {
		log.Fatal("notifier needs USERS_TABLE")
	}

	n := notify.NewNotifier(store, svc, svc, keys, pushSubject)

	if onLambda {
		lambda.Start(func(ctx context.Context) error { return n.Run(ctx) })
		return
	}

	if err := n.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func isLambda() bool {
	_, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")
	return ok
}

// odptSources は、リアルタイムの情報を取る事業者を返す。都営は常に含める。
func odptSources(operators []string) []client.Source {

	sources := []client.Source{client.Sources["Toei"]}

	seen := map[string]bool{"Toei": true}

	for _, name := range operators {
		s, ok := client.Sources[name]
		if !ok || seen[name] {
			log.Printf("ignore unknown or duplicate operator in ODPT_OPERATORS: %q", name)
			continue
		}
		seen[name] = true
		sources = append(sources, s)
	}

	return sources
}

// odptKeys は、ODPT のキーを返す関数を作る。Lambda では SSM から、手元では環境変数から読む。
func odptKeys(cfg config.Config, lambda bool) client.KeyFunc {
	if lambda {
		return odptkey.SSM(cfg.ODPTKeyParameter, cfg.ODPTChallengeKeyParameter)
	}
	return odptkey.Static(cfg.ODPTConsumerKey, cfg.ODPTChallengeConsumerKey)
}
