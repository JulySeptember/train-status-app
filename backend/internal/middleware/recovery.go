package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
)

// Recovery は、ハンドラーの panic を 500 に変え、原因を追えるようにスタックをログに出す
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		defer func() {
			if err := recover(); err != nil {

				// panic の値もログに出るので、ODPT のキーが入る *url.Error などを panic に渡さない
				log.Printf("panic: %s %s: %v\n%s", r.Method, r.URL.EscapedPath(), err, debug.Stack())

				w.Header().Set(
					"Content-Type",
					"application/json",
				)

				w.WriteHeader(
					http.StatusInternalServerError,
				)

				_ = json.NewEncoder(w).Encode(
					map[string]string{
						"error": "internal server error",
					},
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
