package favorite_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/favorite"
)

type catalog struct {
	stations map[string]bool
	railways map[string]bool
}

func (c catalog) HasStation(id string) bool { return c.stations[id] }
func (c catalog) HasRailway(id string) bool { return c.railways[id] }

const (
	asakusa  = "odpt.Station:Toei.Asakusa.Asakusa"
	shinjuku = "odpt.Station:Toei.Oedo.Shinjuku"
	mita     = "odpt.Railway:Toei.Mita"
)

func newService(t *testing.T) (*favorite.Service, *catalog) {
	t.Helper()

	store := auth.NewMemoryStore()
	if _, err := store.EnsureUser(context.Background(), "u1", time.Now()); err != nil {
		t.Fatal(err)
	}

	c := &catalog{
		stations: map[string]bool{asakusa: true, shinjuku: true},
		railways: map[string]bool{mita: true},
	}
	for i := range 40 {
		c.stations[fmt.Sprintf("odpt.Station:Test.S%02d", i)] = true
	}

	return favorite.NewService(store, c), c
}

func TestAddAndList(t *testing.T) {

	ctx := context.Background()
	s, _ := newService(t)

	for _, f := range []struct {
		kind favorite.Kind
		id   string
	}{
		{favorite.Stations, shinjuku},
		{favorite.Stations, asakusa},
		{favorite.Railways, mita},
		{favorite.Journeys, asakusa + "~" + shinjuku},
		{favorite.Stations, shinjuku}, // 2回目は何もしない
	} {
		if err := s.Add(ctx, "u1", f.kind, f.id); err != nil {
			t.Fatalf("add %s %s: %v", f.kind, f.id, err)
		}
	}

	got, err := s.List(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Stations, []string{shinjuku, asakusa}) {
		t.Errorf("stations should be in the order added: %v", got.Stations)
	}
	if !slices.Equal(got.Railways, []string{mita}) {
		t.Errorf("unexpected railways %v", got.Railways)
	}
	if len(got.Journeys) != 1 || got.Journeys[0] != (favorite.JourneyFavorite{From: asakusa, To: shinjuku}) {
		t.Errorf("unexpected journeys %v", got.Journeys)
	}

	if err := s.Remove(ctx, "u1", favorite.Stations, shinjuku); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List(ctx, "u1"); !slices.Equal(got.Stations, []string{asakusa}) {
		t.Errorf("unexpected stations after remove %v", got.Stations)
	}
}

func TestEmptyListIsNotNull(t *testing.T) {
	s, _ := newService(t)
	got, err := s.List(context.Background(), "u1")
	if err != nil || got.Stations == nil || got.Railways == nil || got.Journeys == nil {
		t.Fatalf("lists should be empty, not nil: %+v %v", got, err)
	}
}

func TestAddInvalid(t *testing.T) {

	s, _ := newService(t)

	for _, f := range []struct {
		kind favorite.Kind
		id   string
	}{
		{"trains", asakusa},
		{favorite.Stations, "odpt.Station:Nowhere"},
		{favorite.Stations, mita},
		{favorite.Railways, asakusa},
		{favorite.Journeys, asakusa},
		{favorite.Journeys, asakusa + "~" + asakusa},
		{favorite.Journeys, asakusa + "~odpt.Station:Nowhere"},
		{favorite.Journeys, ""},
	} {
		if err := s.Add(context.Background(), "u1", f.kind, f.id); !errors.Is(err, favorite.ErrInvalid) {
			t.Errorf("add %s %q: want ErrInvalid, got %v", f.kind, f.id, err)
		}
	}

	if err := s.Remove(context.Background(), "u1", "trains", asakusa); !errors.Is(err, favorite.ErrInvalid) {
		t.Errorf("remove with unknown kind: want ErrInvalid, got %v", err)
	}
}

func TestLimit(t *testing.T) {

	ctx := context.Background()
	s, c := newService(t)

	for i := range favorite.Limits[favorite.Stations] {
		if err := s.Add(ctx, "u1", favorite.Stations, fmt.Sprintf("odpt.Station:Test.S%02d", i)); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Add(ctx, "u1", favorite.Stations, asakusa); !errors.Is(err, favorite.ErrLimit) {
		t.Fatalf("want ErrLimit, got %v", err)
	}
	// 登録済みのものは上限でも足せる（何もしない）
	if err := s.Add(ctx, "u1", favorite.Stations, "odpt.Station:Test.S00"); err != nil {
		t.Fatalf("re-adding should succeed: %v", err)
	}
	// ほかの種類は足せる
	if err := s.Add(ctx, "u1", favorite.Railways, mita); err != nil {
		t.Fatal(err)
	}

	// データの更新で無くなった駅は、一覧に出さず、上限にも数えない
	delete(c.stations, "odpt.Station:Test.S00")
	got, _ := s.List(ctx, "u1")
	if len(got.Stations) != favorite.Limits[favorite.Stations]-1 || slices.Contains(got.Stations, "odpt.Station:Test.S00") {
		t.Fatalf("removed station should be hidden: %d %v", len(got.Stations), got.Stations[:1])
	}
	if err := s.Add(ctx, "u1", favorite.Stations, asakusa); err != nil {
		t.Fatalf("removed station should not count: %v", err)
	}
}

func TestAddAfterAccountDeleted(t *testing.T) {
	s := favorite.NewService(auth.NewMemoryStore(), catalog{stations: map[string]bool{asakusa: true}})
	if err := s.Add(context.Background(), "gone", favorite.Stations, asakusa); !errors.Is(err, favorite.ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}
