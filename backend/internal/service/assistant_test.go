package service

import (
	"errors"
	"testing"
	"time"

	"train-status-app/backend/internal/model"
)

func TestGetRailwayConditions(t *testing.T) {

	normal := "現在、１５分以上の遅延はありません。"

	s := newJourneyServiceWith(t, &mockClient{
		trainStatus: []model.TrainStatus{
			{Railway: "odpt.Railway:Toei.Asakusa", TrainInformationText: model.LocalizedString{Ja: normal}},
			{Railway: mitaRailway, TrainInformationText: model.LocalizedString{Ja: normal}},
			{Railway: oedoRailway, TrainInformationText: model.LocalizedString{Ja: "大江戸線は、信号故障の影響で、運転を見合わせています。"}},
			{Railway: "odpt.Railway:Toei.Shinjuku", TrainInformationText: model.LocalizedString{Ja: "新宿線は、車両故障の影響で、遅れが出ています。"}},
		},
		trainLocations: mitaDelayed(10 * 60),
	})

	got, err := s.GetRailwayConditions(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	states := make(map[string]RailwayCondition)
	for _, c := range got {
		states[c.Railway] = c
	}

	if len(got) != len(s.assets.Railways()) {
		t.Fatalf("expected all railways, got %d", len(got))
	}

	tests := []struct {
		railway string
		state   string
		delay   int
	}{
		{"odpt.Railway:Toei.Asakusa", RailwayNormal, 0},
		{mitaRailway, RailwayDelayed, 10},
		{oedoRailway, RailwaySuspended, 0},
		// 列車の遅れは無いが、運行情報の文章が平常でない
		{"odpt.Railway:Toei.Shinjuku", RailwayDelayed, 0},
		// 運行情報が配信されていない路線は平常とみなす
		{"odpt.Railway:Toei.Arakawa", RailwayNormal, 0},
	}

	for _, tt := range tests {
		c := states[tt.railway]
		if c.State != tt.state || c.DelayMinutes != tt.delay {
			t.Errorf("%s: got %s %d分, want %s %d分", tt.railway, c.State, c.DelayMinutes, tt.state, tt.delay)
		}
		if c.RailwayName == "" {
			t.Errorf("%s: railway name is empty", tt.railway)
		}
	}
}

func TestGetRailwayConditionsError(t *testing.T) {
	s := newJourneyServiceWith(t, &mockClient{statusErr: errors.New("boom")})

	if _, err := s.GetRailwayConditions(t.Context()); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindStations(t *testing.T) {
	s := newJourneyService(t)

	got := s.FindStations("春日駅")
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %v", got)
	}
	for _, c := range got {
		if c.Name != "春日" || c.RailwayName == "" {
			t.Fatalf("unexpected candidate %+v", c)
		}
	}

	if got := s.FindStations("渋谷"); len(got) != 0 {
		t.Fatalf("expected no candidates, got %v", got)
	}
}

func TestGetDeparturesNext(t *testing.T) {
	s := newJourneyService(t)

	got, err := s.GetDepartures(t.Context(), DepartureQuery{
		Station: asakusa,
		Limit:   5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.CalendarName != "平日" || got.StationName != "浅草" || got.RailwayName != "浅草線" {
		t.Fatalf("unexpected header %+v", got)
	}
	if len(got.Departures) != 5 || got.Total <= 5 {
		t.Fatalf("expected 5 or more departures, got %d of %d", len(got.Departures), got.Total)
	}

	prev := -1
	for _, d := range got.Departures {
		// 現在（10:00）以降だけを、時刻順に返す
		if d.MinutesFromNow == nil || *d.MinutesFromNow < 0 || *d.MinutesFromNow < prev {
			t.Fatalf("unexpected departure %+v", d)
		}
		prev = *d.MinutesFromNow
		if d.RailDirectionName == "" || d.TrainID == "" {
			t.Fatalf("missing fields %+v", d)
		}
	}
}

func TestGetDeparturesAfterMidnight(t *testing.T) {
	s := newJourneyService(t)

	// 0:10 は前日（平日）の運行日。0時台の列車を選び、5時台の始発は選ばない
	s.now = func() time.Time { return journeyNow.Add(14*time.Hour + 10*time.Minute) }

	got, err := s.GetDepartures(t.Context(), DepartureQuery{
		Station:       asakusa,
		RailDirection: "odpt.RailDirection:Southbound",
		Limit:         1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Departures) != 1 || got.Departures[0].Time[:2] != "00" {
		t.Fatalf("expected a train after midnight, got %+v", got.Departures)
	}
}

func TestGetDeparturesTimetable(t *testing.T) {
	s := newJourneyService(t)

	// 浅草線は土曜・休日を「土休日」にまとめている
	got, err := s.GetDepartures(t.Context(), DepartureQuery{
		Station:       asakusa,
		RailDirection: "odpt.RailDirection:Northbound",
		Calendar:      "odpt.Calendar:Saturday",
		From:          "08:00",
		To:            "08:59",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.CalendarName != "土休日" || len(got.Departures) == 0 {
		t.Fatalf("unexpected result %+v", got)
	}
	for _, d := range got.Departures {
		if d.Time < "08:00" || d.Time > "08:59" || d.MinutesFromNow != nil {
			t.Fatalf("unexpected departure %+v", d)
		}
	}
}

func TestGetDeparturesErrors(t *testing.T) {
	s := newJourneyService(t)

	if _, err := s.GetDepartures(t.Context(), DepartureQuery{Station: "odpt.Station:Toei.Unknown"}); !errors.Is(err, ErrStationNotFound) {
		t.Fatalf("expected ErrStationNotFound, got %v", err)
	}

	if _, err := s.GetDepartures(t.Context(), DepartureQuery{Station: asakusa, From: "8時"}); !errors.Is(err, ErrInvalidDepartureQuery) {
		t.Fatalf("expected ErrInvalidDepartureQuery, got %v", err)
	}
}
