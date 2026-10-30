package service

import (
	"context"
	"sync"
	"time"
)

const (
	// statusTTL は、運行情報（GET /api/status）をキャッシュする時間
	statusTTL = 30 * time.Second

	// trainLocationTTL は、事業者ごとの列車位置（列車の画面）をキャッシュする時間。
	// odpt:Train の配信は30秒ほどの間隔なので、短めにする
	trainLocationTTL = 10 * time.Second

	// fetchRetryAfter は、取得に失敗したあと、取り直すまでの間隔。
	// そのあいだは取りに行かずに同じエラーを返す（ODPT の障害中に、リクエストごとに取得の上限まで待たせない）
	fetchRetryAfter = 5 * time.Second
)

// ttlCache は、ODPT から取得した値をキーごとに ttl のあいだ使い回す。
// 閲覧者の数だけ ODPT を呼ばないようにするため（#135）。
// 同じキーを同時に取りに行くときは、先の取得の結果を待つ。零値のまま使える。
type ttlCache[T any] struct {
	mu      sync.Mutex
	entries map[string]*ttlEntry[T]
}

type ttlEntry[T any] struct {
	mu    sync.Mutex
	at    time.Time
	value T
	ok    bool

	// 前回の取得の失敗と、その時刻
	err      error
	failedAt time.Time
}

// get は、ttl より新しいキャッシュの値か、fetch で取り直した値を返す。
// 取得に失敗したら、fetchRetryAfter のあいだは取りに行かずに同じエラーを返す。
// 取得した時刻・失敗した時刻は、取得を終えたときの now() にする（ODPT が応答せず期限まで待ったときも、そこから数える）。
// 呼び出し側が待てなくなった（期限切れ・キャンセル）だけの失敗は覚えず、次の呼び出しで取り直す。
func (c *ttlCache[T]) get(
	ctx context.Context,
	key string,
	ttl time.Duration,
	now func() time.Time,
	fetch func(context.Context) (T, error),
) (T, error) {

	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*ttlEntry[T])
	}
	e, ok := c.entries[key]
	if !ok {
		e = &ttlEntry[T]{}
		c.entries[key] = e
	}
	c.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	at := now()

	if e.ok && at.Sub(e.at) < ttl {
		return e.value, nil
	}

	if e.err != nil && at.Sub(e.failedAt) < fetchRetryAfter {
		var zero T
		return zero, e.err
	}

	value, err := fetch(ctx)
	if err != nil {
		if ctx.Err() == nil {
			e.err = err
			e.failedAt = now()
		}
		var zero T
		return zero, err
	}

	e.value, e.ok, e.at = value, true, now()
	e.err = nil

	return value, nil
}
