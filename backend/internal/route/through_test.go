package route

import (
	"fmt"
	"reflect"
	"testing"
)

// 路線 X: X1 - X2、路線 Y: Y2 - Y3 - Y4、路線 Z: Z3 - Z5
// （X2 と Y2、Y3 と Z3 は境目の駅。乗り換えは5分）。
// X.100 は X2 で Y.200 に直通する（行先は Y3、間は0分）。Y.400 は直通しない
var throughTrains = []testTrain{
	{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
	{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
	{"Y.400", "Y", weekday, []string{"Y2 - 10:30", "Y3 10:40 -"}},
}

var throughTransfers = []Transfer{
	{From: "X2", To: "Y2", Minutes: 5},
	{From: "Y2", To: "X2", Minutes: 5},
	{From: "Y3", To: "Z3", Minutes: 5},
	{From: "Z3", To: "Y3", Minutes: 5},
}

// newThroughEngine は、列車の行先を destinations（列車ID → 行先の駅）で置き換えて探索エンジンを作る。
// 行先を書かない列車は、終点が行先になる
func newThroughEngine(t *testing.T, trains []testTrain, destinations map[string]string) *Engine {
	t.Helper()

	tt := buildTimetables(t, trains)

	index := make(map[string]int32)
	for i, s := range tt.Strings {
		index[s] = int32(i)
	}

	for i, train := range tt.Trains {
		if dest, ok := destinations[tt.String(train.Train)]; ok {
			tt.Trains[i].Destination = index[dest]
		}
	}

	return New(tt, throughTransfers, DefaultConfig())
}

// chainIDs は、つないだ列車の並びを列車ID で返す。
func chainIDs(e *Engine) [][]string {
	var result [][]string
	for _, chain := range e.chains {
		var ids []string
		for _, ti := range chain {
			ids = append(ids, e.tt.String(e.tt.Trains[ti].Train))
		}
		result = append(result, ids)
	}
	return result
}

func TestThroughChains(t *testing.T) {

	tests := []struct {
		name         string
		trains       []testTrain
		destinations map[string]string
		want         [][]string
	}{
		{
			name:         "直通",
			trains:       throughTrains,
			destinations: map[string]string{"X.100": "Y3"},
			want:         [][]string{{"X.100", "Y.200"}},
		},
		{
			name: "同じ順位・同じ間の候補が2本ならつながない",
			trains: append(throughTrains[:2:2],
				testTrain{"Y.300", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:25 -"}},
			),
			destinations: map[string]string{"X.100": "Y3"},
		},
		{
			name: "間が5分を超えたらつながない",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:16", "Y3 10:30 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
		},
		{
			name: "列車番号の数字が同じなら10分までつなぐ",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.100", "Y", weekday, []string{"Y2 - 10:18", "Y3 10:30 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
			want:         [][]string{{"X.100", "Y.100"}},
		},
		{
			// 行先を通るだけの列車（行先より先まで走る）は、間が短くてもつながない
			name: "行先が同じ列車だけをつなぐ",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.900", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 10:20", "Y4 10:30 -"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:12", "Y3 10:22 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
			want:         [][]string{{"X.100", "Y.200"}},
		},
		{
			// 事業者をまたぐと番号がたまたま一致することがある（京王新線 1804 → 新宿始発の都営 1804T）
			name: "事業者をまたぐ組は、列車番号の数字が同じでも5分まで",
			trains: []testTrain{
				{"odpt.Train:A.X.100", "odpt.Railway:A.X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"odpt.Train:B.Y.100", "odpt.Railway:B.Y", weekday, []string{"Y2 - 10:18", "Y3 10:30 -"}},
				{"odpt.Train:B.Y.200", "odpt.Railway:B.Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
			},
			destinations: map[string]string{"odpt.Train:A.X.100": "Y3", "odpt.Train:B.Y.100": "Y3"},
			want:         [][]string{{"odpt.Train:A.X.100", "odpt.Train:B.Y.200"}},
		},
		{
			name: "列車番号の数字が同じ列車を、間の短さより優先する",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.900", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
				{"Y.100", "Y", weekday, []string{"Y2 - 10:13", "Y3 10:23 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
			want:         [][]string{{"X.100", "Y.100"}},
		},
		{
			name: "1本の列車に2本がつながるなら、良い方だけを残す",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:08 -"}},
				{"X.300", "X", weekday, []string{"X1 - 10:02", "X2 10:10 -"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
			},
			destinations: map[string]string{"X.100": "Y3", "X.300": "Y3"},
			want:         [][]string{{"X.300", "Y.200"}},
		},
		{
			name: "1本の列車に同じ順位・同じ間の2本がつながるなら、どれもつながない",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"X.300", "X", weekday, []string{"X0 - 10:02", "X2 10:10 -"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
			},
			destinations: map[string]string{"X.100": "Y3", "X.300": "Y3"},
		},
		{
			name: "3本の並び",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
				{"Z.300", "Z", weekday, []string{"Z3 - 10:22", "Z5 10:30 -"}},
			},
			destinations: map[string]string{"X.100": "Z5", "Y.200": "Z5"},
			want:         [][]string{{"X.100", "Y.200", "Z.300"}},
		},
		{
			// 都内に絞った切れ目の駅は境目ではない
			name: "終点に到着時刻が無い列車はつながない",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 - 10:10"}},
				{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
		},
		{
			name: "ダイヤ種別が違えばつながない",
			trains: []testTrain{
				{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
				{"Y.200", "Y", holiday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
			},
			destinations: map[string]string{"X.100": "Y3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newThroughEngine(t, tt.trains, tt.destinations)
			if got := chainIDs(e); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("chains = %v, want %v", got, tt.want)
			}
		})
	}
}

// 境目の駅で、直通先として少ない路線（最も多い路線の1割未満）は、行先が同じでも外してつなぎ直す
// （都営新宿線の新宿で、京王新線ではなく、すぐに出る京王線の列車を選んでしまう誤りを防ぐ）
func TestThroughChainsMinorRailway(t *testing.T) {

	transfers := append(throughTransfers,
		Transfer{From: "X2", To: "W2", Minutes: 5},
		Transfer{From: "W2", To: "X2", Minutes: 5},
	)

	var trains []testTrain
	want := [][]string{}

	// X の列車 12 本は、2分後に Y の列車へ直通する
	for i := range 12 {
		x, y := fmt.Sprintf("X.%d", 100+i), fmt.Sprintf("Y.%d", 200+i)
		h := 10 + i
		trains = append(trains,
			testTrain{x, "X", weekday, []string{fmt.Sprintf("X1 - %d:00", h), fmt.Sprintf("X2 %d:10 -", h)}},
			testTrain{y, "Y", weekday, []string{fmt.Sprintf("Y2 - %d:12", h), fmt.Sprintf("Y3 %d:20 -", h)}},
		)
		want = append(want, []string{x, y})
	}

	// 最後の X の列車には、隣の駅 W2 を0分で出る、行先が同じ W の列車もある
	trains = append(trains, testTrain{"W.900", "W", weekday, []string{"W2 - 21:10", "Y3 21:30 -"}})

	tt := buildTimetables(t, trains)
	index := make(map[string]int32)
	for i, s := range tt.Strings {
		index[s] = int32(i)
	}
	for i, train := range tt.Trains {
		if tt.String(train.Railway) == "X" {
			tt.Trains[i].Destination = index["Y3"]
		}
	}

	e := New(tt, transfers, DefaultConfig())

	if got := chainIDs(e); !reflect.DeepEqual(got, want) {
		t.Errorf("chains = %v, want %v", got, want)
	}
}

func throughQuery(from, to string) Query {
	return Query{
		From: []string{from}, To: []string{to}, Calendars: []string{weekday},
		Time: hm("09:55"), MaxTransfers: DefaultMaxTransfers,
	}
}

func TestSearchThrough(t *testing.T) {

	e := newThroughEngine(t, throughTrains, map[string]string{"X.100": "Y3"})

	// 境目で乗り換えず（乗り換えの5分を足さず）、直通の列車に乗り続ける
	got, err := e.Search(throughQuery("X1", "Y3"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Journey{{
		Departure: hm("10:00"),
		Arrival:   hm("10:20"),
		Legs: []Leg{
			{Railway: "X", Train: "X.100", TrainNumber: "X.100", TrainType: "Local", Destination: "Y3",
				From: "X1", To: "X2", Departure: hm("10:00"), Arrival: hm("10:10")},
			{Railway: "Y", Train: "Y.200", TrainNumber: "Y.200", TrainType: "Local", Destination: "Y3",
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
	q := throughQuery("X1", "Y3")
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
		{"X1", "X2", "X.100"},
		{"Y2", "Y3", "Y.200"},
	} {
		got, err := e.Search(throughQuery(tt.from, tt.to))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || len(got[0].Legs) != 1 || got[0].Legs[0].Train != tt.train || got[0].Legs[0].Through {
			t.Errorf("%s>%s: got %+v", tt.from, tt.to, got)
		}
	}

	// 前の区間の終点（X2）では乗れないので、Y2 まで歩く（5分）。10:06 に出ると 10:10 の Y.200 には間に合わない
	got, err = e.Search(Query{
		From: []string{"X2"}, To: []string{"Y3"}, Calendars: []string{weekday},
		Time: hm("10:06"), MaxTransfers: DefaultMaxTransfers,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Legs[0].Train != "Y.400" || got[0].Legs[0].From != "Y2" {
		t.Errorf("X2>Y3: got %+v", got)
	}

	// 直通先の路線を使わないなら、Y の区間には乗らない（Y は Y.200・Y.400 だけ）
	q = throughQuery("X1", "Y3")
	q.Avoid = []string{"Y"}
	if got, err := e.Search(q); err != nil || len(got) != 0 {
		t.Errorf("avoid Y: got %+v, %v", got, err)
	}

	// 前の区間の路線を使わなくても、次の区間の始発駅からは乗れる
	q = throughQuery("Y2", "Y3")
	q.Avoid = []string{"X"}
	if got, err := e.Search(q); err != nil || len(got) != 1 || got[0].Legs[0].Train != "Y.200" {
		t.Errorf("avoid X: got %+v, %v", got, err)
	}
}

// 3本の並びは、元の列車ごとの3つの区間に分ける
func TestSearchThroughChainOfThree(t *testing.T) {

	e := newThroughEngine(t, []testTrain{
		{"X.100", "X", weekday, []string{"X1 - 10:00", "X2 10:10 -"}},
		{"Y.200", "Y", weekday, []string{"Y2 - 10:10", "Y3 10:20 -"}},
		{"Z.300", "Z", weekday, []string{"Z3 - 10:22", "Z5 10:30 -"}},
	}, map[string]string{"X.100": "Z5", "Y.200": "Z5"})

	got, err := e.Search(throughQuery("X1", "Z5"))
	if err != nil {
		t.Fatal(err)
	}

	if s := summary(got); !reflect.DeepEqual(s, [][]string{{"X.100@X1>X2", "Y.200@Y2>Y3", "Z.300@Z3>Z5"}}) {
		t.Fatalf("got %v", s)
	}
	if got[0].Transfers() != 0 || got[0].Legs[0].Through || !got[0].Legs[1].Through || !got[0].Legs[2].Through {
		t.Errorf("unexpected through flags: %+v", got[0])
	}
}

// 遅れは、それまでの区間の遅れの最大を足す
func TestSearchThroughDelay(t *testing.T) {

	e := newThroughEngine(t, throughTrains, map[string]string{"X.100": "Y3"})

	tests := []struct {
		name         string
		delays       []Delay
		xDep, yDep   string
		xDelay, yDel int
	}{
		{"前の区間の遅れを持ち越す", []Delay{{Railway: "X", Minutes: 5}}, "10:05", "10:15", 5, 5},
		{"次の区間の方が遅れが大きい", []Delay{{Railway: "X", Minutes: 2}, {Railway: "Y", Minutes: 7}}, "10:02", "10:17", 2, 7},
		{"次の区間だけ遅れている", []Delay{{Railway: "Y", Minutes: 3}}, "10:00", "10:13", 0, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := throughQuery("X1", "Y3")
			q.Delays, q.DelayUntil = tt.delays, hm("11:00")

			got, err := e.Search(q)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || len(got[0].Legs) != 2 {
				t.Fatalf("got %+v", got)
			}
			x, y := got[0].Legs[0], got[0].Legs[1]
			if x.Departure != hm(tt.xDep) || x.Delay != tt.xDelay || !y.Through || y.Departure != hm(tt.yDep) || y.Delay != tt.yDel {
				t.Errorf("x %+v, y %+v", x, y)
			}
		})
	}
}
