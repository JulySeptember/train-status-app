package infer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// 列車時刻表のある事業者の駅時刻表に推定をかけ、本物の列車（駅時刻表の odpt:train）と比べて正しさを測る
// （docs/design/multi-operator.md 8.2）。
//
// 都営のデータ（assets）は常にある。他社のデータ（.odpt-cache/dumps）は手元にあるときだけ測る。

func readJSONFile[T any](t *testing.T, path string) (T, bool) {
	t.Helper()
	var v T
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return v, false
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v, true
}

// buildLines は、事業者の駅時刻表を Line にする（keep が true の路線だけ）。
// 途中から入ってくる列車に印を付けたもの（MarkEntering。connect は駅 → 乗り換えでつながる駅）も返す。
func buildLines(railways []RawRailway, timetables []RawStationTimetable, operator string, keep func(railway string) bool, connect map[string][]string) (map[string]Line, map[string]Line) {
	ops := make(map[string]bool)
	for _, r := range railways {
		if r.Operator == operator && keep(r.SameAs) {
			ops[r.SameAs] = true
		}
	}
	lines := Lines(railways, timetables, func(r string) bool { return ops[r] })
	inferred := make(map[LineKey][]Train, len(lines))
	for k, l := range lines {
		inferred[k] = Infer(l)
	}
	marked, _ := MarkEntering(lines, inferred, connect)

	str := func(m map[LineKey]Line) map[string]Line {
		result := make(map[string]Line, len(m))
		for k, l := range m {
			result[k.Railway+" "+k.RailDirection+" "+k.Calendar] = l
		}
		return result
	}
	return str(lines), str(marked)
}

type accuracy struct {
	links, correctLinks int
	trains, exact       int

	// 路線の中で2駅以上発車する列車だけ（1駅だけの列車は、1駅の列車として推定すれば必ず正しくなる）
	longTrains, longExact int

	// 終点の到着時刻の見込みと本物の差（分）。本物の列車時刻表がある列車だけ
	terminals []int

	// 途中の駅から入ってくる列車（路線の中の最初の発車に始発の印が無く、行先の駅でもない）の数と、
	// そのうち推定でもその発車から新しい列車として始まった数（前の列車の続きにされなかった数）
	entering, enteringStarted int

	// 各停（列車種別が .Local で終わる）以外の列車の数と、そのうち丸ごと正しく推定できた数
	nonLocal, nonLocalExact int
}

// measure は、推定した列車のつなぎ（隣どうしの発車）のうち同じ本物の列車どうしの割合と、
// 本物の列車のうち推定で丸ごと同じ1本になった割合を数える。
// marked は、lines の途中から入ってくる列車に印を付けたもの（MarkEntering）で、推定はこちらで行う。
// 途中から入ってくる列車は、元の始発の印（lines）で決める。
func measure(lines, marked map[string]Line, arrivals map[[3]string]int) accuracy {

	var a accuracy

	for id, l := range lines {
		calendar := strings.Fields(id)[2]

		index := make(map[string]int, len(l.Stations))
		for i, s := range l.Stations {
			index[s] = i
		}

		truth := make(map[string]int)       // 本物の列車 → 発車の数
		first := make(map[string]Departure) // 本物の列車 → 路線の中の最初の発車
		for _, ds := range l.Departures {
			for _, d := range ds {
				if d.Truth == "" {
					panic("a departure without odpt:train cannot be measured")
				}
				truth[d.Truth]++
				if f, ok := first[d.Truth]; !ok || index[d.Station] < index[f.Station] {
					first[d.Truth] = d
				}
			}
		}
		a.trains += len(truth)
		for _, n := range truth {
			if n >= 2 {
				a.longTrains++
			}
		}
		entering := make(map[string]bool)
		for id, d := range first {
			if !d.Origin && index[d.Station] > 0 {
				entering[id] = true
			}
			if !strings.HasSuffix(d.TrainType, ".Local") {
				a.nonLocal++
			}
		}
		a.entering += len(entering)

		for _, tr := range Infer(marked[id]) {
			if s := tr.Stops[0]; entering[s.Truth] && first[s.Truth].Station == s.Station && first[s.Truth].Minutes == s.Minutes {
				a.enteringStarted++
			}
			same := true
			for i := 1; i < len(tr.Stops); i++ {
				a.links++
				if tr.Stops[i].Truth == tr.Stops[i-1].Truth {
					a.correctLinks++
				} else {
					same = false
				}
			}
			if same && truth[tr.Stops[0].Truth] == len(tr.Stops) {
				a.exact++
				if len(tr.Stops) >= 2 {
					a.longExact++
				}
				if !strings.HasSuffix(tr.Stops[0].TrainType, ".Local") {
					a.nonLocalExact++
				}
				if real, ok := arrivals[[3]string{tr.Stops[0].Truth, calendar, tr.Terminal}]; ok && tr.Terminal != "" {
					a.terminals = append(a.terminals, tr.TerminalMinutes-real)
				}
			}
		}
	}

	return a
}

func (a accuracy) String() string {
	s := fmt.Sprintf("links %d/%d (%.1f%%), trains %d/%d (%.1f%%; 2+ stops %.1f%%)",
		a.correctLinks, a.links, 100*float64(a.correctLinks)/float64(max(a.links, 1)),
		a.exact, a.trains, 100*float64(a.exact)/float64(max(a.trains, 1)),
		100*float64(a.longExact)/float64(max(a.longTrains, 1)))
	s += fmt.Sprintf(", non-local %d/%d (%.1f%%), entering mid-line started %d/%d (%.1f%%)",
		a.nonLocalExact, a.nonLocal, 100*float64(a.nonLocalExact)/float64(max(a.nonLocal, 1)),
		a.enteringStarted, a.entering, 100*float64(a.enteringStarted)/float64(max(a.entering, 1)))
	if len(a.terminals) > 0 {
		d := slices.Clone(a.terminals)
		slices.Sort(d)
		within := 0
		for _, v := range d {
			if v >= -1 && v <= 1 {
				within++
			}
		}
		s += fmt.Sprintf(", terminal arrival error: median %d, p5 %d, p95 %d min, within ±1 min %.1f%% (%d trains)",
			d[len(d)/2], d[len(d)*5/100], d[len(d)*95/100], 100*float64(within)/float64(len(d)), len(d))
	}
	return s
}

type rawTrainTimetable struct {
	Train    string `json:"odpt:train"`
	Calendar string `json:"odpt:calendar"`
	Objects  []struct {
		ArrivalTime    string `json:"odpt:arrivalTime"`
		ArrivalStation string `json:"odpt:arrivalStation"`
	} `json:"odpt:trainTimetableObject"`
}

// arrivalsOf は、列車時刻表から（列車ID, ダイヤ種別, 駅）→ 到着時刻（分）を作る（列車ID はダイヤ種別をまたいで使い回される）。
func arrivalsOf(tts []rawTrainTimetable) map[[3]string]int {
	result := make(map[[3]string]int)
	for _, tt := range tts {
		for _, o := range tt.Objects {
			if m, ok := Minutes(o.ArrivalTime); ok && o.ArrivalStation != "" {
				result[[3]string{tt.Train, tt.Calendar, o.ArrivalStation}] = m
			}
		}
	}
	return result
}

func TestAccuracy(t *testing.T) {

	assetsDir := filepath.Join("..", "..", "assets")
	cacheDir := filepath.Join("..", "..", ".odpt-cache")

	railways, ok1 := readJSONFile[[]RawRailway](t, filepath.Join(assetsDir, "railway.json"))
	timetables, ok2 := readJSONFile[[]RawStationTimetable](t, filepath.Join(assetsDir, "station_timetable.json"))
	trainTimetables, ok3 := readJSONFile[[]rawTrainTimetable](t, filepath.Join(assetsDir, "train_timetable.json"))
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("Toei assets are missing")
	}

	// 大江戸線は環状部に行先が無いので除く。都営の駅データには乗り換えの駅が無い
	toei, toeiMarked := buildLines(railways, timetables, "odpt.Operator:Toei",
		func(r string) bool { return r != "odpt.Railway:Toei.Oedo" }, nil)
	t.Logf("Toei: %v", measure(toei, toeiMarked, arrivalsOf(trainTimetables)))

	// 都営の地下鉄（荒川線を除く）では、ほぼすべての列車を正しく推定できる。
	// 荒川線（路面電車）は、王子駅前の手前で長く停まる電車が途切れる（推定する5社は普通の鉄道なので対象外）
	subway := make(map[string]Line)
	for id, l := range toei {
		if !strings.HasPrefix(id, "odpt.Railway:Toei.Arakawa ") {
			subway[id] = l
		}
	}
	a := measure(subway, toeiMarked, arrivalsOf(trainTimetables))
	t.Logf("Toei subway: %v", a)
	if a.trains < 3000 || a.correctLinks*100 < a.links*99 || a.longExact*100 < a.longTrains*98 {
		t.Errorf("Toei subway accuracy is too low: %v", a)
	}

	for _, op := range []string{"TokyoMetro", "Keio", "Tobu", "JR-East", "TWR", "MIR"} {
		rs, ok := readJSONFile[[]RawRailway](t, filepath.Join(cacheDir, "operators", op, "Railway.json"))
		if !ok {
			continue
		}
		stations, _ := readJSONFile[[]struct {
			SameAs     string   `json:"owl:sameAs"`
			Connecting []string `json:"odpt:connectingStation"`
		}](t, filepath.Join(cacheDir, "operators", op, "Station.json"))
		connect := make(map[string][]string)
		for _, s := range stations {
			connect[s.SameAs] = append(connect[s.SameAs], s.Connecting...)
		}
		tts, trains := cachedDumps(t, cacheDir)
		// 環状の山手線は、大江戸線の環状部と同じく推定の対象外（推定する5社に環状の路線は無い）
		lines, marked := buildLines(rs, tts, "odpt.Operator:"+op,
			func(r string) bool { return r != "odpt.Railway:JR-East.Yamanote" }, connect)
		t.Logf("%s: %v", op, measure(lines, marked, trains))
	}
}

var dumps struct {
	stationTimetables []RawStationTimetable
	arrivals          map[[3]string]int
}

// cachedDumps は、全件版の駅時刻表と列車時刻表を読む（大きいので1回だけ）。
func cachedDumps(t *testing.T, cacheDir string) ([]RawStationTimetable, map[[3]string]int) {
	t.Helper()
	if dumps.arrivals == nil {
		var trains []rawTrainTimetable
		for _, host := range []string{"basic", "challenge"} {
			if v, ok := readJSONFile[[]RawStationTimetable](t, filepath.Join(cacheDir, "dumps", host, "StationTimetable.json")); ok {
				dumps.stationTimetables = append(dumps.stationTimetables, v...)
			}
			if v, ok := readJSONFile[[]rawTrainTimetable](t, filepath.Join(cacheDir, "dumps", host, "TrainTimetable.json")); ok {
				trains = append(trains, v...)
			}
		}
		dumps.arrivals = arrivalsOf(trains)
	}
	return dumps.stationTimetables, dumps.arrivals
}
