package ai

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryLimiterPerIP(t *testing.T) {

	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

	l := NewMemoryLimiter(Limits{PerIPPerMinute: 2, PerIPPerDay: 3, CallsPerDay: 100})
	l.now = func() time.Time { return now }

	ctx := t.Context()

	for range 2 {
		if err := l.AllowQuestion(ctx, "a"); err != nil {
			t.Fatal(err)
		}
	}
	// 1分あたりの上限は、少し待てば使える
	if err := l.AllowQuestion(ctx, "a"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	// 別の IP は数えない
	if err := l.AllowQuestion(ctx, "b"); err != nil {
		t.Fatal(err)
	}

	// 次の分は使えるが、1日の上限（3回）に達する
	now = now.Add(time.Minute)
	if err := l.AllowQuestion(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := l.AllowQuestion(ctx, "a"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestMemoryLimiterDailyCallsResetAtPacificMidnight(t *testing.T) {

	// 2026-10-08 23:59 米国太平洋時間（夏時間、UTC-7）= 2026-10-09 06:59 UTC
	now := time.Date(2026, 10, 9, 6, 59, 0, 0, time.UTC)

	l := NewMemoryLimiter(Limits{PerIPPerMinute: 10, PerIPPerDay: 10, CallsPerDay: 1})
	l.now = func() time.Time { return now }

	if err := l.AllowCall(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := l.AllowCall(t.Context()); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}

	// 太平洋時間の午前0時（日本時間の16時）で数え直す
	now = now.Add(time.Minute)
	if err := l.AllowCall(t.Context()); err != nil {
		t.Fatalf("expected reset at pacific midnight, got %v", err)
	}
}

func TestQuotaDay(t *testing.T) {
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)

	// 冬（太平洋標準時、UTC-8）は日本時間の17時に切り替わる
	if got := QuotaDay(time.Date(2026, 12, 1, 16, 59, 0, 0, jst)); got != "2026-11-30" {
		t.Fatalf("unexpected day %s", got)
	}
	if got := QuotaDay(time.Date(2026, 12, 1, 17, 0, 0, 0, jst)); got != "2026-12-01" {
		t.Fatalf("unexpected day %s", got)
	}
}
