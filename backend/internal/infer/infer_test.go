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

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, nil, map[Partner]bool{}))
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
	for _, tr := range ExtendToPartners(l, "Self", Infer(l), partners, nil, map[Partner]bool{}) {
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

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, nil, map[Partner]bool{}))
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

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, nil, map[Partner]bool{}))
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

	got := trainsOf(ExtendToPartners(l, "Self", Infer(l), partners, nil, map[Partner]bool{}))
	if want := [][]string{{"A@100", "B@102", "B=102"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// branchLines は、本線 S - K - Y と、K から分かれる空港線 K' - H3 - H（両方向）。
// 空港行きのエアポート快特は、本線の S を出ると K を通過して空港線に入り、H3 で初めて空港線の駅時刻表に載る。
// stopsAtK なら、エアポート快特が K に停まる時間帯もある
func branchLines(stopsAtK bool) map[LineKey]Line {
	mainK := []Departure{dep("Main.K", 92, "Local", "Main.Y")}
	if stopsAtK {
		mainK = append(mainK, dep("Main.K", 60, "AirportRapid", "Air.H"))
	}
	return map[LineKey]Line{
		{Railway: "Main", RailDirection: "Out"}: {
			Stations: []string{"Main.S", "Main.K", "Main.Y"},
			Departures: map[string][]Departure{
				"Main.S": {dep("Main.S", 90, "Local", "Main.Y"), dep("Main.S", 100, "AirportRapid", "Air.H")},
				"Main.K": mainK,
			},
		},
		{Railway: "Air", RailDirection: "Out"}: {
			Stations: []string{"Air.K", "Air.H3", "Air.H"},
			Departures: map[string][]Departure{
				"Air.K":  {dep("Air.K", 80, "Local", "Air.H")},
				"Air.H3": {dep("Air.H3", 84, "Local", "Air.H"), dep("Air.H3", 108, "AirportRapid", "Air.H")},
			},
		},
		// 逆向き（空港発）には H 行きが無いので、K から分かれる駅の並びにしない
		{Railway: "Air", RailDirection: "In"}: {
			Stations: []string{"Air.H", "Air.H3", "Air.K"},
			Departures: map[string][]Departure{
				"Air.H":  {dep("Air.H", 70, "Local", "Main.Y")},
				"Air.H3": {dep("Air.H3", 72, "Local", "Main.Y")},
			},
		},
	}
}

// 直通先が乗り換えでつながる別の路線の先の駅から出ていれば（京急蒲田を通過して空港線に入るエアポート快特）、その駅まで延ばす
func TestExtendToPartnersThroughBranch(t *testing.T) {

	lines := branchLines(false)
	connect := map[string][]string{"Main.K": {"Air.K"}, "Air.K": {"Main.K"}}
	main := LineKey{Railway: "Main", RailDirection: "Out"}

	if got := len(Branches(lines, connect)(main, "Main.K", "Air.H")); got != 1 {
		t.Fatalf("got %d branches, want 1 (toward Air.H only)", got)
	}

	partners := func(station string) []Partner {
		switch station {
		case "Main.K":
			// K から出る空港線の各停（別の列車）
			return []Partner{{Station: "Air.K", Minutes: 80, Destination: "Air.H", Railway: "Air", TrainType: "Local"}}
		case "Air.H3":
			return []Partner{
				{Station: "Air.H3", Minutes: 84, Destination: "Air.H", Railway: "Air", TrainType: "Local"},
				{Station: "Air.H3", Minutes: 108, Destination: "Air.H", Railway: "Air", TrainType: "AirportRapid"},
			}
		}
		return nil
	}

	branches := func(station, destination string) []Branch {
		return Branches(lines, connect)(main, station, destination)
	}
	got := trainsOf(ExtendToPartners(lines[main], "Main", Infer(lines[main]), partners, branches, map[Partner]bool{}))
	want := [][]string{
		{"Main.S@090", "Main.K@092", "Main.Y=094"},
		{"Main.S@100", "Air.H3=108"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// その列車種別・行先の列車が乗り換えの駅に停まることがあれば、別の路線の先の駅は探さない
// （停まる種別なら、その駅の発車が直通先のはず。先の駅から出る別の列車を取らないように）
func TestExtendToPartnersNoBranchWhenStopping(t *testing.T) {

	lines := branchLines(true)
	connect := map[string][]string{"Main.K": {"Air.K"}, "Air.K": {"Main.K"}}
	main := LineKey{Railway: "Main", RailDirection: "Out"}

	partners := func(station string) []Partner {
		if station == "Air.H3" {
			return []Partner{{Station: "Air.H3", Minutes: 108, Destination: "Air.H", Railway: "Air", TrainType: "AirportRapid"}}
		}
		return nil
	}
	branches := func(station, destination string) []Branch {
		return Branches(lines, connect)(main, station, destination)
	}

	for _, tr := range ExtendToPartners(lines[main], "Main", Infer(lines[main]), partners, branches, map[Partner]bool{}) {
		if tr.Stops[0].TrainType == "AirportRapid" && tr.Terminal != "" {
			t.Errorf("extended to %s", tr.Terminal)
		}
	}
}

// MaxExtendStations より先の直通先は、区間ごとの最短の所要時間の和の半分より速いものを使わない
func TestInFarRange(t *testing.T) {
	// 京急の快特: 品川 → 京急蒲田 7分（見込み 24分、最短の和 11分）
	if !inFarRange(7, 24, 11) {
		t.Error("rejected the rapid limited express")
	}
	// 西武秩父線の特急: 横瀬 → 飯能が約30分なのに、10分後に出る別の特急（見込み 40分、最短の和 36分）
	if inFarRange(10, 40, 36) {
		t.Error("accepted another limited express")
	}
}

// 途中で列車種別が変わる、直通先へ行く列車（西武新宿線の拝島行きの急行は、途中から各停）は、1本につなぐ
func TestInferTrainTypeChange(t *testing.T) {

	// A・B は急行、C・D は各停として載る。急行の発車は B より先に無い。行先は路線の外
	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z")},
			"B": {dep("B", 105, "Express", "Other.Z")},
			"C": {dep("C", 108, "Local", "Other.Z")},
			"D": {dep("D", 110, "Local", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	if want := [][]string{{"A@100", "B@105", "C@108", "D@110"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 行先が路線の駅なら、終点まで通過する特急に、途中の駅から始まる各停をつながない（西武池袋線の特急 所沢 → 池袋）
func TestInferNoTrainTypeChangeToLineDestination(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "LimitedExpress", "D")},
			"B": {dep("B", 103, "LimitedExpress", "D")},
			// 特急は C を通過する。C から各停が始まる（別の路線から入ってくる列車）
			"C": {dep("C", 106, "Local", "D")},
		},
	}

	got := trainsOf(Infer(l))
	if len(got) != 2 {
		t.Errorf("got %v", got)
	}
}

// 前の種別がその駅より先にも現れる（途中を通過する急行）なら、別の種別の発車にはつながない
func TestInferNoTrainTypeChangeWhenContinuing(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "D")},
			// 急行は B を通過して C に停まる。B から始まる各停がある
			"B": {dep("B", 103, "Local", "D")},
			"C": {dep("C", 106, "Express", "D"), dep("C", 107, "Local", "D")},
		},
	}

	// 急行は A → C、各停は B → C（B から始まる）。急行が B の各停を取り込まない
	got := trainsOf(Infer(l))
	want := [][]string{{"A@100", "C@106", "D=108"}, {"B@103", "C@107", "D=109"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 同じ種別・行先の別の列車が先まで走っていても、種別が変わる列車はつなぐ（その日の最後の駅で決めない）
func TestInferTrainTypeChangeWithLaterContinuing(t *testing.T) {

	// 100 の急行は C から各停になる。200・300 の急行は D まで急行のまま
	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z"), dep("A", 200, "Express", "Other.Z"), dep("A", 300, "Express", "Other.Z")},
			"B": {dep("B", 105, "Express", "Other.Z"), dep("B", 205, "Express", "Other.Z"), dep("B", 305, "Express", "Other.Z")},
			"C": {dep("C", 108, "Local", "Other.Z"), dep("C", 208, "Express", "Other.Z"), dep("C", 308, "Express", "Other.Z")},
			"D": {dep("D", 110, "Local", "Other.Z"), dep("D", 210, "Express", "Other.Z"), dep("D", 310, "Express", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	want := [][]string{
		{"A@100", "B@105", "C@108", "D@110"},
		{"A@200", "B@205", "C@208", "D@210"},
		{"A@300", "B@305", "C@308", "D@310"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 前の列車が同じ種別・行先のまま先の駅で発車するなら、行先が路線の外でも、途中から始まる列車をつながない
func TestInferNoTrainTypeChangeWhenContinuingThrough(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z")},
			// 急行は B を通過して C に停まる。B から各停が始まる
			"B": {dep("B", 103, "Local", "Other.Z")},
			"C": {dep("C", 106, "Express", "Other.Z"), dep("C", 107, "Local", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	want := [][]string{{"A@100", "C@106"}, {"B@103", "C@107"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 同じ駅で種別が変わる列車が2本あれば、時刻の順にそれぞれの続きへつなぐ。
// 見込みから大きく外れる発車（別の路線から入ってくる列車）は、どれにもつながない
func TestInferTrainTypeChangeTwoTrains(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z"), dep("A", 120, "Express", "Other.Z")},
			"B": {dep("B", 105, "Express", "Other.Z"), dep("B", 125, "Express", "Other.Z")},
			"C": {dep("C", 108, "Local", "Other.Z"), dep("C", 128, "Local", "Other.Z"), dep("C", 160, "Local", "Other.Z")},
			"D": {dep("D", 110, "Local", "Other.Z"), dep("D", 130, "Local", "Other.Z"), dep("D", 162, "Local", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	want := [][]string{
		{"A@100", "B@105", "C@108", "D@110"},
		{"A@120", "B@125", "C@128", "D@130"},
		{"C@160", "D@162"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 種別が変わる駅で、前の列車の見込みから大きく外れる発車しかなければつながない
func TestInferNoTrainTypeChangeFarFromExpected(t *testing.T) {

	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z")},
			"B": {dep("B", 105, "Express", "Other.Z")},
			"C": {dep("C", 140, "Local", "Other.Z")},
			"D": {dep("D", 142, "Local", "Other.Z")},
		},
	}

	if got := trainsOf(Infer(l)); len(got) != 2 {
		t.Errorf("got %v", got)
	}
}

// 同じ種別・行先の列車が短い間隔で走っていても、後ろの列車の発車を前の列車の続きと取り違えない
func TestInferTrainTypeChangeShortHeadway(t *testing.T) {

	// 10分おきの急行のうち、100 の急行だけが C から各停になる
	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 90, "Express", "Other.Z"), dep("A", 100, "Express", "Other.Z"), dep("A", 110, "Express", "Other.Z")},
			"B": {dep("B", 95, "Express", "Other.Z"), dep("B", 105, "Express", "Other.Z"), dep("B", 115, "Express", "Other.Z")},
			"C": {dep("C", 98, "Express", "Other.Z"), dep("C", 108, "Local", "Other.Z"), dep("C", 118, "Express", "Other.Z")},
			"D": {dep("D", 100, "Express", "Other.Z"), dep("D", 110, "Local", "Other.Z"), dep("D", 120, "Express", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	want := [][]string{
		{"A@090", "B@095", "C@098", "D@100"},
		{"A@100", "B@105", "C@108", "D@110"},
		{"A@110", "B@115", "C@118", "D@120"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// 急行が種別の変わる駅を通過する場合も、後ろの急行の先の駅の発車を、前の列車の続きと取り違えない
func TestInferTrainTypeChangePassingStation(t *testing.T) {

	// 急行は C を通過して D に停まる。100 の急行だけが C から各停になる（110 の急行は D まで急行）
	l := Line{
		Stations: []string{"A", "B", "C", "D"},
		Departures: map[string][]Departure{
			"A": {dep("A", 100, "Express", "Other.Z"), dep("A", 110, "Express", "Other.Z")},
			"B": {dep("B", 105, "Express", "Other.Z"), dep("B", 115, "Express", "Other.Z")},
			"C": {dep("C", 108, "Local", "Other.Z")},
			"D": {dep("D", 110, "Local", "Other.Z"), dep("D", 119, "Express", "Other.Z")},
		},
	}

	got := trainsOf(Infer(l))
	want := [][]string{
		{"A@100", "B@105", "C@108", "D@110"},
		{"A@110", "B@115", "D@119"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ほかの路線から途中の駅に入ってくる列車の最初の発車に始発の印を付け、前の列車の続きとしてつながないようにする
func TestMarkEntering(t *testing.T) {

	// 本線 A - B - J - C。本線の列車は J に2〜4分で着く（B → J の見込みは4分）。
	// 支線 F1 - F2 - J の列車は J から本線に入る（J 発 104）。印が無いと、100 の本線の列車が J 発 104 の
	// 入ってくる列車を続きとして取り、本当の続きの J 発 106 が別の列車になる
	main := LineKey{"Main", "Inbound", "Weekday"}
	branch := LineKey{"Branch", "Inbound", "Weekday"}
	lines := map[LineKey]Line{
		main: {
			Stations: []string{"Main.A", "Main.B", "Main.J", "Main.C"},
			Departures: map[string][]Departure{
				"Main.A": {dep("Main.A", 100, "Local", "Other.Z"), dep("Main.A", 120, "Local", "Other.Z")},
				"Main.B": {dep("Main.B", 102, "Local", "Other.Z"), dep("Main.B", 122, "Local", "Other.Z")},
				"Main.J": {dep("Main.J", 104, "Local", "Other.Z"), dep("Main.J", 106, "Local", "Other.Z"), dep("Main.J", 126, "Local", "Other.Z")},
				"Main.C": {dep("Main.C", 106, "Local", "Other.Z"), dep("Main.C", 108, "Local", "Other.Z"), dep("Main.C", 128, "Local", "Other.Z")},
			},
		},
		branch: {
			Stations: []string{"Branch.F1", "Branch.F2", "Branch.J"},
			Departures: map[string][]Departure{
				"Branch.F1": {dep("Branch.F1", 98, "Local", "Other.Z")},
				"Branch.F2": {dep("Branch.F2", 102, "Local", "Other.Z")},
			},
		},
	}
	connect := map[string][]string{"Branch.J": {"Main.J"}, "Main.J": {"Branch.J"}}

	inferred := map[LineKey][]Train{main: Infer(lines[main]), branch: Infer(lines[branch])}

	if got := trainsOf(inferred[main]); !reflect.DeepEqual(got[0], []string{"Main.A@100", "Main.B@102", "Main.J@104", "Main.C@106"}) {
		t.Fatalf("expected the wrong link without marks, got %v", got)
	}

	marked, n := MarkEntering(lines, inferred, connect)
	if n != 1 {
		t.Errorf("marked %d, want 1", n)
	}

	got := trainsOf(Infer(marked[main]))
	want := [][]string{
		{"Main.A@100", "Main.B@102", "Main.J@106", "Main.C@108"},
		{"Main.J@104", "Main.C@106"},
		{"Main.A@120", "Main.B@122", "Main.J@126", "Main.C@128"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// 入ってくる列車と種別が違う発車には付けない（同じ時刻・行先で別の種別の列車が出る駅がある。京王の調布）
	l := lines[branch]
	l.Departures = map[string][]Departure{
		"Branch.F1": {dep("Branch.F1", 98, "Express", "Other.Z")},
		"Branch.F2": {dep("Branch.F2", 102, "Express", "Other.Z")},
	}
	lines[branch] = l
	inferred[branch] = Infer(l)
	if _, n := MarkEntering(lines, inferred, connect); n != 0 {
		t.Errorf("marked %d departures of a different train type", n)
	}
}

// 入ってくる列車より、推定でつないだ前の列車の方がよく合う発車には印を付けない
func TestMarkEnteringKeepsBetterLink(t *testing.T) {

	main := LineKey{"Main", "Inbound", "Weekday"}
	branch := LineKey{"Branch", "Inbound", "Weekday"}
	lines := map[LineKey]Line{
		// 本線の列車は B → J を2分で走り、J 発 104 は本線の列車の続き
		main: {
			Stations: []string{"Main.A", "Main.B", "Main.J", "Main.C"},
			Departures: map[string][]Departure{
				"Main.A": {dep("Main.A", 100, "Local", "Other.Z")},
				"Main.B": {dep("Main.B", 102, "Local", "Other.Z")},
				"Main.J": {dep("Main.J", 104, "Local", "Other.Z")},
				"Main.C": {dep("Main.C", 106, "Local", "Other.Z")},
			},
		},
		// 支線の列車は F2 発 98 で、J には 100 ごろ着く（J 発 104 は見込みから4分ずれる）
		branch: {
			Stations: []string{"Branch.F1", "Branch.F2", "Branch.J"},
			Departures: map[string][]Departure{
				"Branch.F1": {dep("Branch.F1", 96, "Local", "Other.Z")},
				"Branch.F2": {dep("Branch.F2", 98, "Local", "Other.Z")},
			},
		},
	}
	connect := map[string][]string{"Branch.J": {"Main.J"}}
	inferred := map[LineKey][]Train{main: Infer(lines[main]), branch: Infer(lines[branch])}

	if _, n := MarkEntering(lines, inferred, connect); n != 0 {
		t.Errorf("marked %d, want 0", n)
	}
}
