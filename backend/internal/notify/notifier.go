package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"slices"
	"sync"
	"time"

	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/notify/webpush"
	"train-status-app/backend/internal/service"
)

const (
	// 遅れとみなす、走っている列車の遅れの中央値（分）
	DelayThresholdMinutes = 10

	// 遅れから平常に戻すのは、中央値がこれより小さくなってから（10分の前後を行き来するたびに知らせないように）
	RecoverThresholdMinutes = 5

	// この時間帯（JST）は確かめない。終電から始発までは状態が変わっても知らせる意味が薄い
	quietFrom = 1
	quietTo   = 5

	// 路線の前回の状態がこれより古ければ（夜の止める時間帯・配信が止まっていたあと）、比べずに記録だけする
	staleAfter = 15 * time.Minute

	// push サービスが届けるのを待つ時間。古い運行情報は届けない
	messageTTL = 30 * time.Minute

	// 同時に送る利用者の数
	sendParallelism = 8

	// 通知の本文の長さ（文字）。運行情報の文章が長いときは切る（暗号化したレコードに収める）
	maxBodyRunes = 300
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

// level は、運行状況を通知の状態にする。prev は前回の状態（無ければ空）。
// 運行情報（文章）か列車位置（遅れ）を今回取れなかった路線は ok が false（前回の状態のままにする。
// 取れないだけで「平常に戻った」と送らないため）
func level(c service.RailwayCondition, prev Level) (Level, bool) {

	if !c.StatusAvailable || !c.LocationAvailable {
		return "", false
	}

	switch {
	case c.State == service.RailwaySuspended:
		return Suspended, true
	case !service.IsNormalText(c.Text):
		return Delayed, true
	case c.DelayMinutes >= DelayThresholdMinutes:
		return Delayed, true
	case prev == Delayed && c.DelayMinutes >= RecoverThresholdMinutes:
		return Delayed, true
	}
	return Normal, true
}

// Run は、運行状況を確かめ、前回から状態が変わった路線を見ている利用者に知らせる。
// 前回の状態が無いか古い路線は、記録だけする
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

	next := make(map[string]State, len(conds))
	for id, st := range prev {
		next[id] = st
	}

	var changed []service.RailwayCondition
	for _, c := range conds {
		p, seen := prev[c.Railway]
		l, ok := level(c, p.Level)
		if !ok {
			// 取れなかった路線は、前の状態と確かめた時刻のままにする
			continue
		}
		// 前に確かめたのが古ければ（夜の止める時間帯・配信が止まっていたあと）、比べずに記録だけする。
		// 時間帯の外で起きた変化を、あとからまとめて送らないため
		if seen && p.Level != l && now.Sub(p.CheckedAt) <= staleAfter {
			changed = append(changed, c)
		}
		next[c.Railway] = State{Level: l, CheckedAt: now}
	}

	// 送る相手と内容は、状態を保存する前に決める。鍵や条件を読めなければ、保存せずに終わり、次の回に送る
	messages, sender, err := n.prepare(ctx, now, changed, next)
	if err != nil {
		return err
	}

	// 状態は送る前に保存する（送信の途中で止まっても、次の回に同じ通知を送り直さない）
	if err := n.store.SaveStates(ctx, next); err != nil {
		return fmt.Errorf("save states: %w", err)
	}

	if len(messages) > 0 {
		n.send(ctx, sender, messages, len(changed))
	}
	return nil
}

// prepare は、変わった路線を見ていて、曜日・時間帯の合う利用者ごとに送る通知をまとめる
func (n *Notifier) prepare(ctx context.Context, now time.Time, changed []service.RailwayCondition, states map[string]State) (map[string][]Message, Sender, error) {

	if len(changed) == 0 || n.keys == nil {
		return nil, nil, nil
	}
	keys, err := n.keys.Keys(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("vapid keys: %w", err)
	}

	day := Weekday
	if !slices.Contains(calendar.Calendars(now), calendar.Weekday) {
		day = Holiday
	}
	minutes := now.Hour()*60 + now.Minute()

	messages := map[string][]Message{}
	for _, c := range changed {
		alerts, err := n.store.AlertsForRailway(ctx, c.Railway)
		if err != nil {
			return nil, nil, fmt.Errorf("alerts for railway: %w", err)
		}
		msg := message(c, states[c.Railway].Level, n.catalog.RailwayName(c.Railway))
		for _, a := range alerts {
			if matches(a.Alert, day, minutes) {
				messages[a.UserID] = append(messages[a.UserID], msg)
			}
		}
	}
	return messages, n.sender(keys), nil
}

// send は、利用者ごとに並列に送る
func (n *Notifier) send(ctx context.Context, sender Sender, messages map[string][]Message, changed int) {

	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		users, sent int
		sem         = make(chan struct{}, sendParallelism)
	)
	for userID, msgs := range messages {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()

			count := n.sendToUser(ctx, sender, userID, msgs)
			if count > 0 {
				mu.Lock()
				users++
				sent += count
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	// 利用者を特定できる値は出さない
	log.Printf("notify: %d railways changed, %d users, %d notifications sent", changed, users, sent)
}

// sendToUser は、利用者のすべての端末に通知を送り、送れた数を返す
func (n *Notifier) sendToUser(ctx context.Context, sender Sender, userID string, msgs []Message) int {

	// 退会の途中で失敗して条件だけが残っていても、送らない
	ok, err := n.store.HasUser(ctx, userID)
	if err != nil || !ok {
		if err != nil {
			log.Printf("notify: has user: %v", err)
		}
		return 0
	}

	subs, err := n.store.ListSubscriptions(ctx, userID)
	if err != nil {
		log.Printf("notify: list subscriptions: %v", err)
		return 0
	}

	sent := 0
	for _, m := range msgs {
		payload, err := json.Marshal(m)
		if err != nil {
			log.Printf("notify: marshal: %v", err)
			continue
		}
		sent += deliver(ctx, n.store, sender, userID, subs, payload, messageTTL)
	}
	return sent
}

// matches は、条件の曜日・時間帯に now が入るかを返す。
// 曜日は運行日（3時前は前日）で、時間帯は時計の時刻で比べる（0時台は前日の運行日の 00:00〜）
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

	if r := []rune(m.Body); len(r) > maxBodyRunes {
		m.Body = string(r[:maxBodyRunes-1]) + "…"
	}
	return m
}
