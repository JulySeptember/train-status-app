// Package gemini は Gemini API（REST の generateContent）の Provider。
// Gemini の形式はこのパッケージの外に出さず、ai の共通の形に変換する。
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"train-status-app/backend/internal/ai"
)

const DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

// maxResponseBytes は応答の大きさの上限
const maxResponseBytes = 4 << 20

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type Option func(*Provider)

// WithBaseURL は接続先を変える（テストで httptest.Server に向けるため）
func WithBaseURL(u string) Option {
	return func(p *Provider) { p.baseURL = strings.TrimSuffix(u, "/") }
}

func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		// 時間の上限は呼び出し側の context で決める
		client: &http.Client{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) Name() string { return "gemini" }

// =========================
// Gemini の形式
// =========================

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`

	// Gemini 3 では、道具の呼び出しに付いた署名を、同じ往復の中でそのまま送り返す必要がある
	ThoughtSignature string `json:"thoughtSignature,omitempty"`
}

type functionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type functionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type functionDeclaration struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

type toolConfig struct {
	FunctionCallingConfig struct {
		Mode string `json:"mode"`
	} `json:"functionCallingConfig"`
}

type request struct {
	SystemInstruction *content    `json:"systemInstruction,omitempty"`
	Contents          []content   `json:"contents"`
	Tools             []tool      `json:"tools,omitempty"`
	ToolConfig        *toolConfig `json:"toolConfig,omitempty"`
}

type response struct {
	Candidates []struct {
		Content      json.RawMessage `json:"content"`
		FinishReason string          `json:"finishReason"`
	} `json:"candidates"`

	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`

	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
}

type errorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Details []struct {
			Reason string `json:"reason"`
		} `json:"details"`
	} `json:"error"`
}

// =========================
// Generate
// =========================

func (p *Provider) Generate(ctx context.Context, req ai.Request) (ai.Response, error) {

	body, err := json.Marshal(buildRequest(req))
	if err != nil {
		return ai.Response{}, fmt.Errorf("%w: encode request: %v", ai.ErrInvalidResponse, err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent", p.baseURL, url.PathEscape(req.Model))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ai.Response{}, fmt.Errorf("%w: %v", ai.ErrInvalidModel, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		// エラーメッセージに URL が入るが、キーはヘッダーで送っているので含まれない
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return ai.Response{}, fmt.Errorf("%w: gemini: %v", ai.ErrTimeout, err)
		}
		return ai.Response{}, fmt.Errorf("%w: gemini: %v", ai.ErrUnavailable, err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, maxResponseBytes))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return ai.Response{}, fmt.Errorf("%w: gemini: read body: %v", ai.ErrTimeout, err)
		}
		return ai.Response{}, fmt.Errorf("%w: gemini: read body: %v", ai.ErrUnavailable, err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return ai.Response{}, statusError(httpResp.StatusCode, data)
	}

	return parseResponse(data)
}

// statusError は、Gemini のエラーを共通のエラーにする。Gemini のメッセージは含めない
func statusError(status int, body []byte) error {

	var e errorResponse
	_ = json.Unmarshal(body, &e)

	detail := fmt.Sprintf("gemini status %d %s", status, e.Error.Status)

	for _, d := range e.Error.Details {
		// API キーが不正なときは 400 で返る
		if d.Reason == "API_KEY_INVALID" {
			return fmt.Errorf("%w: %s", ai.ErrAuth, detail)
		}
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", ai.ErrAuth, detail)
	case status == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ai.ErrInvalidModel, detail)
	case status == http.StatusTooManyRequests:
		return fmt.Errorf("%w: %s", ai.ErrRateLimited, detail)
	case status == http.StatusGatewayTimeout:
		return fmt.Errorf("%w: %s", ai.ErrTimeout, detail)
	case status >= 500:
		return fmt.Errorf("%w: %s", ai.ErrUnavailable, detail)
	}

	// 400 などは、こちらのリクエストの形式の誤り
	return fmt.Errorf("%w: %s", ai.ErrInvalidResponse, detail)
}

func buildRequest(req ai.Request) request {

	r := request{Contents: make([]content, 0, len(req.Messages))}

	if req.System != "" {
		r.SystemInstruction = &content{Parts: []part{{Text: req.System}}}
	}

	for _, m := range req.Messages {
		r.Contents = append(r.Contents, buildContent(m))
	}

	if len(req.Tools) > 0 {
		decls := make([]functionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			decls = append(decls, functionDeclaration{
				Name:                 t.Name,
				Description:          t.Description,
				ParametersJSONSchema: t.Parameters,
			})
		}
		r.Tools = []tool{{FunctionDeclarations: decls}}

		cfg := &toolConfig{}
		cfg.FunctionCallingConfig.Mode = "AUTO"
		if req.ToolChoice == ai.ToolChoiceNone {
			cfg.FunctionCallingConfig.Mode = "NONE"
		}
		r.ToolConfig = cfg
	}

	return r
}

func buildContent(m ai.Message) content {

	switch m.Role {

	case ai.RoleAssistant:
		// Gemini が返した内容があれば、署名を保つためにそのまま送り返す
		var c content
		if len(m.ProviderContent) > 0 && json.Unmarshal(m.ProviderContent, &c) == nil {
			c.Role = "model"
			return c
		}

		c = content{Role: "model"}
		if m.Text != "" {
			c.Parts = append(c.Parts, part{Text: m.Text})
		}
		for _, call := range m.ToolCalls {
			c.Parts = append(c.Parts, part{FunctionCall: &functionCall{
				ID:   call.ID,
				Name: call.Name,
				Args: call.Arguments,
			}})
		}
		return c

	case ai.RoleTool:
		c := content{Role: "user"}
		for _, r := range m.ToolResults {
			c.Parts = append(c.Parts, part{FunctionResponse: &functionResponse{
				ID:       r.CallID,
				Name:     r.Name,
				Response: r.Content,
			}})
		}
		return c
	}

	return content{Role: "user", Parts: []part{{Text: m.Text}}}
}

func parseResponse(data []byte) (ai.Response, error) {

	var r response
	if err := json.Unmarshal(data, &r); err != nil {
		return ai.Response{}, fmt.Errorf("%w: gemini: decode: %v", ai.ErrInvalidResponse, err)
	}

	usage := ai.Usage{
		InputTokens:  r.UsageMetadata.PromptTokenCount,
		OutputTokens: r.UsageMetadata.CandidatesTokenCount + r.UsageMetadata.ThoughtsTokenCount,
	}

	if len(r.Candidates) == 0 {
		reason := ""
		if r.PromptFeedback != nil {
			reason = r.PromptFeedback.BlockReason
		}
		return ai.Response{Usage: usage}, fmt.Errorf("%w: gemini: no candidates (block reason %q)", ai.ErrInvalidResponse, reason)
	}

	cand := r.Candidates[0]

	var c content
	if len(cand.Content) > 0 {
		if err := json.Unmarshal(cand.Content, &c); err != nil {
			return ai.Response{Usage: usage}, fmt.Errorf("%w: gemini: decode content: %v", ai.ErrInvalidResponse, err)
		}
	}

	resp := ai.Response{Usage: usage, ProviderContent: cand.Content}

	var text strings.Builder
	for _, pt := range c.Parts {
		switch {
		case pt.FunctionCall != nil:
			args := pt.FunctionCall.Args
			if len(args) == 0 {
				args = json.RawMessage("{}")
			}
			resp.ToolCalls = append(resp.ToolCalls, ai.ToolCall{
				ID:        pt.FunctionCall.ID,
				Name:      pt.FunctionCall.Name,
				Arguments: args,
			})
		case pt.Thought:
			// 思考の要約は回答に含めない
		default:
			text.WriteString(pt.Text)
		}
	}
	resp.Text = text.String()

	if resp.Text == "" && len(resp.ToolCalls) == 0 {
		return resp, fmt.Errorf("%w: gemini: empty content (finish reason %q)", ai.ErrInvalidResponse, cand.FinishReason)
	}

	return resp, nil
}
