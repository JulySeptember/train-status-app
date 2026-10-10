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

	// A - B - C（C が境目。直通する列車は C で発車しない）。各駅2分
	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "Other.Z"), dep("A", 110, "Local", "Other.Z"), dep("A", 120, "Local", "Other.Y")},
			"B": {dep("B", 102, "Local", "Other.Z"), dep("B", 112, "Local", "Other.Z"), dep("B", 122, "Local", "Other.Y")},
		},
	}

	partners := func(station string) []Partner {
		switch station {
		case "B":
			// B から、同じ路線の推定列車（途中で切れたもの）の始発が出る。直通先にはしない
			return []Partner{{Station: "B", Minutes: 113, Destination: "Other.Z", Railway: "Self"}}
		case "C":
			// C とつながる駅 Other.C から、Z 行きが 1:44（1本目の続き）・1:54（2本目の続き）・2:30（遠すぎる）に出る。
			// 1:44 は2本目には早すぎる（所要時間が見込みの半分未満）。Y 行きは無い
			return []Partner{
				{Station: "Other.C", Minutes: 104, Destination: "Other.Z"},
				{Station: "Other.C", Minutes: 114, Destination: "Other.Z"},
				{Station: "Other.C", Minutes: 150, Destination: "Other.Z"},
			}
		}
		return nil
	}

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, map[Partner]bool{}))
	want := [][]string{
		{"A@100", "B@102", "C=104"},
		{"A@110", "B@112", "C=114"},
		// 直通先が見つからない列車は延ばさない
		{"A@120", "B@122"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 1つの直通先には1本だけをつなぐ（見込みに近い方）
func TestExtendToPartnersOnePerPartner(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z"), dep("A", 101, "Local", "Other.Z")},
			"B": {dep("B", 102, "Express", "Other.Z"), dep("B", 103, "Local", "Other.Z")},
		},
	}
	partners := func(station string) []Partner {
		if station == "C" {
			return []Partner{{Station: "Other.C", Minutes: 105, Destination: "Other.Z"}}
		}
		return nil
	}

	extended := 0
	for _, tr := range ExtendToPartners(l, "Self", Infer(l), partners, map[Partner]bool{}) {
		if tr.Terminal != "" {
			extended++
		}
	}
	if extended != 1 {
		t.Errorf("extended %d trains to one partner", extended)
	}
}

// 最後の発車の駅が境目なら、その駅を終点にする（到着はその駅の発車の分）
func TestExtendToPartnersAtLastStop(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "Other.Z")},
			"B": {dep("B", 102, "Local", "Other.Z")},
		},
	}
	partners := func(station string) []Partner {
		if station == "B" {
			return []Partner{{Station: "Other.B", Minutes: 102, Destination: "Other.Z"}}
		}
		return nil
	}

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, map[Partner]bool{}))
	if want := [][]string{{"A@100", "B@102", "B=102"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 境目の手前で追い越しが無いので、境目に着く見込みの早い列車から順に、早い直通先を割り当てる。
// 境目の所要時間が分からない（境目に自社の発車が無いので既定の2分になる）とき、見込みとの差で決めると、
// 各停が直前の急行の直通先を取ってしまう（三軒茶屋 → 池尻大橋 → 渋谷）
func TestExtendToPartnersKeepsOrder(t *testing.T) {

	// A - B - C（C が境目）。急行は A に 9:14 に停まり B を通過、各停は A 9:16・B 9:18
	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 554, "Express", "Other.Z"), dep("A", 556, "Local", "Other.Z")},
			"B": {dep("B", 558, "Local", "Other.Z")},
		},
	}
	partners := func(station string) []Partner {
		if station == "C" {
			// 急行の続き 9:19、各停の続き 9:22
			return []Partner{
				{Station: "Other.C", Minutes: 559, Destination: "Other.Z"},
				{Station: "Other.C", Minutes: 562, Destination: "Other.Z"},
			}
		}
		return nil
	}

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, map[Partner]bool{}))
	want := [][]string{
		{"A@554", "C=559"},
		{"A@556", "B@558", "C=562"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 最後の発車の駅が境目で、直通先がその駅の発車の少し後なら、到着はその駅の発車の分にする（到着が発車より後にならない）
func TestExtendToPartnersAtLastStopWithGap(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Local", "Other.Z")},
			"B": {dep("B", 102, "Local", "Other.Z")},
		},
	}
	partners := func(station string) []Partner {
		if station == "B" {
			return []Partner{{Station: "Other.B", Minutes: 105, Destination: "Other.Z"}}
		}
		return nil
	}

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, map[Partner]bool{}))
	if want := [][]string{{"A@100", "B@102", "B=102"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
