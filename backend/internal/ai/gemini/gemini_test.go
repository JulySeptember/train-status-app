package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"train-status-app/backend/internal/ai"
)

func newServer(t *testing.T, handler http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return New("test-key", WithBaseURL(srv.URL))
}

func reply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

var baseRequest = ai.Request{
	Model:    "gemini-test",
	System:   "system",
	Messages: []ai.Message{{Role: ai.RoleUser, Text: "春日から浅草"}},
	Tools: []ai.Tool{{
		Name:        "find_station",
		Description: "駅を探す",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	}},
	ToolChoice: ai.ToolChoiceAuto,
}

func TestGenerateText(t *testing.T) {

	var got map[string]any

	p := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-test:generateContent" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("api key is not sent in the header")
		}
		if r.URL.RawQuery != "" {
			t.Errorf("api key must not be sent in the URL: %s", r.URL.RawQuery)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)

		_, _ = io.WriteString(w, `{
			"candidates": [{"content": {"role": "model", "parts": [
				{"text": "考え中", "thought": true},
				{"text": "浅草線で"},
				{"text": "行けます。"}
			]}, "finishReason": "STOP"}],
			"usageMetadata": {"promptTokenCount": 100, "candidatesTokenCount": 20, "thoughtsTokenCount": 30}
		}`)
	})

	resp, err := p.Generate(t.Context(), baseRequest)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Text != "浅草線で行けます。" {
		t.Fatalf("unexpected text %q", resp.Text)
	}
	if resp.Usage.InputTokens != 100 || resp.Usage.OutputTokens != 50 {
		t.Fatalf("unexpected usage %+v", resp.Usage)
	}

	// 送った形式
	if got["systemInstruction"] == nil || got["tools"] == nil {
		t.Fatalf("system instruction or tools are missing: %v", got)
	}
	mode := got["toolConfig"].(map[string]any)["functionCallingConfig"].(map[string]any)["mode"]
	if mode != "AUTO" {
		t.Fatalf("unexpected mode %v", mode)
	}
	decl := got["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)[0].(map[string]any)
	if decl["parametersJsonSchema"] == nil {
		t.Fatalf("parameters are missing: %v", decl)
	}
}

func TestGenerateParallelToolCalls(t *testing.T) {

	p := newServer(t, reply(200, `{
		"candidates": [{"content": {"role": "model", "parts": [
			{"functionCall": {"id": "a", "name": "find_station", "args": {"name": "春日"}}, "thoughtSignature": "sig-1"},
			{"functionCall": {"id": "b", "name": "find_station", "args": {"name": "浅草"}}},
			{"functionCall": {"name": "get_train_status"}}
		]}}]
	}`))

	resp, err := p.Generate(t.Context(), baseRequest)
	if err != nil {
		t.Fatal(err)
	}

	if len(resp.ToolCalls) != 3 {
		t.Fatalf("expected 3 tool calls, got %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[1].ID != "b" || string(resp.ToolCalls[1].Arguments) != `{"name": "浅草"}` {
		t.Fatalf("unexpected tool call %+v", resp.ToolCalls[1])
	}
	// 引数の無い呼び出しは空のオブジェクトにする
	if string(resp.ToolCalls[2].Arguments) != "{}" {
		t.Fatalf("unexpected arguments %s", resp.ToolCalls[2].Arguments)
	}
	if !strings.Contains(string(resp.ProviderContent), "sig-1") {
		t.Fatalf("thought signature is not kept: %s", resp.ProviderContent)
	}
}

// 道具の呼び出しを含む履歴を送るとき、Gemini が返した内容（署名を含む）をそのまま送り返す
func TestGenerateSendsBackSignature(t *testing.T) {

	var got request

	p := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = io.WriteString(w, `{"candidates": [{"content": {"role": "model", "parts": [{"text": "OK"}]}}]}`)
	})

	providerContent := json.RawMessage(`{"role":"model","parts":[{"functionCall":{"id":"a","name":"find_station","args":{"name":"春日"}},"thoughtSignature":"sig-1"}]}`)

	req := baseRequest
	req.ToolChoice = ai.ToolChoiceNone
	req.Messages = []ai.Message{
		{Role: ai.RoleUser, Text: "春日"},
		{
			Role:            ai.RoleAssistant,
			ToolCalls:       []ai.ToolCall{{ID: "a", Name: "find_station", Arguments: json.RawMessage(`{"name":"春日"}`)}},
			ProviderContent: providerContent,
		},
		{
			Role:        ai.RoleTool,
			ToolResults: []ai.ToolResult{{CallID: "a", Name: "find_station", Content: json.RawMessage(`{"candidates":[]}`)}},
		},
	}

	if _, err := p.Generate(t.Context(), req); err != nil {
		t.Fatal(err)
	}

	if len(got.Contents) != 3 {
		t.Fatalf("unexpected contents %+v", got.Contents)
	}

	model := got.Contents[1]
	if model.Role != "model" || model.Parts[0].ThoughtSignature != "sig-1" {
		t.Fatalf("signature is not sent back: %+v", model)
	}

	toolResult := got.Contents[2]
	if toolResult.Role != "user" || toolResult.Parts[0].FunctionResponse == nil ||
		toolResult.Parts[0].FunctionResponse.ID != "a" || toolResult.Parts[0].FunctionResponse.Name != "find_station" {
		t.Fatalf("unexpected function response %+v", toolResult)
	}

	if got.ToolConfig.FunctionCallingConfig.Mode != "NONE" {
		t.Fatalf("unexpected mode %s", got.ToolConfig.FunctionCallingConfig.Mode)
	}
}

func TestGenerateErrors(t *testing.T) {

	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "invalid api key",
			handler: reply(400, `{"error": {"code": 400, "message": "API key not valid. key=secret", "status": "INVALID_ARGUMENT",
				"details": [{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": "API_KEY_INVALID"}]}}`),
			want: ai.ErrAuth,
		},
		{name: "forbidden", handler: reply(403, `{"error": {"code": 403, "status": "PERMISSION_DENIED"}}`), want: ai.ErrAuth},
		{name: "model not found", handler: reply(404, `{"error": {"code": 404, "status": "NOT_FOUND"}}`), want: ai.ErrInvalidModel},
		{name: "rate limited", handler: reply(429, `{"error": {"code": 429, "status": "RESOURCE_EXHAUSTED"}}`), want: ai.ErrRateLimited},
		{name: "internal", handler: reply(500, `{"error": {"code": 500, "status": "INTERNAL"}}`), want: ai.ErrUnavailable},
		{name: "unavailable", handler: reply(503, `oops`), want: ai.ErrUnavailable},
		{name: "bad request", handler: reply(400, `{"error": {"code": 400, "status": "INVALID_ARGUMENT"}}`), want: ai.ErrInvalidResponse},
		{name: "invalid json", handler: reply(200, `{"candidates": [`), want: ai.ErrInvalidResponse},
		{name: "blocked", handler: reply(200, `{"promptFeedback": {"blockReason": "SAFETY"}}`), want: ai.ErrInvalidResponse},
		{name: "empty", handler: reply(200, `{"candidates": [{"content": {"parts": []}, "finishReason": "MAX_TOKENS"}]}`), want: ai.ErrInvalidResponse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newServer(t, tt.handler)

			_, err := p.Generate(t.Context(), baseRequest)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			// Gemini のメッセージ（キーを含みうる）はエラーに入れない
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "test-key") {
				t.Fatalf("error contains sensitive text: %v", err)
			}
		})
	}
}

func TestGenerateTimeout(t *testing.T) {

	p := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if _, err := p.Generate(ctx, baseRequest); !errors.Is(err, ai.ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestGenerateConnectionError(t *testing.T) {
	p := New("test-key", WithBaseURL("http://127.0.0.1:1"))

	if _, err := p.Generate(t.Context(), baseRequest); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}
