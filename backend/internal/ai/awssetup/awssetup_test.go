package awssetup

import (
	"context"
	"errors"
	"testing"
	"time"

	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/fake"
)

type noTools struct{}

func (noTools) Definitions() []ai.Tool                          { return nil }
func (noTools) Call(context.Context, ai.ToolCall) ai.ToolOutput { return ai.ToolOutput{} }

type setup struct {
	chat      *Chat
	now       time.Time
	key       string
	keyErr    error
	keyLoads  int
	providers []*fake.Provider
}

func newSetup() *setup {
	s := &setup{now: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}

	s.chat = &Chat{
		now: func() time.Time { return s.now },
		deps: deps{
			loadKey: func(context.Context) (string, error) {
				s.keyLoads++
				return s.key, s.keyErr
			},
			newLimiter: func(context.Context) (ai.Limiter, error) {
				return ai.NewMemoryLimiter(ai.DefaultLimits()), nil
			},
			newChat: func(key string, limiter ai.Limiter) *ai.Service {
				p := s.providers[0]
				s.providers = s.providers[1:]
				return ai.NewService(p, "model", noTools{}, limiter, ai.DefaultConfig())
			},
		},
	}
	return s
}

func ask(c *Chat) (*ai.ChatResponse, error) {
	return c.Chat(context.Background(), "ip", ai.ChatRequest{Messages: []ai.ChatMessage{{Role: "user", Text: "こんにちは"}}})
}

func TestNotRegisteredThenRegistered(t *testing.T) {

	s := newSetup()
	s.keyErr = errNotRegistered
	s.providers = []*fake.Provider{fake.New(fake.Text("こんにちは"))}

	if _, err := ask(s.chat); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}

	// しばらくは SSM を読み直さない
	s.keyErr = nil
	s.key = "key"
	if _, err := ask(s.chat); !errors.Is(err, ai.ErrUnavailable) || s.keyLoads != 1 {
		t.Fatalf("expected cached ErrUnavailable without reload, got %v (loads %d)", err, s.keyLoads)
	}

	// 間隔を空けると読み直し、登録したキーで使える
	s.now = s.now.Add(notRegisteredRetry)
	res, err := ask(s.chat)
	if err != nil || res.Reply != "こんにちは" {
		t.Fatalf("expected a reply, got %v %v", res, err)
	}

	// 組み立てたあとは読み直さない（fake の応答が尽きているので回答はエラーになる）
	_, _ = ask(s.chat)
	if s.keyLoads != 2 {
		t.Fatalf("key must not be reloaded, loads %d", s.keyLoads)
	}
}

func TestEmptyKey(t *testing.T) {
	s := newSetup()
	s.key = ""

	if _, err := ask(s.chat); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestFailedRetry(t *testing.T) {
	s := newSetup()
	s.keyErr = errors.New("access denied")

	if _, err := ask(s.chat); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}

	s.now = s.now.Add(failedRetry)
	if _, err := ask(s.chat); !errors.Is(err, ai.ErrUnavailable) || s.keyLoads != 2 {
		t.Fatalf("expected retry after %v, loads %d", failedRetry, s.keyLoads)
	}
}

// キーが無効になったら、次の質問でキーを読み直す
func TestReloadAfterAuthError(t *testing.T) {

	s := newSetup()
	s.key = "old"
	s.providers = []*fake.Provider{
		fake.New(fake.Reply{Err: ai.ErrAuth}),
		fake.New(fake.Text("新しいキーで答えます")),
	}

	if _, err := ask(s.chat); !errors.Is(err, ai.ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}

	s.key = "new"
	res, err := ask(s.chat)
	if err != nil || res.Reply != "新しいキーで答えます" || s.keyLoads != 2 {
		t.Fatalf("expected reload, got %v %v (loads %d)", res, err, s.keyLoads)
	}
}
