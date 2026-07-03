package main

import (
	"log"
	"net/http"
	"os"

	"train-status-app/backend/assets"
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

	h := handler.New(svc)

	r := router.New(h)

	var app http.Handler = r

	app = middleware.Logging(app)
	app = middleware.Recovery(app)

	// AWS Lambda
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		adapter := httpadapter.New(app)
		lambda.Start(adapter.ProxyWithContext)
		return
	}

	// Local
	addr := ":" + cfg.Port

	log.Printf("Server started on %s", addr)

	if err := http.ListenAndServe(addr, app); err != nil {
		log.Fatal(err)
	}
}
