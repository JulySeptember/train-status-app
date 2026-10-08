package handler

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"

	"train-status-app/backend/internal/ai"
)

// maxChatBodyBytes は /api/chat のリクエストの大きさの上限（500 文字 × 履歴 6 往復に余裕を持たせた値）
const maxChatBodyBytes = 64 << 10

type ChatService interface {
	Chat(ctx context.Context, clientIP string, req ai.ChatRequest) (*ai.ChatResponse, error)
}

// ChatError は /api/chat のエラー。code で画面の表示（上限に達したときの導線など）を分ける
type ChatError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// chatErrors は、共通のエラーの種類ごとの HTTP ステータスとユーザー向けの文言（設計書 6.2）。
// AI サービス固有のエラーメッセージは表示しない
var chatErrors = map[string]struct {
	status  int
	message string
}{
	"invalid_request":  {http.StatusBadRequest, "質問の内容を確認してください（1回に500文字まで）。"},
	"auth":             {http.StatusBadGateway, "AI サービスに接続できませんでした。設定を確認してください。"},
	"rate_limited":     {http.StatusTooManyRequests, "現在 AI が混み合っています。しばらくしてからお試しください。"},
	"quota_exceeded":   {http.StatusTooManyRequests, "本日の AI 利用上限に達しました。通常の検索をご利用ください。"},
	"unavailable":      {http.StatusServiceUnavailable, "AI サービスが一時的に利用できません。"},
	"timeout":          {http.StatusGatewayTimeout, "応答に時間がかかっています。もう一度お試しください。"},
	"invalid_model":    {http.StatusInternalServerError, "AI サービスに接続できませんでした。"},
	"invalid_response": {http.StatusBadGateway, "AI の応答を処理できませんでした。"},
	"internal":         {http.StatusInternalServerError, "エラーが発生しました。"},
}

func writeChatError(w http.ResponseWriter, kind string) {
	e, ok := chatErrors[kind]
	if !ok {
		kind = "internal"
		e = chatErrors[kind]
	}
	writeJSON(w, e.status, ChatError{Error: e.message, Code: kind})
}

// Chat godoc
//
//	@Summary		Ask the AI agent
//	@Description	AI エージェントに質問する。会話の履歴はブラウザが持ち、毎回送る（最後はユーザーの発言）。
//	@Description	入力は AI の提供元（Google）に送信される。
//	@Tags			Chat
//	@Accept			json
//	@Produce		json
//	@Param			request	body		ai.ChatRequest	true	"Conversation"
//	@Success		200		{object}	ai.ChatResponse
//	@Failure		400		{object}	ChatError
//	@Failure		429		{object}	ChatError
//	@Failure		502		{object}	ChatError
//	@Failure		503		{object}	ChatError
//	@Failure		504		{object}	ChatError
//	@Router			/api/chat [post]
func (h *Handler) Chat(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.chat == nil {
		writeChatError(w, "unavailable")
		return
	}

	var req ai.ChatRequest

	body := http.MaxBytesReader(w, r.Body, maxChatBodyBytes)
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		writeChatError(w, "invalid_request")
		return
	}

	res, err := h.chat.Chat(r.Context(), clientIP(r), req)
	if err != nil {
		kind := ai.ErrorKind(err)
		if kind == "internal" || kind == "invalid_model" || kind == "auth" {
			// 設定の誤りなど、調べる必要があるもの。エラーには入力の本文やキーを含めない
			log.Printf("chat error: %v", err)
		}
		writeChatError(w, kind)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// clientIP は利用上限に使う利用者の IP を返す。
// CloudFront は X-Forwarded-For の末尾に利用者の IP を足し、API Gateway はその後ろに CloudFront の IP を足すことがある。
// 利用者が送った X-Forwarded-For は先頭側に残るので、末尾から、接続元（RemoteAddr）と同じ値を除いた最後の値を使う。
func clientIP(r *http.Request) string {

	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}

	var forwarded []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		for ip := range strings.SplitSeq(v, ",") {
			if ip = strings.TrimSpace(ip); ip != "" {
				forwarded = append(forwarded, ip)
			}
		}
	}

	for i := len(forwarded) - 1; i >= 0; i-- {
		if forwarded[i] != remote {
			return forwarded[i]
		}
	}

	return remote
}
