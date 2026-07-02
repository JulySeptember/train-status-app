package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	// Lambda環境なら Lambda を起動
	if _, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		lambda.Start(func() (string, error) {
			return "OK", nil
		})
		return
	}

	// ローカル開発
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "OK")
	})

	fmt.Println("Server started on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
