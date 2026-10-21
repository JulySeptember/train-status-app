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
	return service.RailwayCondition{
		Railway: railway, State: state, DelayMinutes: delay, Text: text,
		StatusAvailable: true, LocationAvailable: true,
	}
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

	// 見合わせ
	suspended := "三田線は運転を見合わせています。"
	run(cond(mita, service.RailwaySuspended, 0, suspended), cond(oedo, service.RailwayNormal, 0, normalText))
	got := sentTo("u1")
	if len(got) != 2 || got[1].Title != "三田線：運転見合わせ" || got[1].Body != suspended ||
		got[1].URL != "/routes/odpt.Railway:Toei.Mita" || got[1].Tag != "railway:"+mita {
		t.Fatalf("unexpected suspension message: %+v", got)
	}

	// 運行情報（文章）か列車位置を取れなかった回は、前の状態のまま（「平常に戻った」と送らない）
	noStatus := cond(mita, service.RailwayDelayed, 3, "")
	noStatus.StatusAvailable = false
	run(noStatus, cond(oedo, service.RailwayNormal, 0, normalText))
	noLocation := cond(mita, service.RailwaySuspended, 0, suspended)
	noLocation.LocationAvailable = false
	run(noLocation)
	unknown := cond(mita, service.RailwayUnknown, 0, "")
	unknown.StatusAvailable = false
	run(unknown)
	if got := sentTo("u1"); len(got) != 2 {
		t.Fatalf("unavailable conditions should keep the previous state: %+v", got)
	}
	run(cond(mita, service.RailwaySuspended, 0, suspended))
	if got := sentTo("u1"); len(got) != 2 {
		t.Fatalf("still suspended: %+v", got)
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

	// 10分を少し下回っても、5分未満になるまでは遅れのまま（境目で通知を繰り返さない）
	run(cond(mita, service.RailwayDelayed, 8, normalText))
	run(cond(mita, service.RailwayDelayed, 11, normalText))
	if got := sentTo("u1"); len(got) != 3 {
		t.Fatalf("should not flap around the threshold: %+v", got)
	}
	run(cond(mita, service.RailwayDelayed, 4, normalText))
	if got := sentTo("u1"); len(got) != 4 || got[3].Title != "三田線：平常運転に戻りました" {
		t.Fatalf("should recover below 5 minutes: %+v", got)
	}

	// 前回の状態が古い（15分より前）ときは、記録だけする
	now = now.Add(20 * time.Minute)
	run(cond(mita, service.RailwaySuspended, 0, suspended))
	if got := sentTo("u1"); len(got) != 4 {
		t.Fatalf("stale state should not notify: %+v", got)
	}

	// 終電から始発の間は確かめない
	now = time.Date(2026, 10, 20, 2, 0, 0, 0, jst)
	run(cond(mita, service.RailwayNormal, 0, normalText))
	if len(sentTo("u1")) != 4 {
		t.Fatal("should not run at night")
	}
}

// 0時台は、曜日を前日の運行日で決める（土曜 0:30 は金曜の運行日なので平日）
func TestNotifierAfterMidnight(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 10, 24, 0, 30, 0, 0, jst) // 土曜

	store := newStore(t, "u1")
	sender := &fakeSender{}
	conds := &conditions{}
	n := notify.NewNotifier(store, conds, catalog{}, newKeys(t), "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	if err := s.SetAlert(ctx, "u1", notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "00:00", End: "01:00"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/u1")); err != nil {
		t.Fatal(err)
	}

	for _, c := range []service.RailwayCondition{
		cond(mita, service.RailwayNormal, 0, normalText),
		cond(mita, service.RailwaySuspended, 0, "見合わせ"),
	} {
		conds.items = []service.RailwayCondition{c}
		if err := n.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(sender.sent) != 1 {
		t.Fatalf("weekday alert should match at 0:30 on Saturday: %v", sender.sent)
	}
}

// profileGone は、退会の途中で失敗して、利用者の PROFILE だけが消えた状態を表す
type profileGone struct {
	*auth.MemoryStore
	gone string
}

func (p profileGone) HasUser(ctx context.Context, userID string) (bool, error) {
	if userID == p.gone {
		return false, nil
	}
	return p.MemoryStore.HasUser(ctx, userID)
}

// 退会の途中で条件だけが残っていても、送らない。長い本文は切る
func TestNotifierSkipsDeletedUserAndTruncates(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, jst)

	store := newStore(t, "u1", "u2")
	sender := &fakeSender{}
	conds := &conditions{}
	n := notify.NewNotifier(profileGone{store, "u2"}, conds, catalog{}, newKeys(t), "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	for _, u := range []string{"u1", "u2"} {
		if err := s.SetAlert(ctx, u, notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "07:00", End: "09:00"}); err != nil {
			t.Fatal(err)
		}
		if err := s.Subscribe(ctx, u, subscription("https://fcm.googleapis.com/fcm/send/"+u)); err != nil {
			t.Fatal(err)
		}
	}

	long := strings.Repeat("あ", 2000)
	for _, c := range []service.RailwayCondition{
		cond(mita, service.RailwayNormal, 0, normalText),
		cond(mita, service.RailwaySuspended, 0, long),
	} {
		conds.items = []service.RailwayCondition{c}
		if err := n.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	if got := sender.sent["https://fcm.googleapis.com/fcm/send/u2"]; len(got) != 0 {
		t.Fatalf("deleted user should not be notified: %+v", got)
	}
	got := sender.sent["https://fcm.googleapis.com/fcm/send/u1"]
	if len(got) != 1 || len([]rune(got[0].Body)) > 300 || !strings.HasSuffix(got[0].Body, "…") {
		t.Fatalf("body should be truncated: %d", len([]rune(got[0].Body)))
	}
}

// 共用の端末: 同じ端末をほかの利用者が登録したら、前の利用者の購読から外す
func TestSubscribeMovesSharedDevice(t *testing.T) {

	ctx := context.Background()
	store := newStore(t, "u1", "u2")
	s := notify.NewService(store, catalog{}, newKeys(t), "")
	endpoint := "https://fcm.googleapis.com/fcm/send/shared"

	if err := s.Subscribe(ctx, "u1", subscription(endpoint)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.IsSubscribed(ctx, "u1", endpoint); !ok {
		t.Fatal("u1 should be subscribed")
	}
	if ok, _ := s.IsSubscribed(ctx, "u2", endpoint); ok {
		t.Fatal("u2 should not be subscribed yet")
	}

	if err := s.Subscribe(ctx, "u2", subscription(endpoint)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.IsSubscribed(ctx, "u1", endpoint); ok {
		t.Fatal("u1 should lose the shared device")
	}
	if ok, _ := s.IsSubscribed(ctx, "u2", endpoint); !ok {
		t.Fatal("u2 should be subscribed")
	}
}

type flakyKeys struct {
	fail *bool
	keys staticKeys
}

func (f flakyKeys) Keys(ctx context.Context) (*webpush.Keys, error) {
	if *f.fail {
		return nil, errors.New("ssm unavailable")
	}
	return f.keys.Keys(ctx)
}

// 鍵を読めなかった回は状態を保存せず、次の回に送る（通知を失わない）
func TestNotifierRetriesWhenKeysFail(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 10, 19, 8, 0, 0, 0, jst)

	store := newStore(t, "u1")
	sender := &fakeSender{}
	conds := &conditions{}
	fail := false
	n := notify.NewNotifier(store, conds, catalog{}, flakyKeys{&fail, newKeys(t)}, "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	if err := s.SetAlert(ctx, "u1", notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "07:00", End: "09:00"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/u1")); err != nil {
		t.Fatal(err)
	}

	conds.items = []service.RailwayCondition{cond(mita, service.RailwayNormal, 0, normalText)}
	if err := n.Run(ctx); err != nil {
		t.Fatal(err)
	}

	fail = true
	now = now.Add(5 * time.Minute)
	conds.items = []service.RailwayCondition{cond(mita, service.RailwaySuspended, 0, "見合わせ")}
	if err := n.Run(ctx); err == nil {
		t.Fatal("key failure should be reported")
	}

	fail = false
	now = now.Add(5 * time.Minute)
	if err := n.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if got := sender.sent["https://fcm.googleapis.com/fcm/send/u1"]; len(got) != 1 || got[0].Title != "三田線：運転見合わせ" {
		t.Fatalf("change should be sent on the next run: %+v", got)
	}
}

// 夜のあとの最初の回に取れなかった路線は、次に取れた回も記録だけする（夜のうちの変化を朝に送らない）
func TestNotifierAfterNightWithUnavailableRailway(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 10, 19, 0, 50, 0, 0, jst)

	store := newStore(t, "u1")
	sender := &fakeSender{}
	conds := &conditions{}
	n := notify.NewNotifier(store, conds, catalog{}, newKeys(t), "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	if err := s.SetAlert(ctx, "u1", notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "05:00", End: "09:00"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/u1")); err != nil {
		t.Fatal(err)
	}

	run := func(c service.RailwayCondition) {
		t.Helper()
		conds.items = []service.RailwayCondition{c}
		if err := n.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	run(cond(mita, service.RailwaySuspended, 0, "見合わせ"))

	now = time.Date(2026, 10, 19, 5, 0, 0, 0, jst)
	unavailable := cond(mita, service.RailwayNormal, 0, normalText)
	unavailable.LocationAvailable = false
	run(unavailable)

	now = now.Add(5 * time.Minute)
	run(cond(mita, service.RailwayNormal, 0, normalText))

	if len(sender.sent) != 0 {
		t.Fatalf("change during the night should not be sent: %v", sender.sent)
	}
}

// 遅れていた路線が終電を迎えても「平常運転に戻りました」と送らない。始発のあとも、夜の変化として記録だけする
func TestNotifierOutOfHours(t *testing.T) {

	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	now := time.Date(2026, 10, 20, 0, 20, 0, 0, jst) // 火曜（運行日は月曜）

	store := newStore(t, "u1")
	sender := &fakeSender{}
	conds := &conditions{}
	n := notify.NewNotifier(store, conds, catalog{}, newKeys(t), "")
	notify.SetNotifierSender(n, sender)
	notify.SetNotifierClock(n, func() time.Time { return now })

	s := notify.NewService(store, catalog{}, newKeys(t), "")
	if err := s.SetAlert(ctx, "u1", notify.Alert{RailwayID: mita, Days: []notify.Day{notify.Weekday}, Start: "00:00", End: "06:00"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Subscribe(ctx, "u1", subscription("https://fcm.googleapis.com/fcm/send/u1")); err != nil {
		t.Fatal(err)
	}

	run := func(c service.RailwayCondition) {
		t.Helper()
		conds.items = []service.RailwayCondition{c}
		if err := n.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	run(cond(mita, service.RailwayDelayed, 0, "三田線は遅れています。"))

	now = now.Add(5 * time.Minute)
	run(cond(mita, service.RailwayNormal, 0, "ただいまの時間は運行を終了しております。"))
	if len(sender.sent) != 0 {
		t.Fatalf("end of service should not be sent as recovery: %v", sender.sent)
	}

	now = time.Date(2026, 10, 20, 5, 0, 0, 0, jst)
	run(cond(mita, service.RailwayNormal, 0, normalText))
	if len(sender.sent) != 0 {
		t.Fatalf("first check after the night should only record: %v", sender.sent)
	}
}
