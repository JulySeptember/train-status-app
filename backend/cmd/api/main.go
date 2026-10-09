// @title			Train Status API
// @version		1.0
// @description	Toei Transportation API
// @BasePath		/
package main

import (
	"log"
	"net/http"
	"os"

	_ "train-status-app/backend/docs"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/awssetup"
	"train-status-app/backend/internal/ai/gemini"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/config"
	"train-status-app/backend/internal/handler"
	"train-status-app/backend/internal/middleware"
	"train-status-app/backend/internal/odptkey"
	"train-status-app/backend/internal/router"
	"train-status-app/backend/internal/service"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func main() {
	cfg := config.Load()

	c := client.New(odptSources(cfg.ODPTOperators), odptKeys(cfg, isLambda()))

	loader, err := assets.New()
	if err != nil {
		log.Fatal(err)
	}

	svc := service.New(c, loader)

	onLambda := isLambda()

	h := handler.New(svc, newChat(cfg, svc, onLambda))

	r := router.New(h)

	var app http.Handler = r

	app = middleware.Logging(app)
	app = middleware.Recovery(app)

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
