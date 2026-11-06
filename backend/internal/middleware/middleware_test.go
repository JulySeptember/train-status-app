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
			// Lambda のアダプターは最後の WriteHeader の値を返すので、ログもそれに合わせる
			name: "最後の WriteHeader を使う",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.WriteHeader(http.StatusBadGateway)
			},
			want: "GET /api/status 502 ",
		},
		{
			name: "Write のあとの WriteHeader",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("ok"))
				w.WriteHeader(http.StatusInternalServerError)
			},
			want: "GET /api/status 500 ",
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

func TestLoggingEscapesPath(t *testing.T) {
	buf := captureLog(t)

	req := httptest.NewRequest(http.MethodGet, "/api/x%0AGET%20/fake", nil)
	Logging(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)

	got := buf.String()
	if strings.Count(got, "\n") != 1 || !strings.HasPrefix(got, "GET /api/x%0AGET%20/fake 200 ") {
		t.Errorf("パスの改行・空白がログの行を分けている: %q", got)
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

func TestRecoveryAfterWrite(t *testing.T) {
	buf := captureLog(t)

	// 200 を書いたあとに panic しても、ログは Recovery が書いた 500 になる（Lambda が返すのも 500）
	h := Logging(Recovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		panic("boom")
	})))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/trains", nil))

	if got := buf.String(); !strings.Contains(got, "GET /api/trains 500 ") {
		t.Errorf("log にステータス 500 が無い:\n%s", got)
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
