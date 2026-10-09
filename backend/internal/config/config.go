package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port string

	// 手元で AI エージェントを動かすときの Gemini の API キー。
	// 本番ではキーを環境変数に置かず、SSM Parameter Store から読む（設計書 10.1）
	GeminiAPIKey string

	// Gemini の接続先。手元で Gemini を真似たサーバーに向けるときだけ使う
	GeminiBaseURL string

	// Lambda で使う、API キーを入れた SSM のパラメータ名と、利用上限を数える DynamoDB のテーブル名
	AIKeyParameter string
	AIUsageTable   string

	// 使うモデル。空なら ai.Models の先頭
	AIModel string

	// 利用上限。0 なら ai.DefaultLimits の値
	AILimitPerIPPerMinute int
	AILimitPerIPPerDay    int
	AILimitCallsPerDay    int

	// リアルタイムの情報を取る、都営以外の事業者（例: TokyoMetro,JR-East）。空なら都営だけ
	ODPTOperators []string

	// 手元で使う ODPT のキー（backend/.env）。本番ではキーを環境変数に置かず、SSM から読む
	ODPTConsumerKey          string
	ODPTChallengeConsumerKey string

	// Lambda で使う、ODPT のキーを入れた SSM のパラメータ名
	ODPTKeyParameter          string
	ODPTChallengeKeyParameter string
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
		AIKeyParameter:        os.Getenv("AI_API_KEY_PARAMETER"),
		AIUsageTable:          os.Getenv("AI_USAGE_TABLE"),
		AIModel:               os.Getenv("AI_MODEL"),
		AILimitPerIPPerMinute: intEnv("AI_LIMIT_PER_IP_PER_MINUTE"),
		AILimitPerIPPerDay:    intEnv("AI_LIMIT_PER_IP_PER_DAY"),
		AILimitCallsPerDay:    intEnv("AI_LIMIT_CALLS_PER_DAY"),

		ODPTOperators:             listEnv("ODPT_OPERATORS"),
		ODPTConsumerKey:           os.Getenv("ODPT_CONSUMER_KEY"),
		ODPTChallengeConsumerKey:  os.Getenv("ODPT_CHALLENGE_CONSUMER_KEY"),
		ODPTKeyParameter:          os.Getenv("ODPT_KEY_PARAMETER"),
		ODPTChallengeKeyParameter: os.Getenv("ODPT_CHALLENGE_KEY_PARAMETER"),
	}
}

// listEnv は、カンマ区切りの値を返す。空の要素は捨てる。
func listEnv(name string) []string {
	var result []string
	for _, v := range strings.Split(os.Getenv(name), ",") {
		if v = strings.TrimSpace(v); v != "" {
			result = append(result, v)
		}
	}
	return result
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
