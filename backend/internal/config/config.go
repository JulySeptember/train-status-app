package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port string

	// 手元で AI エージェントを動かすときの Gemini の API キー。
	// 本番ではキーを環境変数に置かず、SSM Parameter Store から読む（設計書 10.1）
	GeminiAPIKey string

	// Gemini の接続先。手元で Gemini を真似たサーバーに向けるときだけ使う
	GeminiBaseURL string

	// 使うモデル。空なら ai.Models の先頭
	AIModel string

	// 利用上限。0 なら ai.DefaultLimits の値
	AILimitPerIPPerMinute int
	AILimitPerIPPerDay    int
	AILimitCallsPerDay    int
}

func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		Port:                  port,
		GeminiAPIKey:          os.Getenv("GEMINI_API_KEY"),
		GeminiBaseURL:         os.Getenv("GEMINI_BASE_URL"),
		AIModel:               os.Getenv("AI_MODEL"),
		AILimitPerIPPerMinute: intEnv("AI_LIMIT_PER_IP_PER_MINUTE"),
		AILimitPerIPPerDay:    intEnv("AI_LIMIT_PER_IP_PER_DAY"),
		AILimitCallsPerDay:    intEnv("AI_LIMIT_CALLS_PER_DAY"),
	}
}

func intEnv(name string) int {
	v := os.Getenv(name)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		log.Printf("ignore invalid %s=%q", name, v)
		return 0
	}
	return n
}
