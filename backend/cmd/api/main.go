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
	"train-status-app/backend/internal/ai/gemini"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/config"
	"train-status-app/backend/internal/handler"
	"train-status-app/backend/internal/middleware"
	"train-status-app/backend/internal/router"
	"train-status-app/backend/internal/service"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func main() {
	cfg := config.Load()

	c := client.New()

	loader, err := assets.New()
	if err != nil {
		log.Fatal(err)
	}

	svc := service.New(c, loader)

	_, onLambda := os.LookupEnv("AWS_LAMBDA_RUNTIME_API")

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
// Lambda では、API キー（SSM）と利用上限の保存先（DynamoDB）を用意するまで使わない
// （メモリ上の利用上限はインスタンスごとにしか数えられないため）。
func newChat(cfg config.Config, svc *service.Service, onLambda bool) handler.ChatService {

	if onLambda || cfg.GeminiAPIKey == "" {
		log.Printf("AI agent is disabled")
		return nil
	}

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
	log.Printf("AI agent is enabled (model %s)", model)

	var opts []gemini.Option
	if cfg.GeminiBaseURL != "" {
		opts = append(opts, gemini.WithBaseURL(cfg.GeminiBaseURL))
	}

	return ai.NewService(
		gemini.New(cfg.GeminiAPIKey, opts...),
		model,
		ai.NewTools(svc),
		ai.NewMemoryLimiter(limits),
		ai.DefaultConfig(),
	)
}
