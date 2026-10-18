// Package notify は、登録した路線の運行情報の変化を Web Push で知らせる（docs/design/login.md 5章）。
//
//   - Service: 通知の条件（路線・曜日・時間帯）と、端末の購読を登録する（API から使う）
//   - Notifier: 定期的に運行状況を確かめ、状態が変わった路線を見ている利用者に送る（通知用の Lambda から使う）
package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"train-status-app/backend/internal/notify/webpush"
)

const (
	// 1人が登録できる通知の条件（路線）の数
	MaxAlerts = 10

	// 1人が登録できる端末の数。超えたら古い端末から外す
	MaxSubscriptions = 5

	// 試しの通知を送れる間隔
	TestInterval = time.Minute
)

var (
	// ErrInvalid は、条件や購読の中身が正しくないこと
	ErrInvalid = errors.New("notify: invalid request")

	// ErrLimit は、通知の条件の上限に達していること
	ErrLimit = errors.New("notify: limit reached")

	// ErrUserNotFound は、利用者がいない（退会したところ）こと
	ErrUserNotFound = errors.New("notify: user not found")

	// ErrNoSubscription は、通知を受け取る端末が無いこと
	ErrNoSubscription = errors.New("notify: no subscription")

	// ErrTooSoon は、試しの通知を続けて送ろうとしたこと
	ErrTooSoon = errors.New("notify: too soon")

	// ErrUnavailable は、通知を送れない（VAPID の鍵が無い）こと
	ErrUnavailable = errors.New("notify: unavailable")
)

// Day は、通知する日の種類。祝日は calendar で判定する
type Day string

const (
	Weekday Day = "weekday"
	Holiday Day = "holiday" // 土休日
)

// Alert は、1つの路線の通知の条件。時間帯は start 以上 end 未満（HH:MM、30分単位、24:00 まで）
type Alert struct {
	RailwayID string `json:"railwayId"`
	Days      []Day  `json:"days"`
	Start     string `json:"start"`
	End       string `json:"end"`
}

// UserAlert は、利用者と通知の条件の組（路線から引いたもの）
type UserAlert struct {
	UserID string
	Alert
}

// Subscription は、保存する端末の購読
type Subscription struct {
	webpush.Subscription
	CreatedAt time.Time
}

// Level は、通知に使う路線の状態
type Level string

const (
	Normal    Level = "normal"
	Delayed   Level = "delayed"
	Suspended Level = "suspended"
)

// Store は、通知の条件・購読・前回の路線の状態の保存先。利用者を消すと（退会）、条件と購読も消える
type Store interface {
	ListAlerts(ctx context.Context, userID string) ([]Alert, error)

	// PutAlert は条件を足すか置き換える。利用者がいなければ ErrUserNotFound
	PutAlert(ctx context.Context, userID string, alert Alert) error

	DeleteAlert(ctx context.Context, userID, railwayID string) error

	// AlertsForRailway は、路線を見ているすべての利用者の条件を返す
	AlertsForRailway(ctx context.Context, railwayID string) ([]UserAlert, error)

	ListSubscriptions(ctx context.Context, userID string) ([]Subscription, error)

	// SubscriptionOwners は、endpoint を購読に登録しているすべての利用者を返す
	SubscriptionOwners(ctx context.Context, endpoint string) ([]string, error)

	// PutSubscription は購読を足すか置き換える（endpoint で見分ける）。利用者がいなければ ErrUserNotFound
	PutSubscription(ctx context.Context, userID string, sub Subscription) error

	DeleteSubscription(ctx context.Context, userID, endpoint string) error

	// TakeTestSlot は、前に試しの通知を送ってから interval たっていれば記録して true を返す
	TakeTestSlot(ctx context.Context, userID string, now time.Time, interval time.Duration) (bool, error)

	// LoadStates は前回の路線の状態と、保存した時刻を返す。まだ無ければ空
	LoadStates(ctx context.Context) (map[string]Level, time.Time, error)

	SaveStates(ctx context.Context, states map[string]Level, at time.Time) error

	// HasUser は、利用者がいる（退会していない）かを返す
	HasUser(ctx context.Context, userID string) (bool, error)
}

// Sender は通知を送る（webpush.Sender、またはテストの偽物）
type Sender interface {
	Send(ctx context.Context, sub webpush.Subscription, payload []byte, ttl time.Duration) error
}

// Message は、Service Worker（frontend/public/sw.js）に送る通知の中身
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`

	// 同じ路線の通知は、新しいもので置き換える
	Tag string `json:"tag"`
}

// parseClock は HH:MM（30分単位、00:00〜24:00）を0時からの分にする
func parseClock(s string) (int, error) {
	var h, m int
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("%w: time %q", ErrInvalid, s)
	}
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil {
		return 0, fmt.Errorf("%w: time %q", ErrInvalid, s)
	}
	minutes := h*60 + m
	if (m != 0 && m != 30) || minutes > 24*60 {
		return 0, fmt.Errorf("%w: time %q", ErrInvalid, s)
	}
	return minutes, nil
}
