package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/model"
)

func TestTTLCache(t *testing.T) {

	var c ttlCache[int]
	now := journeyNow

	calls := 0
	fetch := func(context.Context) (int, error) {
		calls++
		return calls, nil
	}

	get := func(key string) int {
		t.Helper()
		v, err := c.get(context.Background(), key, 10*time.Second, func() time.Time { return now }, fetch)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	if v := get("a"); v != 1 {
		t.Fatalf("first = %d", v)
	}

	// ttl のあいだは取りに行かない
	now = journeyNow.Add(10*time.Second - time.Nanosecond)
	if v := get("a"); v != 1 || calls != 1 {
		t.Fatalf("cached = %d, calls = %d", v, calls)
	}

	// キーごとに取る
	if v := get("b"); v != 2 {
		t.Fatalf("other key = %d", v)
	}

	// ttl を過ぎたら取り直す
	now = journeyNow.Add(10 * time.Second)
	if v := get("a"); v != 3 {
		t.Fatalf("expired = %d", v)
	}
}

func TestTTLCacheError(t *testing.T) {

	var c ttlCache[int]
	now := journeyNow
	clock := func() time.Time { return now }

	calls := 0
	failure := errors.New("timeout")

	// 取得に10秒かかって失敗する（ODPT が応答せず期限まで待った）。失敗は取得を終えた時刻から数える
	fetch := func(context.Context) (int, error) {
		calls++
		now = now.Add(10 * time.Second)
		return 0, failure
	}

	if _, err := c.get(context.Background(), "", time.Minute, clock, fetch); !errors.Is(err, failure) {
		t.Fatalf("err = %v", err)
	}
	failedAt := now

	now = failedAt.Add(fetchRetryAfter - time.Second)
	if _, err := c.get(context.Background(), "", time.Minute, clock, fetch); !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("within retry interval: err = %v, calls = %d", err, calls)
	}

	// fetchRetryAfter を過ぎたら取り直す
	now = failedAt.Add(fetchRetryAfter)
	v, err := c.get(context.Background(), "", time.Minute, clock, func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("retry = %d, %v", v, err)
	}
}

// 呼び出し側がキャンセルしただけの失敗は覚えず、次の呼び出しで取り直す
func TestTTLCacheCanceled(t *testing.T) {

	var c ttlCache[int]

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	clock := func() time.Time { return journeyNow }

	if _, err := c.get(ctx, "", time.Minute, clock, func(ctx context.Context) (int, error) { return 0, ctx.Err() }); err == nil {
		t.Fatal("expected error")
	}

	v, err := c.get(context.Background(), "", time.Minute, clock, func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("after cancel = %d, %v", v, err)
	}
}

// 運行情報（GET /api/status）は statusTTL のあいだ使い回す
func TestGetTrainStatusCache(t *testing.T) {

	m := &mockClient{trainStatus: []model.TrainStatus{
		{Railway: mitaRailway, TrainInformationText: model.LocalizedString{Ja: "現在、１５分以上の遅延はありません。"}},
	}}
	s := newJourneyServiceWith(t, m)

	now := journeyNow
	s.now = func() time.Time { return now }

	for range 2 {
		if _, err := s.GetTrainStatus(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if m.statusCalls != 1 {
		t.Fatalf("calls = %d, want 1", m.statusCalls)
	}

	now = journeyNow.Add(statusTTL)
	if _, err := s.GetTrainStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.statusCalls != 2 {
		t.Fatalf("calls = %d, want 2", m.statusCalls)
	}
}

// 列車位置は事業者ごとに trainLocationTTL のあいだ使い回す
func TestGetTrainLocationCache(t *testing.T) {

	m := &mockClient{}
	s := newJourneyServiceWith(t, m)

	now := journeyNow
	s.now = func() time.Time { return now }

	trainID, _ := findTrain(t, s.assets, mitaRailway)

	get := func() {
		t.Helper()
		if _, err := s.GetTrainLocation(context.Background(), trainID); err != nil {
			t.Fatal(err)
		}
	}

	get()
	get()
	if !slices.Equal(m.locationOperators, []string{"Toei"}) {
		t.Fatalf("operators = %v, want one call", m.locationOperators)
	}

	now = journeyNow.Add(trainLocationTTL)
	get()
	if len(m.locationOperators) != 2 {
		t.Fatalf("operators = %v, want two calls", m.locationOperators)
	}
}

// 通知用の Service は、時刻表を読まずに、New と同じ運行状況を返す
func TestNewConditions(t *testing.T) {

	m := &mockClient{
		trainStatus: []model.TrainStatus{
			{Railway: mitaRailway, TrainInformationText: model.LocalizedString{Ja: "三田線は、人身事故の影響で、運転を見合わせています。"}},
		},
		trainLocations: mitaDelayed(600),
	}

	loader, err := assets.New(assets.WithoutTimetables())
	if err != nil {
		t.Fatal(err)
	}

	light := NewConditions(m, loader)
	light.now = func() time.Time { return journeyNow }

	got, err := light.GetRailwayConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want, err := newJourneyServiceWith(t, m).GetRailwayConditions(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("conditions = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%d: %+v, want %+v", i, got[i], want[i])
		}
	}

	if !light.HasRailway(mitaRailway) || light.RailwayName(mitaRailway) != "三田線" {
		t.Error("railway names are not available")
	}
}
