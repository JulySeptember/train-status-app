package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

const (
	kasugaMita   = "odpt.Station:Toei.Mita.Kasuga"
	kasugaOedo   = "odpt.Station:Toei.Oedo.Kasuga"
	asakusa      = "odpt.Station:Toei.Asakusa.Asakusa"
	shinjukuLine = "odpt.Station:Toei.Shinjuku.Shinjuku"
	oshiage      = "odpt.Station:Toei.Asakusa.Oshiage"
	nishiMagome  = "odpt.Station:Toei.Asakusa.NishiMagome"
	oedoRailway  = "odpt.Railway:Toei.Oedo"
	mitaRailway  = "odpt.Railway:Toei.Mita"
	mitaMita     = "odpt.Station:Toei.Mita.Mita"
)

// 2026-10-08（木）10:00。平日ダイヤ
var journeyNow = time.Date(2026, 10, 8, 10, 0, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))

func newJourneyService(t *testing.T) *Service {
	t.Helper()
	return newJourneyServiceWith(t, &mockClient{})
}

func newJourneyServiceWith(t *testing.T, c *mockClient) *Service {
	t.Helper()

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	s := New(c, loader)
	s.now = func() time.Time { return journeyNow }

	return s
}

// assertTimetable は、経路の各区間の乗車時刻が駅時刻表どおりで、
// 区間どうしがつながっていることを確かめる。
func assertTimetable(t *testing.T, s *Service, journeys []Journey) {
	t.Helper()

	calendars := calendar.Calendars(journeyNow)

	departures := make(map[[2]string]string)
	for _, tt := range s.assets.StationTimetables() {
		if !slices.Contains(calendars, tt.Calendar) {
			continue
		}
		for _, obj := range tt.StationTimetableObject {
			if obj.DepartureTime != "" {
				departures[[2]string{tt.Station, obj.Train}] = obj.DepartureTime
			}
		}
	}

	for _, j := range journeys {

		if j.Transfers != len(j.Legs)-1 {
			t.Errorf("transfers = %d, legs = %d", j.Transfers, len(j.Legs))
		}

		if j.DepartureTime != j.Legs[0].DepartureTime ||
			j.ArrivalTime != j.Legs[len(j.Legs)-1].ArrivalTime {
			t.Errorf("journey times %s-%s do not match legs", j.DepartureTime, j.ArrivalTime)
		}

		for i, l := range j.Legs {

			// 遅れを足した時刻から、時刻表の時刻に戻して比べる
			dep, err := parseClock(l.DepartureTime)
			if err != nil {
				t.Fatal(err)
			}
			scheduled := formatClock(dep - l.DelayMinutes)

			got, ok := departures[[2]string{l.From, l.Train}]
			if !ok || got != scheduled {
				t.Errorf("%s at %s: departure %s (delay %d), station timetable %q", l.Train, l.From, l.DepartureTime, l.DelayMinutes, got)
			}

			if l.ArrivalTime < l.DepartureTime {
				t.Errorf("%s: arrives %s before departure %s", l.Train, l.ArrivalTime, l.DepartureTime)
			}

			if i > 0 {
				prev := j.Legs[i-1]
				if l.DepartureTime < prev.ArrivalTime {
					t.Errorf("%s departs %s before arriving %s", l.Train, l.DepartureTime, prev.ArrivalTime)
				}
				if l.From != prev.To && !slices.Contains(s.stationGroups[prev.To], l.From) &&
					!slices.ContainsFunc(differentNameTransfers, func(p [2]string) bool {
						return p == [2]string{prev.To, l.From} || p == [2]string{l.From, prev.To}
					}) {
					t.Errorf("cannot transfer from %s to %s", prev.To, l.From)
				}
			}
		}
	}
}

func TestSearchJourneys(t *testing.T) {

	s := newJourneyService(t)

	tests := []struct {
		name     string
		from, to string
	}{
		// 三田線の春日を指定しても、大江戸線の春日から乗れる
		{"春日 → 浅草", kasugaMita, asakusa},
		// 馬喰横山 ⇔ 東日本橋で乗り換える
		{"新宿 → 押上", shinjukuLine, oshiage},
		{"西馬込 → 押上", nishiMagome, oshiage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := s.SearchJourneys(context.Background(), JourneyQuery{
				From:         tt.from,
				To:           tt.to,
				DepartAt:     "10:00",
				MaxTransfers: route.DefaultMaxTransfers,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(got.Journeys) == 0 {
				t.Fatal("no journeys")
			}

			assertTimetable(t, s, got.Journeys)

			for i, j := range got.Journeys {
				if j.DepartureTime < "10:00" {
					t.Errorf("departs %s before 10:00", j.DepartureTime)
				}
				if i > 0 {
					prev := got.Journeys[i-1]
					if j.Transfers <= prev.Transfers || j.ArrivalTime >= prev.ArrivalTime {
						t.Errorf("journey %d (%d transfers, %s) is not better than %d (%d transfers, %s)",
							i, j.Transfers, j.ArrivalTime, i-1, prev.Transfers, prev.ArrivalTime)
					}
				}
			}
		})
	}
}

func TestSearchJourneysLabels(t *testing.T) {

	s := newJourneyService(t)

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         kasugaMita,
		To:           asakusa,
		DepartAt:     "10:00",
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	l := got.Journeys[0].Legs[0]

	if l.From != kasugaOedo || l.FromName != "春日" {
		t.Errorf("from = %s %s, want %s 春日", l.From, l.FromName, kasugaOedo)
	}

	if l.RailwayName != "大江戸線" {
		t.Errorf("railwayName = %q", l.RailwayName)
	}

	if l.TrainTypeName == "" || l.TrainNumber == "" || l.DestinationName == "" {
		t.Errorf("missing labels: %+v", l)
	}
}

func TestSearchJourneysAvoid(t *testing.T) {

	s := newJourneyService(t)

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         kasugaMita,
		To:           asakusa,
		DepartAt:     "10:00",
		MaxTransfers: route.DefaultMaxTransfers,
		Avoid:        []string{oedoRailway},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Journeys) == 0 {
		t.Fatal("no journeys")
	}

	for _, j := range got.Journeys {
		for _, l := range j.Legs {
			if l.Railway == oedoRailway {
				t.Errorf("uses avoided railway: %+v", l)
			}
		}
	}
}

func TestSearchJourneysArriveBy(t *testing.T) {

	s := newJourneyService(t)

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         kasugaMita,
		To:           asakusa,
		ArriveBy:     "18:00",
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Journeys) == 0 {
		t.Fatal("no journeys")
	}

	assertTimetable(t, s, got.Journeys)

	for _, j := range got.Journeys {
		if j.ArrivalTime > "18:00" {
			t.Errorf("arrives %s after 18:00", j.ArrivalTime)
		}
	}
}

// 時刻の指定が無ければ、現在時刻に出発する
func TestSearchJourneysNow(t *testing.T) {

	s := newJourneyService(t)
	s.now = func() time.Time { return journeyNow.Add(7 * time.Hour) } // 17:00

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         kasugaMita,
		To:           asakusa,
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Journeys) == 0 {
		t.Fatal("no journeys")
	}

	if d := got.Journeys[0].DepartureTime; d < "17:00" || d > "17:30" {
		t.Errorf("departure = %s, want soon after 17:00", d)
	}
}

// 0時台の出発は、その日の運行日の終電として扱う
func TestSearchJourneysAfterMidnight(t *testing.T) {

	s := newJourneyService(t)

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         nishiMagome,
		To:           oshiage,
		DepartAt:     "23:00",
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Journeys) == 0 {
		t.Fatal("no journeys")
	}

	// 終電を過ぎると経路は無い
	got, err = s.SearchJourneys(context.Background(), JourneyQuery{
		From:         nishiMagome,
		To:           oshiage,
		DepartAt:     "02:00",
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Journeys) != 0 {
		t.Errorf("got %d journeys after the last train", len(got.Journeys))
	}
}

func TestSearchJourneysInvalid(t *testing.T) {

	s := newJourneyService(t)

	base := JourneyQuery{
		From:         kasugaMita,
		To:           asakusa,
		MaxTransfers: route.DefaultMaxTransfers,
	}

	tests := []struct {
		name   string
		modify func(q *JourneyQuery)
		want   error
	}{
		{"存在しない駅", func(q *JourneyQuery) { q.From = "odpt.Station:Toei.Mita.Unknown" }, ErrStationNotFound},
		{"同じ名前の駅", func(q *JourneyQuery) { q.To = kasugaOedo }, ErrInvalidJourneyQuery},
		{"出発と到着の両方", func(q *JourneyQuery) { q.DepartAt, q.ArriveBy = "10:00", "11:00" }, ErrInvalidJourneyQuery},
		{"時刻の形式", func(q *JourneyQuery) { q.DepartAt = "10時" }, ErrInvalidJourneyQuery},
		{"時刻の範囲", func(q *JourneyQuery) { q.ArriveBy = "25:00" }, ErrInvalidJourneyQuery},
		{"1桁の時", func(q *JourneyQuery) { q.DepartAt = "9:00" }, ErrInvalidJourneyQuery},
		{"乗り換え回数の上限", func(q *JourneyQuery) { q.MaxTransfers = 4 }, ErrInvalidJourneyQuery},
		{"負の乗り換え回数", func(q *JourneyQuery) { q.MaxTransfers = -1 }, ErrInvalidJourneyQuery},
		{"存在しない路線", func(q *JourneyQuery) { q.Avoid = []string{"odpt.Railway:Toei.Unknown"} }, ErrInvalidJourneyQuery},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			q := base
			tt.modify(&q)

			_, err := s.SearchJourneys(context.Background(), q)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParseClock(t *testing.T) {

	tests := []struct {
		in   string
		want int
	}{
		{"10:00", 600},
		{"23:59", 1439},
		{"00:15", 1455}, // 3時前は翌日
		{"02:59", 1619},
		{"03:00", 180},
	}

	for _, tt := range tests {
		got, err := parseClock(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseClock(%q) = %d, %v, want %d", tt.in, got, err, tt.want)
		}
		if f := formatClock(got); f != tt.in {
			t.Errorf("formatClock(%d) = %q, want %q", got, f, tt.in)
		}
	}
}

func TestSameNameStations(t *testing.T) {

	s := newJourneyService(t)

	if got := s.stationGroups[kasugaMita]; !slices.Equal(got, []string{kasugaMita, kasugaOedo}) {
		t.Errorf("group = %v", got)
	}

	if got := s.stationGroups[asakusa]; !slices.Equal(got, []string{asakusa}) {
		t.Errorf("group = %v", got)
	}

	// 同じ名前の駅は8組
	pairs := 0
	for id, group := range s.stationGroups {
		if len(group) == 2 && group[0] == id {
			pairs++
		}
	}
	if pairs != 8 {
		t.Errorf("same-name pairs = %d, want 8", pairs)
	}
}

// mitaDelayed は、三田線の両方向が delay 秒遅れている列車位置。
func mitaDelayed(delay int) []model.TrainLocation {
	return []model.TrainLocation{
		{Railway: mitaRailway, RailDirection: "odpt.RailDirection:Southbound", Delay: delay},
		{Railway: mitaRailway, RailDirection: "odpt.RailDirection:Southbound", Delay: delay},
		{Railway: mitaRailway, RailDirection: "odpt.RailDirection:Southbound", Delay: 0},
		{Railway: mitaRailway, RailDirection: "odpt.RailDirection:Northbound", Delay: delay},
	}
}

func TestSearchJourneysDelay(t *testing.T) {

	query := JourneyQuery{From: kasugaMita, To: mitaMita, MaxTransfers: 0}

	scheduled, err := newJourneyService(t).SearchJourneys(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}

	s := newJourneyServiceWith(t, &mockClient{trainLocations: mitaDelayed(300)})

	got, err := s.SearchJourneys(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}

	if !got.DelayApplied {
		t.Error("delayApplied = false")
	}

	if len(got.Journeys) != 1 || len(scheduled.Journeys) != 1 {
		t.Fatalf("journeys = %d, %d", len(got.Journeys), len(scheduled.Journeys))
	}

	assertTimetable(t, s, got.Journeys)

	// 中央値の5分遅れ。時刻表で 10:00 より前に出た列車にも乗れるので、時刻表どおりより早く出る
	l := got.Journeys[0].Legs[0]
	if l.Railway != mitaRailway || l.DelayMinutes != 5 {
		t.Errorf("leg = %s delay %d", l.Railway, l.DelayMinutes)
	}
	if l.DepartureTime < "10:00" || l.DepartureTime > scheduled.Journeys[0].DepartureTime {
		t.Errorf("departs %s, scheduled journey departs %s", l.DepartureTime, scheduled.Journeys[0].DepartureTime)
	}
}

func TestSearchJourneysSuspended(t *testing.T) {

	s := newJourneyServiceWith(t, &mockClient{
		trainStatus: []model.TrainStatus{
			{Railway: mitaRailway, TrainInformationText: model.LocalizedString{Ja: "三田線は、人身事故の影響で、運転を見合わせています。"}},
			{Railway: oedoRailway, TrainInformationText: model.LocalizedString{Ja: "現在、１５分以上の遅延はありません。"}},
		},
	})

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:         kasugaMita,
		To:           mitaMita,
		DepartAt:     "10:00",
		MaxTransfers: route.DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []Railway{{ID: mitaRailway, Name: "三田線"}}
	if !slices.Equal(got.SuspendedRailways, want) {
		t.Errorf("suspendedRailways = %v, want %v", got.SuspendedRailways, want)
	}

	// 三田駅には浅草線でも行ける
	if len(got.Journeys) == 0 {
		t.Fatal("no journeys")
	}

	for _, j := range got.Journeys {
		for _, l := range j.Legs {
			if l.Railway == mitaRailway {
				t.Errorf("uses suspended railway: %+v", l)
			}
		}
	}
}

func TestSearchJourneysRealtimeError(t *testing.T) {

	s := newJourneyServiceWith(t, &mockClient{
		trainLocations: mitaDelayed(300),
		statusErr:      errors.New("timeout"),
	})

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{From: kasugaMita, To: mitaMita, DepartAt: "10:00"})
	if err != nil {
		t.Fatal(err)
	}

	if got.DelayApplied {
		t.Error("delayApplied = true")
	}

	if len(got.Journeys) == 0 || got.Journeys[0].Legs[0].DelayMinutes != 0 {
		t.Errorf("journeys = %+v", got.Journeys)
	}
}

func TestRailwayConditionsCache(t *testing.T) {

	c := &mockClient{trainLocations: mitaDelayed(300)}
	s := newJourneyServiceWith(t, c)

	now := journeyNow
	s.now = func() time.Time { return now }

	minutes := func() int {
		t.Helper()
		conds, err := s.railwayConditions(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return conds.delays[0].Minutes
	}

	if got := minutes(); got != 5 {
		t.Fatalf("minutes = %d", got)
	}

	c.trainLocations = mitaDelayed(600)

	now = journeyNow.Add(realtimeTTL - time.Second)
	if got := minutes(); got != 5 {
		t.Errorf("within TTL: minutes = %d, want cached 5", got)
	}

	now = journeyNow.Add(realtimeTTL)
	if got := minutes(); got != 10 {
		t.Errorf("after TTL: minutes = %d, want 10", got)
	}
}

func TestIsSuspended(t *testing.T) {

	tests := []struct {
		text string
		want bool
	}{
		{"現在、１５分以上の遅延はありません。", false},
		{"浅草線は、車両点検の影響で、ダイヤが乱れています。", false},
		{"三田線は、人身事故の影響で、白金高輪駅～目黒駅間の運転を見合わせています。", true},
		{"新宿線は、信号故障の影響で、運転を中止しています。", true},
		{"三田線は、人身事故の影響で運転を見合わせていましたが、運転を再開しました。", false},
	}

	for _, tt := range tests {
		if got := isSuspended(tt.text); got != tt.want {
			t.Errorf("isSuspended(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestMedianDelays(t *testing.T) {

	got := medianDelays([]model.TrainLocation{
		{Railway: "R", RailDirection: "Up", Delay: 0},
		{Railway: "R", RailDirection: "Up", Delay: 400},
		{Railway: "R", RailDirection: "Up", Delay: 120},
		{Railway: "R", RailDirection: "Down", Delay: 20},
		{Railway: "R", RailDirection: "Down", Delay: 600},
		// odpt:delay が null の路線は 0 になり、含めない
		{Railway: "Arakawa", RailDirection: "Up"},
	})

	want := []route.Delay{
		// 20秒と600秒の中央値（2件では後ろの値）
		{Railway: "R", RailDirection: "Down", Minutes: 10},
		{Railway: "R", RailDirection: "Up", Minutes: 2},
	}

	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSearchJourneysTimetableOnly(t *testing.T) {

	s := newJourneyServiceWith(t, &mockClient{
		trainStatus: []model.TrainStatus{
			{Railway: mitaRailway, TrainInformationText: model.LocalizedString{Ja: "三田線は、人身事故の影響で、運転を見合わせています。"}},
		},
		trainLocations: mitaDelayed(300),
	})

	got, err := s.SearchJourneys(context.Background(), JourneyQuery{
		From:          kasugaMita,
		To:            mitaMita,
		DepartAt:      "10:00",
		MaxTransfers:  0,
		TimetableOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.DelayApplied || len(got.SuspendedRailways) != 0 {
		t.Errorf("delayApplied = %v, suspendedRailways = %v", got.DelayApplied, got.SuspendedRailways)
	}

	// 見合わせ中でも三田線で、遅れを足さずに探す
	if len(got.Journeys) != 1 {
		t.Fatalf("journeys = %d", len(got.Journeys))
	}
	if l := got.Journeys[0].Legs[0]; l.Railway != mitaRailway || l.DelayMinutes != 0 {
		t.Errorf("leg = %s delay %d", l.Railway, l.DelayMinutes)
	}

	assertTimetable(t, s, got.Journeys)
}
