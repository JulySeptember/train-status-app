package odptkey

import (
	"context"
	"errors"
	"testing"
	"time"

	"train-status-app/backend/internal/client"
)

func TestStatic(t *testing.T) {

	key := Static("basic-key", "")

	if v, err := key(context.Background(), client.HostBasic); err != nil || v != "basic-key" {
		t.Fatalf("expected basic-key, got %q %v", v, err)
	}

	if _, err := key(context.Background(), client.HostChallenge); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestCached(t *testing.T) {

	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

	calls := map[string]int{}
	fail := true

	load := func(_ context.Context, name string) (string, error) {
		calls[name]++
		if fail {
			return "", errors.New("parameter is not registered")
		}
		return "value-of-" + name, nil
	}

	c := newCached(load, "/basic", "", func() time.Time { return now })

	// パラメータ名の無いホストは読まない
	if _, err := c.key(context.Background(), client.HostChallenge); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}

	// 失敗したら、retryInterval のあいだは読み直さない
	if _, err := c.key(context.Background(), client.HostBasic); err == nil {
		t.Fatal("expected an error")
	}
	fail = false
	if _, err := c.key(context.Background(), client.HostBasic); !errors.Is(err, errUnavailable) {
		t.Fatalf("expected errUnavailable, got %v", err)
	}
	if calls["/basic"] != 1 {
		t.Fatalf("expected 1 load, got %d", calls["/basic"])
	}

	// 間隔が過ぎたら読み直し、読めた値は持ち続ける
	now = now.Add(retryInterval)
	for range 2 {
		v, err := c.key(context.Background(), client.HostBasic)
		if err != nil || v != "value-of-/basic" {
			t.Fatalf("expected the value, got %q %v", v, err)
		}
	}
	if calls["/basic"] != 2 {
		t.Fatalf("expected 2 loads, got %d", calls["/basic"])
	}
}

func TestCachedEmptyValue(t *testing.T) {

	c := newCached(func(context.Context, string) (string, error) { return "", nil }, "/basic", "", time.Now)

	if _, err := c.key(context.Background(), client.HostBasic); err == nil {
		t.Fatal("expected an error for an empty value")
	}
}
