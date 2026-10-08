package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	_ "time/tzdata" // Lambda（provided.al2023）にタイムゾーンのデータが無くても動くように
)

// Limiter は利用上限（設計書 9章）。共有の無料枠を1人で使い切られないようにする。
type Limiter interface {
	// AllowQuestion は、IP ごとの質問の回数（1分・1日）を数える。
	// 1分の上限を超えたら ErrRateLimited（少し待てば使える）、1日の上限を超えたら ErrQuotaExceeded を返す
	AllowQuestion(ctx context.Context, clientIP string) error

	// AllowCall は、アプリ全体の1日の AI 呼び出しの回数を数え、上限を超えたら ErrQuotaExceeded を返す。
	// 1回の質問で AI を複数回呼ぶので、質問ではなく呼び出しで数える
	AllowCall(ctx context.Context) error
}

type Limits struct {
	PerIPPerMinute int
	PerIPPerDay    int

	// アプリ全体の1日の AI 呼び出しの回数。Gemini の無料枠（AI Studio で確認する）の 80% にする
	CallsPerDay int
}

func DefaultLimits() Limits {
	return Limits{
		PerIPPerMinute: 5,
		PerIPPerDay:    30,
		CallsPerDay:    200,
	}
}

// pacific は Gemini の1日の上限がリセットされるタイムゾーン（米国太平洋時間の午前0時）
var pacific = mustLoadLocation("America/Los_Angeles")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// QuotaDay は、利用上限の1日の区切り（米国太平洋時間の日付）
func QuotaDay(t time.Time) string {
	return t.In(pacific).Format("2006-01-02")
}

// MemoryLimiter はメモリ上で数える Limiter。手元での実行とテストに使う。
// Lambda ではインスタンスごとに数えることになるので、本番では DynamoDB の実装を使う。
type MemoryLimiter struct {
	limits Limits
	now    func() time.Time

	mu     sync.Mutex
	day    string
	daily  map[string]int // IP → その日の質問の回数
	minute map[string]int // IP + 分 → 質問の回数
	calls  int
}

func NewMemoryLimiter(limits Limits) *MemoryLimiter {
	return &MemoryLimiter{
		limits: limits,
		now:    time.Now,
	}
}

// reset は日付が変わっていたら数え直す。mu を持って呼ぶ
func (l *MemoryLimiter) reset(now time.Time) {
	if day := QuotaDay(now); day != l.day {
		l.day = day
		l.daily = make(map[string]int)
		l.minute = make(map[string]int)
		l.calls = 0
	}
}

func (l *MemoryLimiter) AllowQuestion(_ context.Context, clientIP string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.reset(now)

	minuteKey := fmt.Sprintf("%s|%d", clientIP, now.Unix()/60)

	if l.minute[minuteKey] >= l.limits.PerIPPerMinute {
		return fmt.Errorf("%w: per-ip per-minute", ErrRateLimited)
	}
	if l.daily[clientIP] >= l.limits.PerIPPerDay {
		return fmt.Errorf("%w: per-ip per-day", ErrQuotaExceeded)
	}

	l.minute[minuteKey]++
	l.daily[clientIP]++
	return nil
}

func (l *MemoryLimiter) AllowCall(context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.reset(l.now())

	if l.calls >= l.limits.CallsPerDay {
		return fmt.Errorf("%w: app daily calls", ErrQuotaExceeded)
	}

	l.calls++
	return nil
}
