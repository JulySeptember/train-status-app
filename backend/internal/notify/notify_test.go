package notify_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/notify"
	"train-status-app/backend/internal/notify/webpush"
	"train-status-app/backend/internal/service"
)

const (
	mita    = "odpt.Railway:Toei.Mita"
	oedo    = "odpt.Railway:Toei.Oedo"
	unknown = "odpt.Railway:Toei.Nowhere"
)

type catalog struct{}

func (catalog) HasRailway(id string) bool { return id == mita || id == oedo }
func (catalog) RailwayName(id string) string {
	return map[string]string{mita: "三田線", oedo: "大江戸線"}[id]
}

type staticKeys struct{ keys *webpush.Keys }

func (k staticKeys) Keys(context.Context) (*webpush.Keys, error) { return k.keys, nil }

func newKeys(t *testing.T) staticKeys {
	t.Helper()
	// cmd/vapid-key と同じ形の秘密鍵（32 バイトの base64url）
	keys, err := webpush.ParseKeys("q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94")
	if err != nil {
		t.Fatal(err)
	}
	return staticKeys{keys}
}

// fakeSender は送った通知を記録する。gone の endpoint には ErrGone を返す
type fakeSender struct {
	mu   sync.Mutex
	sent map[string][]notify.Message
	gone map[string]bool
}

func (f *fakeSender) Send(_ context.Context, sub webpush.Subscription, payload []byte, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.gone[sub.Endpoint] {
		return webpush.ErrGone
	}
	var m notify.Message
	if err := json.Unmarshal(payload, &m); err != nil {
		return err
	}
	if f.sent == nil {
		f.sent = map[string][]notify.Message{}
	}
	f.sent[sub.Endpoint] = append(f.sent[sub.Endpoint], m)
	return nil
}

func subscription(endpoint string) webpush.Subscription {
	var s webpush.Subscription
	s.Endpoint = endpoint
	// P-256 の点（65 バイト）と 16 バイトの auth。中身は送らないので何でもよい
	s.Keys.P256dh = base64.RawURLEncoding.EncodeToString(append([]byte{4}, make([]byte, 64)...))
	s.Keys.Auth = base64.RawURLEncoding.EncodeToString(make([]byte, 16))
	return s
}

func newStore(t *testing.T, users ...string) *auth.MemoryStore {
	t.Helper()
	store := auth.NewMemoryStore()
	for _, u := range users {
		if _, err := store.EnsureUser(context.Background(), u, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	return store
}

func TestSetAlert(t *testing.T) {

	ctx := context.Background()
	s := notify.NewService(newStore(t, "u1"), catalog{}, nil, "")

	valid := notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "07:00", End: "09:30"}
	if err := s.SetAlert(ctx, "u1", valid); err != nil {
		t.Fatal(err)
	}

	// 同じ路線は置き換える
	valid.End = "24:00"
	if err := s.SetAlert(ctx, "u1", valid); err != nil {
		t.Fatal(err)
	}
	alerts, _ := s.Alerts(ctx, "u1")
	if len(alerts) != 1 || alerts[0].End != "24:00" {
		t.Fatalf("unexpected alerts %+v", alerts)
	}

	for name, a := range map[string]notify.Alert{
		"unknown railway":  {RailwayID: unknown, Days: []notify.Day{notify.Weekday}, Start: "07:00", End: "09:00"},
		"no days":          {RailwayID: mita, Start: "07:00", End: "09:00"},
		"duplicate days":   {RailwayID: mita, Days: []notify.Day{notify.Weekday, notify.Weekday}, Start: "07:00", End: "09:00"},
		"unknown day":      {RailwayID: mita, Days: []notify.Day{"sunday"}, Start: "07:00", End: "09:00"},
		"not 30-min step":  {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "07:15", End: "09:00"},
		"start after end":  {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "09:00", End: "07:00"},
		"empty window":     {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "09:00", End: "09:00"},
		"after 24:00":      {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "23:00", End: "24:30"},
		"malformed time":   {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "7:00", End: "09:00"},
		"non-numeric time": {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "ab:cd", End: "09:00"},
	} {
		if err := s.SetAlert(ctx, "u1", a); !errors.Is(err, notify.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
}

func TestSubscribe(t *testing.T) {

	ctx := context.Background()
	store := newStore(t, "u1")
	s := notify.NewService(store, catalog{}, newKeys(t), "")

	for _, endpoint := range []string{
		"https://fcm.googleapis.com/fcm/send/a",
		"https://updates.push.services.mozilla.com/wpush/v2/b",
		"https://web.push.apple.com/c",
	} {
		if err := s.Subscribe(ctx, "u1", subscription(endpoint)); err != nil {
			t.Errorf("%s: %v", endpoint, err)
		}
	}

	// 利用者が送った任意の URL には送らない
	for _, endpoint := range []string{
		"http://fcm.googleapis.com/fcm/send/a",
		"https://evil.example.com/push",
		"https://fcm.googleapis.com.evil.example.com/x",
		"https://fcm.googleapis.com:8443/x",
		"https://user@fcm.googleapis.com/x",
		"https://169.254.169.254/latest",
		"not a url",
	} {
		if err := s.Subscribe(ctx, "u1", subscription(endpoint)); !errors.Is(err, notify.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", endpoint, err)
		}
	}

	bad := subscription("https://fcm.googleapis.com/fcm/send/z")
	bad.Keys.Auth = "AAAA"
	if err := s.Subscribe(ctx, "u1", bad); !errors.Is(err, notify.ErrInvalid) {
		t.Errorf("short auth: want ErrInvalid, got %v", err)
	}

	// 上限を超えたら、いちばん古い端末を外す
	for i := range notify.MaxSubscriptions {
		if err := s.Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/n"+string(rune('0'+i)))); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	subs, _ := store.ListSubscriptions(ctx, "u1")
	if len(subs) != notify.MaxSubscriptions {
		t.Fatalf("want %d subscriptions, got %d", notify.MaxSubscriptions, len(subs))
	}
	for _, sub := range subs {
		if strings.HasSuffix(sub.Endpoint, "/a") {
			t.Fatal("oldest subscription should be removed")
		}
	}

	// 鍵が無ければ購読できない
	if err := notify.NewService(store, catalog{}, nil, "").Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/a")); !errors.Is(err, notify.ErrUnavailable) {
		t.Errorf("want ErrUnavailable, got %v", err)
	}
}

func TestSendTest(t *testing.T) {

	ctx := context.Background()
	store := newStore(t, "u1")
	sender := &fakeSender{gone: map[string]bool{"https://fcm.googleapis.com/fcm/send/gone": true}}
	s := notify.NewService(store, catalog{}, newKeys(t), "")
	notify.SetServiceSender(s, sender)

	if _, err := s.SendTest(ctx, "u1"); !errors.Is(err, notify.ErrNoSubscription) {
		t.Fatalf("want ErrNoSubscription, got %v", err)
	}

	for _, e := range []string{"https://fcm.googleapis.com/fcm/send/ok", "https://fcm.googleapis.com/fcm/send/gone"} {
		if err := s.Subscribe(ctx, "u1", subscription(e)); err != nil {
			t.Fatal(err)
		}
	}

	n, err := s.SendTest(ctx, "u1")
	if err != nil || n != 1 {
		t.Fatalf("want 1 sent, got %d %v", n, err)
	}
	// 無くなった購読は消す
	if subs, _ := store.ListSubscriptions(ctx, "u1"); len(subs) != 1 {
		t.Fatalf("gone subscription should be removed: %+v", subs)
	}
	// 続けては送れない
	if _, err := s.SendTest(ctx, "u1"); !errors.Is(err, notify.ErrTooSoon) {
		t.Fatalf("want ErrTooSoon, got %v", err)
	}
}

type conditions struct{ items []service.RailwayCondition }

func (c *conditions) GetRailwayConditions(context.Context) ([]service.RailwayCondition, error) {
	return c.items, nil
}

func cond(railway, state string, delay int, text string) service.RailwayCondition {
	return service.RailwayCondition{Railway: railway, State: state, DelayMinutes: delay, Text: text}
}

const normalText = "現在、１５分以上の遅延はありません。"

func TestNotifier(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	// 2026-10-19 は月曜日（平日）
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, jst)

	store := newStore(t, "u1", "u2", "u3")
	sender := &fakeSender{}
	conds := &conditions{}
	n := notify.NewNotifier(store, conds, catalog{}, newKeys(t), "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	// u1: 平日の朝の三田線、u2: 土休日だけの三田線、u3: 平日の夜の三田線
	for user, a := range map[string]notify.Alert{
		"u1": {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "07:00", End: "09:30"},
		"u2": {RailwayID: mita, Days: []notify.Day{notify.Holiday}, Start: "07:00", End: "09:30"},
		"u3": {RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "17:00", End: "20:00"},
	} {
		if err := s.SetAlert(ctx, user, a); err != nil {
			t.Fatal(err)
		}
		if err := s.Subscribe(ctx, user, subscription("https://fcm.googleapis.com/fcm/send/"+user)); err != nil {
			t.Fatal(err)
		}
	}

	run := func(items ...service.RailwayCondition) {
		t.Helper()
		conds.items = items
		if err := n.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	sentTo := func(user string) []notify.Message {
		return sender.sent["https://fcm.googleapis.com/fcm/send/"+user]
	}

	// 最初の1回は記録だけ
	run(cond(mita, service.RailwayDelayed, 20, "三田線は遅れています。"), cond(oedo, service.RailwayNormal, 0, normalText))
	if len(sender.sent) != 0 {
		t.Fatalf("first run should not notify: %v", sender.sent)
	}

	// 数分の遅れ（文章は平常）は平常のまま
	run(cond(mita, service.RailwayNormal, 0, normalText), cond(oedo, service.RailwayDelayed, 3, normalText))
	if got := sentTo("u1"); len(got) != 1 || got[0].Title != "三田線：平常運転に戻りました" {
		t.Fatalf("u1 should be notified of recovery: %+v", got)
	}
	if len(sentTo("u2")) != 0 || len(sentTo("u3")) != 0 {
		t.Fatal("u2 (holiday only) and u3 (evening) should not be notified")
	}

	// 運行情報を取れなかった路線は、前の状態のまま（知らせない）
	run(cond(mita, service.RailwayUnknown, 0, ""), cond(oedo, service.RailwayNormal, 0, normalText))
	run(cond(mita, service.RailwayNormal, 0, normalText), cond(oedo, service.RailwayNormal, 0, normalText))
	if got := sentTo("u1"); len(got) != 1 {
		t.Fatalf("unknown should keep the previous state: %+v", got)
	}

	// 見合わせ
	run(cond(mita, service.RailwaySuspended, 0, "三田線は運転を見合わせています。"), cond(oedo, service.RailwayNormal, 0, normalText))
	got := sentTo("u1")
	if len(got) != 2 || got[1].Title != "三田線：運転見合わせ" || got[1].Body != "三田線は運転を見合わせています。" ||
		got[1].URL != "/routes/odpt.Railway:Toei.Mita" || got[1].Tag != "railway:"+mita {
		t.Fatalf("unexpected suspension message: %+v", got)
	}

	// 同じ状態が続く間は送らない
	run(cond(mita, service.RailwaySuspended, 0, "三田線は運転を見合わせています。再開は10時ごろ。"), cond(oedo, service.RailwayNormal, 0, normalText))
	if len(sentTo("u1")) != 2 {
		t.Fatal("same state should not notify again")
	}

	// 文章は平常でも、10分以上の遅れは遅れ
	run(cond(mita, service.RailwayDelayed, 12, normalText), cond(oedo, service.RailwayNormal, 0, normalText))
	if got := sentTo("u1"); len(got) != 3 || got[2].Title != "三田線：遅れ" || got[2].Body != "列車に約12分の遅れが出ています。" {
		t.Fatalf("unexpected delay message: %+v", got)
	}

	// 終電から始発の間は確かめない
	now = time.Date(2026, 10, 20, 2, 0, 0, 0, jst)
	run(cond(mita, service.RailwayNormal, 0, normalText))
	if len(sentTo("u1")) != 3 {
		t.Fatal("should not run at night")
	}
}
