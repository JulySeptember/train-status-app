package infer

import (
	"reflect"
	"testing"
)

// trainsOf は、推定した列車を「駅@分」の並びと終点にする。
func trainsOf(trains []Train) [][]string {
	var result [][]string
	for _, t := range trains {
		var stops []string
		for _, s := range t.Stops {
			stops = append(stops, s.Station+"@"+itoa(s.Minutes))
		}
		if t.Terminal != "" {
			stops = append(stops, t.Terminal+"="+itoa(t.TerminalMinutes))
		}
		result = append(result, stops)
	}
	return result
}

func itoa(n int) string {
	return string(rune('0'+n/100%10)) + string(rune('0'+n/10%10)) + string(rune('0'+n%10))
}

func dep(station string, minutes int, trainType, dest string) Departure {
	return Departure{Station: station, Minutes: minutes, TrainType: trainType, Destination: dest}
}

// 駅 A - B - C - D（各駅2分）。各停は全駅に、急行は A・D だけに停まる。
// 急行は B で各停を追い越すが、列車種別ごとにつなぐので入れ替わらない
func TestInfer(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "D"), dep("A", 103, "Express", "D"), dep("A", 110, "Local", "D")},
			"B": {dep("B", 102, "Local", "D"), dep("B", 112, "Local", "D")},
			"C": {dep("C", 104, "Local", "D"), dep("C", 114, "Local", "D")},
		},
	}

	want := [][]string{
		{"A@100", "B@102", "C@104", "D=106"},
		// 急行の終点は、各駅の所要時間の和（6分）で見込む（ほかの停車駅が無いので速さで縮められない）
		{"A@103", "D=109"},
		{"A@110", "B@112", "C@114", "D=116"},
	}

	if got := trainsOf(Infer(l)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 始発の印（odpt:isOrigin）のある発車は、前の駅の列車につながない
func TestInferOrigin(t *testing.T) {

	b := dep("B", 104, "Local", "C")
	b.Origin = true

	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "B")},
			"B": {b},
		},
	}

	want := [][]string{
		{"A@100", "B=102"},
		{"B@104", "C=106"},
	}

	if got := trainsOf(Infer(l)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 行先が路線に無い（直通運転で他社へ行く）列車は、最後の発車で終わり、終点の見込みは付けない
func TestInferThroughDestination(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "Other.X")},
			"B": {dep("B", 102, "Local", "Other.X")},
		},
	}

	want := [][]string{{"A@100", "B@102"}}

	if got := trainsOf(Infer(l)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 所要時間の見込みの2倍＋10分より遅い発車はつながない
func TestInferGapLimit(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "X"), dep("A", 200, "Local", "X"), dep("A", 300, "Local", "X")},
			// 所要時間の見込みは2分（中央値）。3本目は 2*2+10 = 14分を超える15分
			"B": {dep("B", 102, "Local", "X"), dep("B", 202, "Local", "X"), dep("B", 315, "Local", "X")},
		},
	}

	want := [][]string{{"A@100", "B@102"}, {"A@200", "B@202"}, {"A@300"}, {"B@315"}}
	if got := trainsOf(Infer(l)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 終点の見込みは、その列車が走った区間の速さで縮める（各駅の所要時間の和より速い急行）
func TestInferTerminalSpeed(t *testing.T) {

	// 各停は A-B-C-D-E を各駅2分。急行は A・C に停まり、A → C を2分（各駅の和の半分）で走る
	l := Line{
		Stations: []string{"A", "B", "C", "D", "E"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "E"), dep("A", 105, "Express", "E")},
			"B": {dep("B", 102, "Local", "E")},
			"C": {dep("C", 104, "Local", "E"), dep("C", 107, "Express", "E")},
			"D": {dep("D", 106, "Local", "E")},
		},
	}

	want := [][]string{
		{"A@100", "B@102", "C@104", "D@106", "E=108"},
		// C → E の見込み4分を、速さ（2分 ÷ 4分）で縮めて2分
		{"A@105", "C@107", "E=109"},
	}
	if got := trainsOf(Infer(l)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 駅の並びが進む向きと逆なら、逆にする
func TestOrient(t *testing.T) {

	l := Line{
		Stations: []string{"C", "B", "A"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "C")},
			"B": {dep("B", 102, "Local", "C")},
		},
	}

	if got := Orient(l).Stations; !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
		t.Errorf("got %v", got)
	}
	if got := Orient(Orient(l)).Stations; !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
		t.Errorf("orienting twice: got %v", got)
	}
}

func TestUnplaced(t *testing.T) {
	l := Line{
		Stations:   []string{"A"},
		Departures: map[string][]Departure{"A": {dep("A", 100, "L", "A")}, "Z": {dep("Z", 1, "L", "A"), dep("Z", 2, "L", "A")}},
	}
	if got := Unplaced(l); got != 2 {
		t.Errorf("got %d", got)
	}
}

// 直通先の列車が境目の駅（か乗り換えでつながる駅）から出ていれば、その駅まで延ばす
func TestExtendToPartners(t *testing.T) {

	// A - B - C（C が境目。直通する列車は C で発車しない）
	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "Other.Z"), dep("A", 110, "Local", "Other.Y")},
			"B": {dep("B", 102, "Local", "Other.Z"), dep("B", 112, "Local", "Other.Y")},
		},
	}

	partners := func(station string) []Partner {
		if station != "C" {
			return nil
		}
		// C とつながる駅 Other.C から、Z 行きが 10:45（1本目の続き）と 11:30（遠すぎる）に出る。Y 行きは無い
		return []Partner{
			{Station: "Other.C", Minutes: 105, Destination: "Other.Z"},
			{Station: "Other.C", Minutes: 130, Destination: "Other.Z"},
		}
	}

	got := trainsOf(ExtendToPartners(l, Infer(l), partners))
	want := [][]string{
		{"A@100", "B@102", "C=105"},
		// 直通先が見つからない列車は延ばさない
		{"A@110", "B@112"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
