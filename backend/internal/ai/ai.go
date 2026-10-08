// Package ai は AI 鉄道エージェント（docs/design/ai-api.md）。
// AI に事実を決めさせず、どの道具をどの引数で呼ぶかを決めさせ、結果を説明させる。
package ai

import (
	"context"
	"encoding/json"
	"errors"
)

// Provider は AI サービスごとの接続部品。AI サービス固有の形式は Provider の中で共通の形に変換する。
type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolChoice は、AI に道具を呼ばせるかどうか。
type ToolChoice string

const (
	// ToolChoiceAuto は、道具を呼ぶか答えるかを AI が決める
	ToolChoiceAuto ToolChoice = "auto"

	// ToolChoiceNone は道具を呼ばせない（上限に達したので、今ある情報で答えさせる）
	ToolChoiceNone ToolChoice = "none"
)

type Request struct {
	Model      string
	System     string
	Messages   []Message
	Tools      []Tool
	ToolChoice ToolChoice
}

type Message struct {
	Role Role
	Text string

	// RoleAssistant のとき、AI が呼んだ道具
	ToolCalls []ToolCall

	// RoleTool のとき、道具の結果
	ToolResults []ToolResult

	// ProviderContent は、Provider が返した応答そのもの。同じ Provider に送り返すときに使う
	// （Gemini の thoughtSignature のように、そのまま送り返す必要がある値を保つため）
	ProviderContent json.RawMessage
}

type Tool struct {
	Name        string
	Description string

	// JSON Schema
	Parameters json.RawMessage
}

type ToolCall struct {
	// AI サービスが ID を付けない場合は空
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolResult struct {
	CallID string
	Name   string

	// JSON のオブジェクト
	Content json.RawMessage
}

type Response struct {
	Text      string
	ToolCalls []ToolCall
	Usage     Usage

	// Message.ProviderContent に入れて送り返す値
	ProviderContent json.RawMessage
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}

// 共通のエラー。Provider は元の API のエラーをこれらに wrap して返す。
// API 固有のエラーメッセージはユーザーに見せない。
var (
	ErrAuth            = errors.New("ai: authentication failed")
	ErrRateLimited     = errors.New("ai: rate limited by provider")
	ErrQuotaExceeded   = errors.New("ai: app quota exceeded")
	ErrUnavailable     = errors.New("ai: provider unavailable")
	ErrTimeout         = errors.New("ai: timeout")
	ErrInvalidModel    = errors.New("ai: invalid model")
	ErrInvalidResponse = errors.New("ai: invalid response")

	// ErrInvalidRequest は、ユーザーのリクエストが不正なこと（入力が長すぎるなど）
	ErrInvalidRequest = errors.New("ai: invalid request")
)
