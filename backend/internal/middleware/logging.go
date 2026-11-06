package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logging は、リクエストごとにメソッド・パス・ステータスコード・処理時間を1行で出す。
// CloudWatch Logs Insights で 5xx・429・502 の割合を数えられるよう、ステータスコードを出す。
// AI への入力などの本文は出さない（パスだけで、クエリも出さない）
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		start := time.Now()

		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		log.Printf(
			"%s %s %d %s",
			r.Method,
			r.URL.Path,
			rec.Status(),
			time.Since(start),
		)
	})
}

// statusRecorder は、ハンドラーが返したステータスコードを覚える
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap は、http.ResponseController が元の ResponseWriter（Flush など）を使えるようにする
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Status は、ハンドラーが何も書かずに終わったときは 200 を返す（net/http と同じ）
func (r *statusRecorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
