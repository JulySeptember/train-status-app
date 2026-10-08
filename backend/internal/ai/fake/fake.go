// Package fake は、テスト用の Provider。決めておいた応答を順に返し、受け取ったリクエストを記録する。
package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"train-status-app/backend/internal/ai"
)

// Reply は1回の呼び出しへの応答。
type Reply struct {
	Response ai.Response
	Err      error

	// Delay だけ待ってから返す。待っている間に ctx が終われば ctx.Err() を返す
	Delay time.Duration
}

type Provider struct {
	mu       sync.Mutex
	replies  []Reply
	requests []ai.Request
}

func New(replies ...Reply) *Provider {
	return &Provider{replies: replies}
}

func (p *Provider) Name() string { return "fake" }

func (p *Provider) Generate(ctx context.Context, req ai.Request) (ai.Response, error) {

	p.mu.Lock()
	req.Messages = slices.Clone(req.Messages)
	p.requests = append(p.requests, req)

	if len(p.replies) == 0 {
		p.mu.Unlock()
		return ai.Response{}, fmt.Errorf("%w: fake has no more replies", ai.ErrInvalidResponse)
	}
	reply := p.replies[0]
	p.replies = p.replies[1:]
	p.mu.Unlock()

	if reply.Delay > 0 {
		select {
		case <-time.After(reply.Delay):
		case <-ctx.Done():
			return ai.Response{}, ctx.Err()
		}
	}

	return reply.Response, reply.Err
}

// Requests は、これまでに受け取ったリクエスト
func (p *Provider) Requests() []ai.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.requests)
}

// Text は、文章で答える応答
func Text(text string) Reply {
	return Reply{Response: ai.Response{Text: text, Usage: ai.Usage{InputTokens: 10, OutputTokens: 5}}}
}

// Calls は、道具を呼ぶ応答
func Calls(calls ...ai.ToolCall) Reply {
	return Reply{Response: ai.Response{ToolCalls: calls, Usage: ai.Usage{InputTokens: 10, OutputTokens: 5}}}
}

// Call は道具の呼び出し。args は JSON にできる値
func Call(name string, args any) ai.ToolCall {
	raw, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return ai.ToolCall{ID: "call-" + name, Name: name, Arguments: raw}
}
