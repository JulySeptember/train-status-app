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
