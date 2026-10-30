package service

import (
	"context"
	"errors"
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
		v, err := c.get(context.Background(), key, 10*time.Second, now, fetch)
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

	calls := 0
	failure := errors.New("timeout")
	fetch := func(context.Context) (int, error) {
		calls++
		return 0, failure
	}

	for _, at := range []time.Duration{0, fetchRetryAfter - time.Second} {
		_, err := c.get(context.Background(), "", time.Minute, now.Add(at), fetch)
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("at %v: err = %v, calls = %d", at, err, calls)
		}
	}

	// fetchRetryAfter を過ぎたら取り直す
	v, err := c.get(context.Background(), "", time.Minute, now.Add(fetchRetryAfter), func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("retry = %d, %v", v, err)
	}
}

// 呼び出し側がキャンセルしただけの失敗は覚えず、次の呼び出しで取り直す
func TestTTLCacheCanceled(t *testing.T) {

	var c ttlCache[int]

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.get(ctx, "", time.Minute, journeyNow, func(ctx context.Context) (int, error) { return 0, ctx.Err() }); err == nil {
		t.Fatal("expected error")
	}

	v, err := c.get(context.Background(), "", time.Minute, journeyNow, func(context.Context) (int, error) { return 7, nil })
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

	for range 2 {
		if _, err := s.GetTrainLocation(context.Background(), "odpt.Train:Toei.Asakusa.1001T"); err != nil && !errors.Is(err, ErrTrainNotFound) {
			t.Fatal(err)
		}
	}

	if len(m.locationOperators) > 1 {
		t.Fatalf("operators = %v, want at most one call", m.locationOperators)
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
