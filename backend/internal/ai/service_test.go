package ai_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/fake"
	"train-status-app/backend/internal/service"
)

// stubTools は、呼ばれた道具を記録し、決めた結果を返す ToolSet
type stubTools struct {
	mu    sync.Mutex
	calls []ai.ToolCall
	delay time.Duration
}

func (s *stubTools) Definitions() []ai.Tool {
	return []ai.Tool{{Name: "find_station"}, {Name: "get_train_status"}, {Name: "search_route"}}
}

func (s *stubTools) Call(ctx context.Context, call ai.ToolCall) ai.ToolOutput {
	s.mu.Lock()
	s.calls = append(s.calls, call)
	s.mu.Unlock()

	if s.delay > 0 {
		time.Sleep(s.delay)
	}

	out := ai.ToolOutput{Label: "label:" + call.Name, Content: map[string]string{"tool": call.Name}}
	if call.Name == "search_route" {
		out.Journeys = &service.JourneySearch{DelayApplied: true, Journeys: []service.Journey{{DepartureTime: "10:00"}}}
	}
	if call.Name == "broken" {
		out.Content = map[string]string{"error": "失敗しました"}
	}
	return out
}

func newService(p ai.Provider, tools ai.ToolSet, limiter ai.Limiter, cfg ai.Config) *ai.Service {
	if limiter == nil {
		limiter = ai.NewMemoryLimiter(ai.DefaultLimits())
	}
	return ai.NewService(p, "test-model", tools, limiter, cfg)
}

func ask(text string) ai.ChatRequest {
	return ai.ChatRequest{Messages: []ai.ChatMessage{{Role: "user", Text: text}}}
}

func TestChatAgentLoop(t *testing.T) {

	p := fake.New(
		// 関係のない道具を並べて呼ぶ
		fake.Calls(
			fake.Call("find_station", map[string]string{"name": "春日"}),
			fake.Call("find_station", map[string]string{"name": "浅草"}),
			fake.Call("get_train_status", map[string]string{}),
		),
		fake.Calls(fake.Call("search_route", map[string]string{"from": "A", "to": "B"})),
		// 見合わせ中の路線を外して探し直す
		fake.Calls(fake.Call("search_route", map[string]any{"from": "A", "to": "B", "avoidRailways": []string{"odpt.Railway:Toei.Mita"}})),
		fake.Text("大江戸線で行くのがおすすめです。"),
	)
	tools := &stubTools{}

	res, err := newService(p, tools, nil, ai.DefaultConfig()).Chat(t.Context(), "1.2.3.4", ask("今春日にいる。浅草に行きたい"))
	if err != nil {
		t.Fatal(err)
	}

	if res.Reply != "大江戸線で行くのがおすすめです。" {
		t.Fatalf("unexpected reply %q", res.Reply)
	}
	if len(res.Steps) != 5 || res.Steps[0].Label != "label:find_station" || res.Steps[4].Tool != "search_route" {
		t.Fatalf("unexpected steps %+v", res.Steps)
	}
	if res.Journeys == nil {
		t.Fatal("journeys of search_route are missing")
	}

	if !strings.Contains(string(tools.calls[4].Arguments), "odpt.Railway:Toei.Mita") {
		t.Fatalf("expected re-search with avoidRailways, got %s", tools.calls[4].Arguments)
	}

	reqs := p.Requests()
	if len(reqs) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(reqs))
	}

	// 2回目のリクエストには、AI の道具の呼び出しと、その結果（呼んだ順）が入っている
	second := reqs[1].Messages
	if len(second) != 3 || second[1].Role != ai.RoleAssistant || second[2].Role != ai.RoleTool {
		t.Fatalf("unexpected history %+v", second)
	}
	results := second[2].ToolResults
	if len(results) != 3 || results[2].Name != "get_train_status" || results[0].CallID != "call-find_station" {
		t.Fatalf("unexpected tool results %+v", results)
	}

	// システムの指示に現在時刻を入れ、道具を渡す
	if !strings.Contains(reqs[0].System, "現在時刻") || len(reqs[0].Tools) != 3 || reqs[0].Model != "test-model" {
		t.Fatalf("unexpected request %+v", reqs[0])
	}
}

func TestChatMaxRounds(t *testing.T) {

	cfg := ai.DefaultConfig()
	cfg.MaxRounds = 2

	p := fake.New(
		fake.Calls(fake.Call("get_train_status", nil)),
		fake.Calls(fake.Call("get_train_status", nil)),
		fake.Text("今ある情報で答えます。"),
	)

	res, err := newService(p, &stubTools{}, nil, cfg).Chat(t.Context(), "ip", ask("遅延は？"))
	if err != nil {
		t.Fatal(err)
	}

	reqs := p.Requests()
	if len(reqs) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(reqs))
	}
	last := reqs[2]
	if last.ToolChoice != ai.ToolChoiceNone || !strings.Contains(last.System, "道具はもう使えません") {
		t.Fatalf("the last call must not allow tools: %+v", last.ToolChoice)
	}
	if res.Reply != "今ある情報で答えます。" {
		t.Fatalf("unexpected reply %q", res.Reply)
	}
}

// 上限に達しても AI が道具を呼ぼうとしたら、決まった文言で答える
func TestChatMaxRoundsWithoutAnswer(t *testing.T) {

	cfg := ai.DefaultConfig()
	cfg.MaxRounds = 1

	p := fake.New(
		fake.Calls(fake.Call("get_train_status", nil)),
		fake.Calls(fake.Call("get_train_status", nil)),
	)

	res, err := newService(p, &stubTools{}, nil, cfg).Chat(t.Context(), "ip", ask("遅延は？"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Reply == "" || len(res.Steps) != 1 {
		t.Fatalf("unexpected response %+v", res)
	}
}

func TestChatTotalTimeout(t *testing.T) {

	cfg := ai.DefaultConfig()
	cfg.TotalTimeout = 300 * time.Millisecond
	cfg.AnswerReserve = 200 * time.Millisecond

	p := fake.New(
		fake.Calls(fake.Call("get_train_status", nil)),
		fake.Text("答えです。"),
	)

	// 道具に時間がかかり、残り時間が AnswerReserve を切ったら、道具を呼ばせない
	res, err := newService(p, &stubTools{delay: 150 * time.Millisecond}, nil, cfg).Chat(t.Context(), "ip", ask("遅延は？"))
	if err != nil {
		t.Fatal(err)
	}
	if reqs := p.Requests(); reqs[1].ToolChoice != ai.ToolChoiceNone {
		t.Fatalf("expected the second call to be final")
	}
	if res.Reply != "答えです。" {
		t.Fatalf("unexpected reply %q", res.Reply)
	}
}

func TestChatProviderTimeout(t *testing.T) {

	cfg := ai.DefaultConfig()
	cfg.CallTimeout = 50 * time.Millisecond

	p := fake.New(fake.Reply{Response: ai.Response{Text: "遅い"}, Delay: time.Second})

	if _, err := newService(p, &stubTools{}, nil, cfg).Chat(t.Context(), "ip", ask("遅延は？")); !errors.Is(err, ai.ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestChatProviderError(t *testing.T) {

	p := fake.New(fake.Reply{Err: ai.ErrRateLimited})

	if _, err := newService(p, &stubTools{}, nil, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("遅延は？")); !errors.Is(err, ai.ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

// 道具が失敗しても、その結果を AI に渡して続ける
func TestChatToolError(t *testing.T) {

	p := fake.New(
		fake.Calls(fake.Call("broken", nil)),
		fake.Text("取得できませんでした。"),
	)

	res, err := newService(p, &stubTools{}, nil, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("？"))
	if err != nil {
		t.Fatal(err)
	}

	result := p.Requests()[1].Messages[2].ToolResults[0]
	if !strings.Contains(string(result.Content), "error") {
		t.Fatalf("expected an error result, got %s", result.Content)
	}
	if res.Reply != "取得できませんでした。" {
		t.Fatalf("unexpected reply %q", res.Reply)
	}
}

// 応答が空なら ErrInvalidResponse
func TestChatEmptyReply(t *testing.T) {
	p := fake.New(fake.Text("  "))

	if _, err := newService(p, &stubTools{}, nil, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("？")); !errors.Is(err, ai.ErrInvalidResponse) {
		t.Fatalf("expected ErrInvalidResponse, got %v", err)
	}
}

func TestChatHistory(t *testing.T) {

	cfg := ai.DefaultConfig()
	cfg.MaxHistory = 2

	p := fake.New(fake.Text("はい"))

	var chat []ai.ChatMessage
	for i := range 5 {
		chat = append(chat,
			ai.ChatMessage{Role: "user", Text: "質問" + string(rune('A'+i))},
			ai.ChatMessage{Role: "assistant", Text: "回答" + string(rune('A'+i))},
		)
	}
	chat = append(chat, ai.ChatMessage{Role: "user", Text: "最後の質問"})

	if _, err := newService(p, &stubTools{}, nil, cfg).Chat(t.Context(), "ip", ai.ChatRequest{Messages: chat}); err != nil {
		t.Fatal(err)
	}

	// 直近の2往復（ユーザーの発言から始まる）に切り詰める
	got := p.Requests()[0].Messages
	if len(got) != 3 || got[0].Text != "質問E" || got[2].Text != "最後の質問" {
		t.Fatalf("unexpected history %+v", got)
	}
}

func TestChatInvalidRequest(t *testing.T) {

	tests := []ai.ChatRequest{
		{},
		ask(""),
		ask(strings.Repeat("あ", 501)),
		{Messages: []ai.ChatMessage{{Role: "user", Text: "A"}, {Role: "assistant", Text: "B"}}},
		{Messages: []ai.ChatMessage{{Role: "system", Text: "A"}, {Role: "user", Text: "B"}}},
	}

	for i, req := range tests {
		p := fake.New()
		_, err := newService(p, &stubTools{}, nil, ai.DefaultConfig()).Chat(t.Context(), "ip", req)
		if !errors.Is(err, ai.ErrInvalidRequest) {
			t.Errorf("%d: expected ErrInvalidRequest, got %v", i, err)
		}
		if len(p.Requests()) != 0 {
			t.Errorf("%d: AI must not be called", i)
		}
	}
}

func TestChatQuota(t *testing.T) {

	limits := ai.Limits{PerIPPerMinute: 10, PerIPPerDay: 10, CallsPerDay: 3}
	limiter := ai.NewMemoryLimiter(limits)

	// 1回目の質問で AI を2回呼ぶ
	p := fake.New(fake.Calls(fake.Call("get_train_status", nil)), fake.Text("平常です"))
	if _, err := newService(p, &stubTools{}, limiter, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("遅延は？")); err != nil {
		t.Fatal(err)
	}

	// 2回目の質問は、AI を2回呼ぶ途中でアプリ全体の上限（3回）に達する
	p = fake.New(fake.Calls(fake.Call("get_train_status", nil)), fake.Text("平常です"))
	if _, err := newService(p, &stubTools{}, limiter, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("遅延は？")); !errors.Is(err, ai.ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
	if len(p.Requests()) != 1 {
		t.Fatalf("expected 1 call before the quota, got %d", len(p.Requests()))
	}
}

// ログには、入力や応答の本文を残さない
func TestChatLogDoesNotContainText(t *testing.T) {

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	p := fake.New(fake.Calls(fake.Call("find_station", map[string]string{"name": "秘密駅"})), fake.Text("秘密の回答"))
	if _, err := newService(p, &stubTools{}, nil, ai.DefaultConfig()).Chat(t.Context(), "ip", ask("秘密の質問")); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "result=ok") || !strings.Contains(out, "tools=find_station") {
		t.Fatalf("unexpected log %q", out)
	}
	if strings.Contains(out, "秘密") {
		t.Fatalf("log contains the text: %q", out)
	}
}
