package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLog は、テストの間だけ標準の logger の出力を受け取る
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	})
	return &buf
}

func TestLoggingStatus(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{
			name:    "WriteHeader で返したステータス",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) },
			want:    "GET /api/status 429 ",
		},
		{
			name: "最初の WriteHeader だけを使う",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				w.WriteHeader(http.StatusOK)
			},
			want: "GET /api/status 502 ",
		},
		{
			name:    "Write だけなら 200",
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) },
			want:    "GET /api/status 200 ",
		},
		{
			name:    "何も書かなければ 200",
			handler: func(http.ResponseWriter, *http.Request) {},
			want:    "GET /api/status 200 ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)

			req := httptest.NewRequest(http.MethodGet, "/api/status?q=secret", nil)
			Logging(tt.handler).ServeHTTP(httptest.NewRecorder(), req)

			got := buf.String()
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf("log = %q, want prefix %q", got, tt.want)
			}
			if strings.Contains(got, "secret") {
				t.Errorf("log にクエリが出ている: %q", got)
			}
		})
	}
}

func TestLoggingKeepsFlusher(t *testing.T) {
	captureLog(t)

	h := Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush: %v", err)
		}
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !rec.Flushed {
		t.Error("元の ResponseWriter が Flush されていない")
	}
}

func TestRecoveryWithLogging(t *testing.T) {
	buf := captureLog(t)

	// main.go と同じ順に組む（Recovery が内側）
	h := Logging(Recovery(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/trains", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"internal server error"`) {
		t.Errorf("body = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	got := buf.String()
	for _, want := range []string{
		"panic: GET /api/trains: boom",
		"middleware.TestRecoveryWithLogging", // スタックに panic した関数が出る
		"GET /api/trains 500 ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log に %q が無い:\n%s", want, got)
		}
	}
}

func TestCORS(t *testing.T) {
	called := false
	h := CORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/status", nil))
	if rec.Code != http.StatusNoContent || called {
		t.Errorf("OPTIONS: status = %d, called = %v", rec.Code, called)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin = %q", got)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if !called {
		t.Error("GET が次のハンドラーに渡っていない")
	}
}
