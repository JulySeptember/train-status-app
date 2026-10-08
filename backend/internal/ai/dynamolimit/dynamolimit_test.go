package dynamolimit

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"train-status-app/backend/internal/ai"
)

// fakeTable は、UpdateItem の条件付きの加算を真似る
type fakeTable struct {
	mu     sync.Mutex
	counts map[string]int
	inputs []*dynamodb.UpdateItemInput
	err    error
}

func (f *fakeTable) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}

	key := in.Key["pk"].(*types.AttributeValueMemberS).Value
	limit, _ := strconv.Atoi(in.ExpressionAttributeValues[":limit"].(*types.AttributeValueMemberN).Value)

	if f.counts[key] >= limit {
		return nil, &types.ConditionalCheckFailedException{}
	}
	f.counts[key]++
	return &dynamodb.UpdateItemOutput{}, nil
}

func newLimiter(table *fakeTable, limits ai.Limits, now *time.Time) *Limiter {
	table.counts = make(map[string]int)
	l := New(table, "usage", limits)
	l.now = func() time.Time { return *now }
	return l
}

func TestAllowQuestion(t *testing.T) {

	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	table := &fakeTable{}
	l := newLimiter(table, ai.Limits{PerIPPerMinute: 2, PerIPPerDay: 3, CallsPerDay: 10}, &now)

	ctx := t.Context()

	for range 2 {
		if err := l.AllowQuestion(ctx, "203.0.113.1"); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.AllowQuestion(ctx, "203.0.113.1"); !errors.Is(err, ai.ErrRateLimited) || !strings.Contains(err.Error(), "per-minute") {
		t.Fatalf("expected per-minute rate limit, got %v", err)
	}
	if err := l.AllowQuestion(ctx, "203.0.113.2"); err != nil {
		t.Fatalf("another IP must be allowed: %v", err)
	}

	now = now.Add(time.Minute)
	if err := l.AllowQuestion(ctx, "203.0.113.1"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := l.AllowQuestion(ctx, "203.0.113.1"); !errors.Is(err, ai.ErrQuotaExceeded) || !strings.Contains(err.Error(), "per-day") {
		t.Fatalf("expected per-day quota, got %v", err)
	}

	// IP はそのまま保存しない。項目には期限（TTL）を付ける
	for _, in := range table.inputs {
		key := in.Key["pk"].(*types.AttributeValueMemberS).Value
		if strings.Contains(key, "203.0.113") {
			t.Fatalf("the key contains the raw IP: %s", key)
		}
		if *in.TableName != "usage" || in.ExpressionAttributeValues[":expires"] == nil {
			t.Fatalf("unexpected input %+v", in)
		}
	}
}

func TestAllowCall(t *testing.T) {

	now := time.Date(2026, 10, 9, 6, 59, 0, 0, time.UTC) // 太平洋時間 10/8 23:59
	table := &fakeTable{}
	l := newLimiter(table, ai.Limits{PerIPPerMinute: 10, PerIPPerDay: 10, CallsPerDay: 2}, &now)

	for range 2 {
		if err := l.AllowCall(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.AllowCall(t.Context()); !errors.Is(err, ai.ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}

	// 太平洋時間の0時で数え直す
	now = now.Add(time.Minute)
	if err := l.AllowCall(t.Context()); err != nil {
		t.Fatalf("expected reset, got %v", err)
	}
}

// テーブルに書けないときは、使えないものとして扱う
func TestUnavailable(t *testing.T) {

	now := time.Now()
	table := &fakeTable{}
	l := newLimiter(table, ai.DefaultLimits(), &now)
	table.err = errors.New("throttled")

	if err := l.AllowCall(t.Context()); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
	if err := l.AllowQuestion(t.Context(), "ip"); !errors.Is(err, ai.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}
