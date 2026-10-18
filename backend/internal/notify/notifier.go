package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"slices"
	"time"

	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/notify/webpush"
	"train-status-app/backend/internal/service"
)

const (
	// 遅れとみなす、走っている列車の遅れの中央値（分）
	DelayThresholdMinutes = 10

	// この時間帯（JST）は確かめない。終電から始発までは状態が変わっても知らせる意味が薄い
	quietFrom = 1
	quietTo   = 5

	// push サービスが届けるのを待つ時間。古い運行情報は届けない
	messageTTL = 30 * time.Minute
)

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// Conditions は、路線ごとの運行状況を返す（service.Service が満たす）
type Conditions interface {
	GetRailwayConditions(ctx context.Context) ([]service.RailwayCondition, error)
}

type Notifier struct {
	store      Store
	conditions Conditions
	catalog    Catalog
	keys       Keys
	sender     func(*webpush.Keys) Sender
	now        func() time.Time
}

func NewNotifier(store Store, conditions Conditions, catalog Catalog, keys Keys, subject string) *Notifier {
	return &Notifier{
		store:      store,
		conditions: conditions,
		catalog:    catalog,
		keys:       keys,
		sender:     func(k *webpush.Keys) Sender { return webpush.NewSender(k, subject) },
		now:        time.Now,
	}
}

// level は、運行状況を通知の状態にする。運行情報を取れなかった路線は ok が false
func level(c service.RailwayCondition) (Level, bool) {
	switch c.State {
	case service.RailwaySuspended:
		return Suspended, true
	case service.RailwayDelayed:
		// 数分の遅れはよくあるので、文章で遅れを知らせているか、10分以上遅れているときだけ遅れとする
		if c.DelayMinutes >= DelayThresholdMinutes || !service.IsNormalText(c.Text) {
			return Delayed, true
		}
		return Normal, true
	case service.RailwayNormal:
		return Normal, true
	}
	return "", false
}

// Run は、運行状況を確かめ、前回から状態が変わった路線を見ている利用者に知らせる。
// 最初の1回（前回の状態が無い）は記録だけする
func (n *Notifier) Run(ctx context.Context) error {

	now := n.now().In(jst)
	if now.Hour() >= quietFrom && now.Hour() < quietTo {
		return nil
	}

	conds, err := n.conditions.GetRailwayConditions(ctx)
	if err != nil {
		return fmt.Errorf("railway conditions: %w", err)
	}

	prev, err := n.store.LoadStates(ctx)
	if err != nil {
		return fmt.Errorf("load states: %w", err)
	}
	first := len(prev) == 0

	next := make(map[string]Level, len(conds))
	for id, l := range prev {
		next[id] = l
	}

	var changed []service.RailwayCondition
	for _, c := range conds {
		l, ok := level(c)
		if !ok {
			// 取れなかった路線は前の状態のままにする（取れないだけで「平常に戻った」と送らない）
			continue
		}
		if p, seen := prev[c.Railway]; seen && p != l {
			changed = append(changed, c)
		}
		next[c.Railway] = l
	}

	if !first && len(changed) > 0 {
		if err := n.notify(ctx, now, changed, next); err != nil {
			return err
		}
	}

	if err := n.store.SaveStates(ctx, next); err != nil {
		return fmt.Errorf("save states: %w", err)
	}
	return nil
}

func (n *Notifier) notify(ctx context.Context, now time.Time, changed []service.RailwayCondition, levels map[string]Level) error {

	// 鍵が読めなくても、状態は保存して次に進む（同じ変化を後でまとめて送らない）
	if n.keys == nil {
		return nil
	}
	keys, err := n.keys.Keys(ctx)
	if err != nil {
		log.Printf("notify: vapid keys: %v", err)
		return nil
	}
	sender := n.sender(keys)

	day := Weekday
	if !slices.Contains(calendar.Calendars(now), calendar.Weekday) {
		day = Holiday
	}
	minutes := now.Hour()*60 + now.Minute()

	// 利用者ごとに、送る通知をまとめる
	messages := map[string][]Message{}
	for _, c := range changed {
		alerts, err := n.store.AlertsForRailway(ctx, c.Railway)
		if err != nil {
			return fmt.Errorf("alerts for railway: %w", err)
		}
		msg := message(c, levels[c.Railway], n.catalog.RailwayName(c.Railway))
		for _, a := range alerts {
			if matches(a.Alert, day, minutes) {
				messages[a.UserID] = append(messages[a.UserID], msg)
			}
		}
	}

	users, sent := 0, 0
	for userID, msgs := range messages {
		subs, err := n.store.ListSubscriptions(ctx, userID)
		if err != nil {
			log.Printf("notify: list subscriptions: %v", err)
			continue
		}
		if len(subs) == 0 {
			continue
		}
		users++
		for _, m := range msgs {
			payload, err := json.Marshal(m)
			if err != nil {
				return err
			}
			sent += deliver(ctx, n.store, sender, userID, subs, payload, messageTTL)
		}
	}

	// 利用者を特定できる値は出さない
	log.Printf("notify: %d railways changed, %d users, %d notifications sent", len(changed), users, sent)
	return nil
}

// matches は、条件の曜日・時間帯に now が入るかを返す
func matches(a Alert, day Day, minutes int) bool {
	if !slices.Contains(a.Days, day) {
		return false
	}
	start, err1 := parseClock(a.Start)
	end, err2 := parseClock(a.End)
	return err1 == nil && err2 == nil && start <= minutes && minutes < end
}

func message(c service.RailwayCondition, l Level, name string) Message {

	m := Message{
		URL: "/routes/" + url.PathEscape(c.Railway),
		Tag: "railway:" + c.Railway,
	}

	switch l {
	case Suspended:
		m.Title = name + "：運転見合わせ"
		m.Body = c.Text
	case Delayed:
		m.Title = name + "：遅れ"
		m.Body = c.Text
		if service.IsNormalText(c.Text) {
			m.Body = fmt.Sprintf("列車に約%d分の遅れが出ています。", c.DelayMinutes)
		}
	default:
		m.Title = name + "：平常運転に戻りました"
		m.Body = c.Text
	}
	return m
}
