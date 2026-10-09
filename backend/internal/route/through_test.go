package route

import (
	"reflect"
	"testing"
)

// 路線 X: X1 - X2、路線 Y: Y2 - Y3（X2 と Y2 は境目の駅。乗り換えは5分）。
// X.1 は X2 で Y.1 に直通する（行先は Y3、間は0分）。Y.2 は直通しない
var throughTrains = []testTrain{
	{"X.1", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
	{"Y.1", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
	{"Y.2", "Y", weekday, []string{"Y2 - 10:30", "Y3 10:40 -"}},
}

var throughTransfers = []Transfer{
	{From: "X2", To: "Y2", Minutes: 5},
	{From: "Y2", To: "X2", Minutes: 5},
}

func newThroughEngine(t *testing.T, trains []testTrain) *Engine {
	t.Helper()

	tt := buildTimetables(t, trains)

	// X の列車の行先は、直通先の Y3
	for i, train := range tt.Trains {
		if tt.String(train.Railway) == "X" {
			for j, s := range tt.Strings {
				if s == "Y3" {
					tt.Trains[i].Destination = int32(j)
				}
			}
		}
	}

	return New(tt, throughTransfers, DefaultConfig())
}

func TestThroughChains(t *testing.T) {

	e := newThroughEngine(t, throughTrains)
	if len(e.chains) != 1 || len(e.chains[0]) != 2 {
		t.Fatalf("chains = %v", e.chains)
	}

	// 同じ間で直通先の候補が2本あれば、つながない
	e = newThroughEngine(t, append(throughTrains,
		testTrain{"Y.3", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:25 -"}},
	))
	if len(e.chains) != 0 {
		t.Fatalf("ambiguous trains must not be joined: %v", e.chains)
	}

	// 間が MaxThroughGapMinutes を超えたら、つながない
	e = newThroughEngine(t, []testTrain{throughTrains[0], throughTrains[2]})
	if len(e.chains) != 0 {
		t.Fatalf("trains with a long gap must not be joined: %v", e.chains)
	}
}

func TestSearchThrough(t *testing.T) {

	e := newThroughEngine(t, throughTrains)

	query := func(from, to string) Query {
		return Query{
			From: []string{from}, To: []string{to}, Calendars: []string{weekday},
			Time: hm("09:55"), MaxTransfers: DefaultMaxTransfers,
		}
	}

	// 境目で乗り換えず（乗り換えの5分を足さず）、直通の列車に乗り続ける
	got, err := e.Search(query("X1", "Y3"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Journey{{
		Departure: hm("10:00"),
		Arrival:   hm("10:20"),
		Legs: []Leg{
			{Railway: "X", Train: "X.1", TrainNumber: "X.1", TrainType: "Local", Destination: "Y3",
				From: "X1", To: "X2", Departure: hm("10:00"), Arrival: hm("10:10")},
			{Railway: "Y", Train: "Y.1", TrainNumber: "Y.1", TrainType: "Local", Destination: "Y3",
				From: "Y2", To: "Y3", Departure: hm("10:10"), Arrival: hm("10:20"), Through: true},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if got[0].Transfers() != 0 {
		t.Errorf("transfers = %d, want 0", got[0].Transfers())
	}

	// 到着時刻を指定しても同じ経路になる
	q := query("X1", "Y3")
	q.ArriveBy, q.Time = true, hm("10:25")
	got, err = e.Search(q)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("arriveBy: got %+v, want %+v", got, want)
	}

	// 直通の列車の一部の区間だけにも乗れる
	for _, tt := range []struct{ from, to, train string }{
		{"X1", "X2", "X.1"},
		{"Y2", "Y3", "Y.1"},
	} {
		got, err := e.Search(query(tt.from, tt.to))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || len(got[0].Legs) != 1 || got[0].Legs[0].Train != tt.train || got[0].Legs[0].Through {
			t.Errorf("%s>%s: got %+v", tt.from, tt.to, got)
		}
	}

	// 直通先の路線を使わないなら、境目で降りて乗り換える経路も無い（Y は Y.1・Y.2 だけ）
	q = query("X1", "Y3")
	q.Avoid = []string{"Y"}
	if got, err := e.Search(q); err != nil || len(got) != 0 {
		t.Errorf("avoid Y: got %+v, %v", got, err)
	}
}

// 前の区間の遅れは、直通先の区間にも持ち越す
func TestSearchThroughDelay(t *testing.T) {

	e := newThroughEngine(t, throughTrains)

	got, err := e.Search(Query{
		From: []string{"X1"}, To: []string{"Y3"}, Calendars: []string{weekday},
		Time: hm("09:55"), MaxTransfers: DefaultMaxTransfers,
		Delays:     []Delay{{Railway: "X", Minutes: 5}},
		DelayUntil: hm("11:00"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || len(got[0].Legs) != 2 {
		t.Fatalf("got %+v", got)
	}
	if y := got[0].Legs[1]; !y.Through || y.Departure != hm("10:15") || y.Arrival != hm("10:25") || y.Delay != 5 {
		t.Errorf("delay is not carried over: %+v", y)
	}
}
