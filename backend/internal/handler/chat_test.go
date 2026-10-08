package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"train-status-app/backend/internal/ai"
)

type stubChat struct {
	res *ai.ChatResponse
	err error

	gotIP  string
	gotReq ai.ChatRequest
}

func (s *stubChat) Chat(_ context.Context, clientIP string, req ai.ChatRequest) (*ai.ChatResponse, error) {
	s.gotIP = clientIP
	s.gotReq = req
	return s.res, s.err
}

func postChat(h *Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Chat(rec, req)
	return rec
}

func TestChat(t *testing.T) {

	chat := &stubChat{res: &ai.ChatResponse{Reply: "大江戸線がおすすめです", Steps: []ai.Step{{Tool: "search_route", Label: "経路を検索"}}}}
	h := New(nil, chat)

	rec := postChat(h, `{"messages":[{"role":"user","text":"春日から浅草"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body)
	}

	var got ai.ChatResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Reply != "大江戸線がおすすめです" || len(got.Steps) != 1 {
		t.Fatalf("unexpected response %+v", got)
	}
	if chat.gotIP != "10.0.0.1" || chat.gotReq.Messages[0].Text != "春日から浅草" {
		t.Fatalf("unexpected request %q %+v", chat.gotIP, chat.gotReq)
	}
}

func TestChatDisabled(t *testing.T) {
	rec := postChat(New(nil, nil), `{"messages":[{"role":"user","text":"こんにちは"}]}`)

	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":"unavailable"`) {
		t.Fatalf("unexpected response %d %s", rec.Code, rec.Body)
	}
}

func TestChatInvalidBody(t *testing.T) {
	h := New(nil, &stubChat{})

	for _, body := range []string{`not json`, strings.Repeat("a", maxChatBodyBytes+1)} {
		if rec := postChat(h, body); rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	}
}

func TestChatErrors(t *testing.T) {

	tests := []struct {
		err    error
		status int
		code   string
	}{
		{ai.ErrInvalidRequest, 400, "invalid_request"},
		{ai.ErrAuth, 502, "auth"},
		{ai.ErrRateLimited, 429, "rate_limited"},
		{ai.ErrQuotaExceeded, 429, "quota_exceeded"},
		{ai.ErrUnavailable, 503, "unavailable"},
		{ai.ErrTimeout, 504, "timeout"},
		{ai.ErrInvalidModel, 500, "invalid_model"},
		{ai.ErrInvalidResponse, 502, "invalid_response"},
		{fmt.Errorf("unexpected"), 500, "internal"},
	}

	for _, tt := range tests {
		// 元のエラーメッセージ（AI サービス固有）は返さない
		err := fmt.Errorf("%w: provider says secret-detail", tt.err)
		rec := postChat(New(nil, &stubChat{err: err}), `{"messages":[{"role":"user","text":"?"}]}`)

		var got ChatError
		_ = json.NewDecoder(rec.Body).Decode(&got)

		if rec.Code != tt.status || got.Code != tt.code || got.Error == "" {
			t.Errorf("%v: got %d %+v, want %d %s", tt.err, rec.Code, got, tt.status, tt.code)
		}
		if strings.Contains(got.Error, "secret-detail") {
			t.Errorf("%v: response contains the provider message", tt.err)
		}
	}
}

func TestClientIP(t *testing.T) {

	tests := []struct {
		name      string
		remote    string
		forwarded []string
		want      string
	}{
		{"直接", "203.0.113.1:5000", nil, "203.0.113.1"},
		{"CloudFront 経由", "198.51.100.9", []string{"203.0.113.1"}, "203.0.113.1"},
		// 利用者が X-Forwarded-For を偽っても、CloudFront が末尾に足した値を使う
		{"偽の X-Forwarded-For", "198.51.100.9", []string{"1.1.1.1, 203.0.113.1"}, "203.0.113.1"},
		// API Gateway が接続元（CloudFront）の IP を末尾に足した場合
		{"接続元が末尾にある", "198.51.100.9", []string{"203.0.113.1, 198.51.100.9"}, "203.0.113.1"},
		{"ヘッダーが複数", "198.51.100.9", []string{"1.1.1.1", "203.0.113.1"}, "203.0.113.1"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		req.RemoteAddr = tt.remote
		for _, v := range tt.forwarded {
			req.Header.Add("X-Forwarded-For", v)
		}
		if got := clientIP(req); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got, tt.want)
		}
	}
}
