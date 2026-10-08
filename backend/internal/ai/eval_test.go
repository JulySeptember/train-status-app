package ai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/gemini"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/service"
)

type evalCase struct {
	Name           string                    `yaml:"name"`
	Input          string                    `yaml:"input"`
	Status         map[string]string         `yaml:"status"`
	ExpectTools    []string                  `yaml:"expect_tools"`
	ExpectArgs     map[string]map[string]any `yaml:"expect_args"`
	ExpectNoTools  bool                      `yaml:"expect_no_tools"`
	ExpectReplyAny []string                  `yaml:"expect_reply_any"`
	ExpectReplyNot []string                  `yaml:"expect_reply_not"`
}

// recordingTools は、呼ばれた道具を記録する ToolSet
type recordingTools struct {
	*ai.Tools

	mu    sync.Mutex
	calls []ai.ToolCall
}

func (r *recordingTools) Call(ctx context.Context, call ai.ToolCall) ai.ToolOutput {
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
	return r.Tools.Call(ctx, call)
}

// TestEval は評価セット（testdata/eval.yaml）を実際の Gemini で実行する。AI_EVAL=1 のときだけ動く
func TestEval(t *testing.T) {

	key := os.Getenv("GEMINI_API_KEY")
	if os.Getenv("AI_EVAL") != "1" || key == "" {
		t.Skip("set AI_EVAL=1 and GEMINI_API_KEY to run the evaluation")
	}

	cases := loadEvalCases(t)

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	modelID := ai.ResolveModel(os.Getenv("AI_MODEL"))
	provider := gemini.New(key)

	// 無料枠の1分あたりの上限に当たらないように、質問の間を空ける
	interval := 10 * time.Second
	if v, err := strconv.Atoi(os.Getenv("AI_EVAL_INTERVAL_SECONDS")); err == nil {
		interval = time.Duration(v) * time.Second
	}

	passed := 0
	for i, c := range cases {
		if i > 0 {
			time.Sleep(interval)
		}

		ok := t.Run(c.Name, func(t *testing.T) {
			tools := &recordingTools{Tools: ai.NewTools(service.New(fixedClient(loader, c.Status), loader))}
			limiter := ai.NewMemoryLimiter(ai.Limits{PerIPPerMinute: 100, PerIPPerDay: 1000, CallsPerDay: 1000})
			svc := ai.NewService(provider, modelID, tools, limiter, ai.DefaultConfig())

			res, err := svc.Chat(t.Context(), "eval", ai.ChatRequest{Messages: []ai.ChatMessage{{Role: "user", Text: c.Input}}})
			if err != nil {
				t.Fatalf("chat failed: %v", err)
			}

			var called []string
			for _, call := range tools.calls {
				called = append(called, fmt.Sprintf("%s%s", call.Name, call.Arguments))
			}
			t.Logf("input: %s\ntools: %s\nreply: %s", c.Input, strings.Join(called, " "), res.Reply)

			checkEval(t, c, tools.calls, res.Reply)
		})
		if ok {
			passed++
		}
	}

	t.Logf("model %s: %d / %d passed", modelID, passed, len(cases))
}

func loadEvalCases(t *testing.T) []evalCase {
	t.Helper()

	data, err := os.ReadFile("testdata/eval.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var cases []evalCase
	if err := yaml.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// TestEvalCases は、評価セットの書き方が正しいかを確かめる（AI は呼ばない。CI でも実行する）
func TestEvalCases(t *testing.T) {

	cases := loadEvalCases(t)
	if len(cases) < 20 {
		t.Fatalf("expected at least 20 cases, got %d", len(cases))
	}

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	var tools []string
	for _, d := range ai.NewTools(service.New(&trainClient{}, loader)).Definitions() {
		tools = append(tools, d.Name)
	}

	railways := make(map[string]bool)
	for _, r := range loader.Railways() {
		railways[r.SameAs] = true
	}

	for _, c := range cases {
		if c.Name == "" || c.Input == "" {
			t.Errorf("name and input are required: %+v", c)
		}
		for _, name := range c.ExpectTools {
			if !slices.Contains(tools, name) {
				t.Errorf("%s: unknown tool %s", c.Name, name)
			}
		}
		for name := range c.ExpectArgs {
			if !slices.Contains(tools, name) {
				t.Errorf("%s: unknown tool %s", c.Name, name)
			}
		}
		for id, state := range c.Status {
			if !railways[id] || (state != "suspended" && !strings.HasPrefix(state, "delayed:")) {
				t.Errorf("%s: invalid status %s: %s", c.Name, id, state)
			}
		}
	}
}

func checkEval(t *testing.T, c evalCase, calls []ai.ToolCall, reply string) {
	t.Helper()

	names := make([]string, 0, len(calls))
	for _, call := range calls {
		names = append(names, call.Name)
	}

	if c.ExpectNoTools && len(calls) > 0 {
		t.Errorf("expected no tools, got %v", names)
	}

	for _, want := range c.ExpectTools {
		if !slices.Contains(names, want) {
			t.Errorf("expected tool %s, got %v", want, names)
		}
	}

	for tool, want := range c.ExpectArgs {
		matched := slices.ContainsFunc(calls, func(call ai.ToolCall) bool {
			return call.Name == tool && argsMatch(call.Arguments, want)
		})
		if !matched {
			t.Errorf("expected %s with %v", tool, want)
		}
	}

	if len(c.ExpectReplyAny) > 0 && !slices.ContainsFunc(c.ExpectReplyAny, func(w string) bool { return strings.Contains(reply, w) }) {
		t.Errorf("expected the reply to contain one of %v", c.ExpectReplyAny)
	}

	for _, w := range c.ExpectReplyNot {
		if strings.Contains(reply, w) {
			t.Errorf("the reply must not contain %q", w)
		}
	}
}

// argsMatch は、引数が want を満たすか。文字列は一致、配列は want の要素をすべて含む
func argsMatch(raw json.RawMessage, want map[string]any) bool {

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		return false
	}

	for k, w := range want {
		switch w := w.(type) {
		case []any:
			list, _ := got[k].([]any)
			for _, item := range w {
				if !slices.Contains(list, item) {
					return false
				}
			}
		default:
			if fmt.Sprint(got[k]) != fmt.Sprint(w) {
				return false
			}
		}
	}
	return true
}

// fixedClient は、評価の項目で決めた運行状況を返す
func fixedClient(loader *assets.Loader, status map[string]string) *trainClient {

	c := &trainClient{}

	for _, r := range loader.Railways() {

		text := "現在、１５分以上の遅延はありません。"
		state := status[r.SameAs]

		switch {
		case state == "suspended":
			text = r.RailwayTitle.Ja + "は、人身事故の影響で、運転を見合わせています。"

		case strings.HasPrefix(state, "delayed:"):
			minutes, _ := strconv.Atoi(strings.TrimPrefix(state, "delayed:"))
			text = r.RailwayTitle.Ja + "は、車両故障の影響で、遅れが出ています。"
			for _, dir := range []string{r.AscendingRailDirection, r.DescendingRailDirection} {
				c.locations = append(c.locations, model.TrainLocation{Railway: r.SameAs, RailDirection: dir, Delay: minutes * 60})
			}
		}

		c.statuses = append(c.statuses, model.TrainStatus{
			Railway:              r.SameAs,
			TrainInformationText: model.LocalizedString{Ja: text},
		})
	}

	return c
}
