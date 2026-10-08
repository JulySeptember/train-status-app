package route

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"train-status-app/backend/assets/slim"
)

const (
	weekday = "Weekday"
	holiday = "Holiday"
)

// testTrain は架空の列車。stops は "駅 HH:MM HH:MM"（到着・発車。無い時刻は "-"）。
type testTrain struct {
	id       string
	railway  string
	calendar string
	stops    []string
}

func buildTimetables(t *testing.T, trains []testTrain) *slim.TrainTimetables {
	t.Helper()

	tt := &slim.TrainTimetables{Strings: []string{""}}
	index := map[string]int32{"": 0}

	str := func(s string) int32 {
		if i, ok := index[s]; ok {
			return i
		}
		i := int32(len(tt.Strings))
		tt.Strings = append(tt.Strings, s)
		index[s] = i
		return i
	}

	minutes := func(s string) int16 {
		if s == "-" {
			return slim.NoTime
		}
		var h, m int
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			t.Fatalf("invalid time %q", s)
		}
		return int16(h*60 + m)
	}

	for _, tr := range trains {
		train := slim.Train{
			Train:       str(tr.id),
			TrainNumber: str(tr.id),
			Railway:     str(tr.railway),
			Calendar:    str(tr.calendar),
			TrainType:   str("Local"),
		}

		for _, s := range tr.stops {
			var station, arr, dep string
			if _, err := fmt.Sscanf(s, "%s %s %s", &station, &arr, &dep); err != nil {
				t.Fatalf("invalid stop %q", s)
			}
			train.Stops = append(train.Stops, slim.Stop{
				Station:   str(station),
				Arrival:   minutes(arr),
				Departure: minutes(dep),
			})
		}

		train.Destination = train.Stops[len(train.Stops)-1].Station

		tt.Trains = append(tt.Trains, train)
	}

	return tt
}

// 路線 A: A1 - A2 - A3（各停と、A2 始発の急行）
// 路線 B: B2 - B3（B2 は A2 と乗り換えられる。5分）
// 路線 L: O - T - a - b - T（大江戸線のように、同じ駅 T を2回通る）
var testTrains = []testTrain{
	{"A.local1000", "A", weekday, []string{"A1 - 10:00", "A2 10:10 10:10", "A3 10:30 -"}},
	{"A.local1005", "A", weekday, []string{"A1 - 10:05", "A2 10:15 10:15", "A3 10:35 -"}},
	{"A.express1012", "A", weekday, []string{"A2 - 10:12", "A3 10:18 -"}},
	{"A.local1100", "A", weekday, []string{"A1 - 11:00", "A2 11:10 11:10", "A3 11:30 -"}},
	{"A.holiday", "A", holiday, []string{"A1 - 09:00", "A2 09:10 09:10", "A3 09:30 -"}},

	{"B.1014", "B", weekday, []string{"B2 - 10:14", "B3 10:24 -"}},
	{"B.1020", "B", weekday, []string{"B2 - 10:20", "B3 10:30 -"}},
	{"B.1040", "B", weekday, []string{"B2 - 10:40", "B3 10:50 -"}},

	{"L.loop", "L", weekday, []string{"O - 10:00", "T 10:05 10:05", "a 10:10 10:10", "b 10:15 10:15", "T 10:20 -"}},
}

var testTransfers = []Transfer{
	{From: "A2", To: "B2", Minutes: 5},
	{From: "B2", To: "A2", Minutes: 5},
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	return New(buildTimetables(t, testTrains), testTransfers, DefaultConfig())
}

func hm(s string) int {
	var h, m int
	fmt.Sscanf(s, "%d:%d", &h, &m)
	return h*60 + m
}

// summary は経路を「列車@乗車駅>降車駅」の並びにする
func summary(journeys []Journey) [][]string {
	result := make([][]string, 0, len(journeys))
	for _, j := range journeys {
		var legs []string
		for _, l := range j.Legs {
			legs = append(legs, fmt.Sprintf("%s@%s>%s", l.Train, l.From, l.To))
		}
		result = append(result, legs)
	}
	return result
}

func TestSearch(t *testing.T) {

	e := newTestEngine(t)

	tests := []struct {
		name  string
		query Query
		want  [][]string
	}{
		{
			name:  "乗り換えなし",
			query: Query{From: []string{"A1"}, To: []string{"A2"}, Time: hm("09:58"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1000@A1>A2"}},
		},
		{
			// 各停で A3 まで行くより、A2 で急行に乗り継いだほうが早い
			name:  "乗り換え回数ごとの結果",
			query: Query{From: []string{"A1"}, To: []string{"A3"}, Time: hm("09:58"), MaxTransfers: DefaultMaxTransfers},
			want: [][]string{
				{"A.local1000@A1>A3"},
				{"A.local1000@A1>A2", "A.express1012@A2>A3"},
			},
		},
		{
			name:  "乗り換え回数の上限",
			query: Query{From: []string{"A1"}, To: []string{"A3"}, Time: hm("09:58"), MaxTransfers: 0},
			want:  [][]string{{"A.local1000@A1>A3"}},
		},
		{
			// 10:00 の列車でも 10:05 の列車でも B.1020 に乗れるので、遅く出る 10:05 の列車を選ぶ
			name:  "別の路線への乗り換え",
			query: Query{From: []string{"A1"}, To: []string{"B3"}, Time: hm("09:58"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1005@A1>A2", "B.1020@B2>B3"}},
		},
		{
			name:  "到着時刻の指定",
			query: Query{From: []string{"A1"}, To: []string{"B3"}, Time: hm("10:45"), ArriveBy: true, MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1005@A1>A2", "B.1020@B2>B3"}},
		},
		{
			// 各停では 10:20 までに着かないので、急行に乗り継ぐ経路だけになる
			name:  "到着時刻の指定で乗り継ぐ",
			query: Query{From: []string{"A1"}, To: []string{"A3"}, Time: hm("10:20"), ArriveBy: true, MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1000@A1>A2", "A.express1012@A2>A3"}},
		},
		{
			// 乗り継いでも同じ列車で出るので、乗り換えの多い経路は返さない
			name:  "到着時刻の指定で出発が遅くならない乗り換え",
			query: Query{From: []string{"A1"}, To: []string{"A3"}, Time: hm("10:32"), ArriveBy: true, MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1000@A1>A3"}},
		},
		{
			name:  "路線を避ける",
			query: Query{From: []string{"A2"}, To: []string{"B3"}, Time: hm("10:00"), Avoid: []string{"B"}, MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{},
		},
		{
			name:  "終電後",
			query: Query{From: []string{"A1"}, To: []string{"A2"}, Time: hm("11:01"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{},
		},
		{
			name:  "ダイヤ種別",
			query: Query{From: []string{"A1"}, To: []string{"A2"}, Time: hm("08:00"), Calendars: []string{holiday}, MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.holiday@A1>A2"}},
		},
		{
			// 出発駅をまとめて指定すると、乗り換え時間なしでどちらの駅からも乗れる
			name:  "出発駅をまとめて指定",
			query: Query{From: []string{"A2", "B2"}, To: []string{"B3"}, Time: hm("10:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"B.1014@B2>B3"}},
		},
		{
			name:  "到着駅をまとめて指定",
			query: Query{From: []string{"A1"}, To: []string{"A2", "B2"}, Time: hm("09:58"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1000@A1>A2"}},
		},
		{
			// A が6分遅れると、どちらの各停でも B.1020 に間に合わない
			name:  "遅れで乗り換えられなくなる",
			query: Query{From: []string{"A1"}, To: []string{"B3"}, Time: hm("09:58"), Delays: []Delay{{Railway: "A", Minutes: 6}}, DelayUntil: hm("12:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"A.local1005@A1>A2", "B.1040@B2>B3"}},
		},
		{
			name:  "遅れで発車済みの列車に乗れる",
			query: Query{From: []string{"B2"}, To: []string{"B3"}, Time: hm("10:25"), Delays: []Delay{{Railway: "B", Minutes: 10}}, DelayUntil: hm("12:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"B.1020@B2>B3"}},
		},
		{
			name:  "別の方向の遅れは足さない",
			query: Query{From: []string{"B2"}, To: []string{"B3"}, Time: hm("10:25"), Delays: []Delay{{Railway: "B", RailDirection: "other", Minutes: 10}}, DelayUntil: hm("12:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"B.1040@B2>B3"}},
		},
		{
			// 10:20 発は 10:15（DelayUntil + 遅れ）より後なので、時刻表どおり
			name:  "遅れは DelayUntil より後の列車に足さない",
			query: Query{From: []string{"B2"}, To: []string{"B3"}, Time: hm("10:25"), Delays: []Delay{{Railway: "B", Minutes: 10}}, DelayUntil: hm("10:05"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"B.1040@B2>B3"}},
		},
		{
			name:  "同じ駅を2回通る列車で、2回目に降りる",
			query: Query{From: []string{"a"}, To: []string{"T"}, Time: hm("10:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"L.loop@a>T"}},
		},
		{
			name:  "同じ駅を2回通る列車で、1回目に乗る",
			query: Query{From: []string{"T"}, To: []string{"b"}, Time: hm("10:00"), MaxTransfers: DefaultMaxTransfers},
			want:  [][]string{{"L.loop@T>b"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			q := tt.query
			if q.Calendars == nil {
				q.Calendars = []string{weekday}
			}

			got, err := e.Search(q)
			if err != nil {
				t.Fatal(err)
			}

			if s := summary(got); !reflect.DeepEqual(s, tt.want) {
				t.Errorf("got %v, want %v", s, tt.want)
			}
		})
	}
}

func TestSearchTimes(t *testing.T) {

	e := newTestEngine(t)

	got, err := e.Search(Query{
		From:         []string{"A1"},
		To:           []string{"B3"},
		Calendars:    []string{weekday},
		Time:         hm("09:58"),
		MaxTransfers: DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []Journey{
		{
			Departure: hm("10:05"),
			Arrival:   hm("10:30"),
			Legs: []Leg{
				{
					Railway:     "A",
					Train:       "A.local1005",
					TrainNumber: "A.local1005",
					TrainType:   "Local",
					Destination: "A3",
					From:        "A1",
					To:          "A2",
					Departure:   hm("10:05"),
					Arrival:     hm("10:15"),
				},
				{
					Railway:     "B",
					Train:       "B.1020",
					TrainNumber: "B.1020",
					TrainType:   "Local",
					Destination: "B3",
					From:        "B2",
					To:          "B3",
					Departure:   hm("10:20"),
					Arrival:     hm("10:30"),
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if got[0].Transfers() != 1 {
		t.Errorf("transfers = %d, want 1", got[0].Transfers())
	}
}

// 乗り換え時間が足りない列車（B.1014。A2 着 10:10 + 5分 > 10:14）には乗らない
func TestSearchTransferTime(t *testing.T) {

	e := New(buildTimetables(t, testTrains), []Transfer{
		{From: "A2", To: "B2", Minutes: 3},
	}, DefaultConfig())

	got, err := e.Search(Query{
		From:         []string{"A1"},
		To:           []string{"B3"},
		Calendars:    []string{weekday},
		Time:         hm("09:58"),
		MaxTransfers: DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"A.local1000@A1>A2", "B.1014@B2>B3"}}
	if s := summary(got); !reflect.DeepEqual(s, want) {
		t.Errorf("got %v, want %v", s, want)
	}
}

// 同じ駅での乗り継ぎ時間が足りなければ、急行に乗り継がない
func TestSearchSameStationMinutes(t *testing.T) {

	cfg := DefaultConfig()
	cfg.SameStationMinutes = 3

	e := New(buildTimetables(t, testTrains), testTransfers, cfg)

	got, err := e.Search(Query{
		From:         []string{"A1"},
		To:           []string{"A3"},
		Calendars:    []string{weekday},
		Time:         hm("09:58"),
		MaxTransfers: DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}

	want := [][]string{{"A.local1000@A1>A3"}}
	if s := summary(got); !reflect.DeepEqual(s, want) {
		t.Errorf("got %v, want %v", s, want)
	}
}

func TestSearchErrors(t *testing.T) {

	e := newTestEngine(t)

	tests := []struct {
		name     string
		from, to string
		want     error
	}{
		{"存在しない出発駅", "X", "A1", ErrUnknownStation},
		{"存在しない到着駅", "A1", "X", ErrUnknownStation},
		{"同じ駅", "A1", "A1", ErrSameStation},
		{"駅の指定なし", "", "A1", ErrUnknownStation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var from []string
			if tt.from != "" {
				from = []string{tt.from}
			}

			_, err := e.Search(Query{From: from, To: []string{tt.to}, Calendars: []string{weekday}})
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSearchDelayTimes(t *testing.T) {

	e := newTestEngine(t)

	// A は 10:02 までの時刻が5分遅れる。それより後の時刻は 10:07 より前にならない
	base := Query{
		From:         []string{"A1"},
		To:           []string{"A2"},
		Calendars:    []string{weekday},
		MaxTransfers: DefaultMaxTransfers,
		Delays:       []Delay{{Railway: "A", Minutes: 5}},
		DelayUntil:   hm("10:02"),
	}

	departAt := base
	departAt.Time = hm("09:58")

	arriveBy := base
	arriveBy.Time = hm("10:10")
	arriveBy.ArriveBy = true

	for name, q := range map[string]Query{"出発時刻": departAt, "到着時刻": arriveBy} {
		t.Run(name, func(t *testing.T) {

			got, err := e.Search(q)
			if err != nil {
				t.Fatal(err)
			}

			if len(got) != 1 || len(got[0].Legs) != 1 {
				t.Fatalf("got %v", summary(got))
			}

			l := got[0].Legs[0]
			if l.Train != "A.local1000" || l.Departure != hm("10:05") || l.Arrival != hm("10:10") || l.Delay != 5 {
				t.Errorf("got %s %d-%d delay %d", l.Train, l.Departure, l.Arrival, l.Delay)
			}
			if got[0].Departure != l.Departure || got[0].Arrival != l.Arrival {
				t.Errorf("journey %d-%d does not match leg", got[0].Departure, got[0].Arrival)
			}
		})
	}
}
