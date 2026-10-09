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

type rawRailway struct {
	SameAs       string `json:"owl:sameAs"`
	Operator     string `json:"odpt:operator"`
	Ascending    string `json:"odpt:ascendingRailDirection"`
	StationOrder []struct {
		Index   int    `json:"odpt:index"`
		Station string `json:"odpt:station"`
	} `json:"odpt:stationOrder"`
}

type rawStationTimetable struct {
	Operator      string `json:"odpt:operator"`
	Railway       string `json:"odpt:railway"`
	Station       string `json:"odpt:station"`
	Calendar      string `json:"odpt:calendar"`
	RailDirection string `json:"odpt:railDirection"`
	Objects       []struct {
		DepartureTime string   `json:"odpt:departureTime"`
		Train         string   `json:"odpt:train"`
		TrainType     string   `json:"odpt:trainType"`
		Destination   []string `json:"odpt:destinationStation"`
		IsOrigin      bool     `json:"odpt:isOrigin"`
	} `json:"odpt:stationTimetableObject"`
}

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

// minutes は "HH:MM" を運行日の0時からの分にする（3時前は +24時間）。
func minutes(v string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 3 {
		h += 24
	}
	return h*60 + m, true
}

// buildLines は、駅時刻表を路線・方向・ダイヤ種別ごとの Line にする。
func buildLines(railways []rawRailway, timetables []rawStationTimetable, operator string) map[string]Line {

	byRailway := make(map[string]rawRailway)
	for _, r := range railways {
		byRailway[r.SameAs] = r
	}

	lines := make(map[string]Line)

	for _, tt := range timetables {
		if tt.Operator != operator {
			continue
		}
		r, ok := byRailway[tt.Railway]
		if !ok {
			continue
		}

		id := tt.Railway + " " + tt.RailDirection + " " + tt.Calendar
		l, ok := lines[id]
		if !ok {
			order := slices.Clone(r.StationOrder)
			slices.SortFunc(order, func(a, b struct {
				Index   int    `json:"odpt:index"`
				Station string `json:"odpt:station"`
			}) int {
				return a.Index - b.Index
			})
			for _, o := range order {
				l.Stations = append(l.Stations, o.Station)
			}
			if tt.RailDirection != r.Ascending {
				slices.Reverse(l.Stations)
			}
			l.Departures = make(map[string][]Departure)
		}

		for _, o := range tt.Objects {
			m, ok := minutes(o.DepartureTime)
			if !ok {
				continue
			}
			l.Departures[tt.Station] = append(l.Departures[tt.Station], Departure{
				Station:     tt.Station,
				Minutes:     m,
				TrainType:   o.TrainType,
				Destination: strings.Join(o.Destination, ","),
				Origin:      o.IsOrigin,
				Truth:       o.Train,
			})
		}

		lines[id] = l
	}

	for id, l := range lines {
		lines[id] = Orient(l)
	}

	return lines
}

type accuracy struct {
	links, correctLinks int
	trains, exact       int

	// 路線の中で2駅以上発車する列車だけ（1駅だけの列車は、1駅の列車として推定すれば必ず正しくなる）
	longTrains, longExact int

	// 終点の到着時刻の見込みと本物の差（分）。本物の列車時刻表がある列車だけ
	terminals []int
}

// measure は、推定した列車のつなぎ（隣どうしの発車）のうち同じ本物の列車どうしの割合と、
// 本物の列車のうち推定で丸ごと同じ1本になった割合を数える。
func measure(lines map[string]Line, arrivals map[[3]string]int) accuracy {

	var a accuracy

	for id, l := range lines {
		calendar := strings.Fields(id)[2]

		truth := make(map[string]int) // 本物の列車 → 発車の数
		for _, ds := range l.Departures {
			for _, d := range ds {
				if d.Truth == "" {
					panic("a departure without odpt:train cannot be measured")
				}
				truth[d.Truth]++
			}
		}
		a.trains += len(truth)
		for _, n := range truth {
			if n >= 2 {
				a.longTrains++
			}
		}

		for _, tr := range Infer(l) {
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
			if m, ok := minutes(o.ArrivalTime); ok && o.ArrivalStation != "" {
				result[[3]string{tt.Train, tt.Calendar, o.ArrivalStation}] = m
			}
		}
	}
	return result
}

func TestAccuracy(t *testing.T) {

	assetsDir := filepath.Join("..", "..", "assets")
	cacheDir := filepath.Join("..", "..", ".odpt-cache")

	railways, ok1 := readJSONFile[[]rawRailway](t, filepath.Join(assetsDir, "railway.json"))
	timetables, ok2 := readJSONFile[[]rawStationTimetable](t, filepath.Join(assetsDir, "station_timetable.json"))
	trainTimetables, ok3 := readJSONFile[[]rawTrainTimetable](t, filepath.Join(assetsDir, "train_timetable.json"))
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("Toei assets are missing")
	}

	// 大江戸線は環状部に行先が無いので除く
	toei := buildLines(railways, timetables, "odpt.Operator:Toei")
	for id := range toei {
		if strings.HasPrefix(id, "odpt.Railway:Toei.Oedo ") {
			delete(toei, id)
		}
	}
	t.Logf("Toei: %v", measure(toei, arrivalsOf(trainTimetables)))

	// 都営の地下鉄（荒川線を除く）では、ほぼすべての列車を正しく推定できる。
	// 荒川線（路面電車）は、王子駅前の手前で長く停まる電車が途切れる（推定する5社は普通の鉄道なので対象外）
	subway := make(map[string]Line)
	for id, l := range toei {
		if !strings.HasPrefix(id, "odpt.Railway:Toei.Arakawa ") {
			subway[id] = l
		}
	}
	a := measure(subway, arrivalsOf(trainTimetables))
	t.Logf("Toei subway: %v", a)
	if a.trains < 3000 || a.correctLinks*100 < a.links*99 || a.longExact*100 < a.longTrains*98 {
		t.Errorf("Toei subway accuracy is too low: %v", a)
	}

	for _, op := range []string{"TokyoMetro", "Keio", "Tobu", "JR-East", "TWR", "MIR"} {
		rs, ok := readJSONFile[[]rawRailway](t, filepath.Join(cacheDir, "operators", op, "Railway.json"))
		if !ok {
			continue
		}
		tts, trains := cachedDumps(t, cacheDir)
		lines := buildLines(rs, tts, "odpt.Operator:"+op)
		// 環状の山手線は、大江戸線の環状部と同じく推定の対象外（推定する5社に環状の路線は無い）
		for id := range lines {
			if strings.HasPrefix(id, "odpt.Railway:JR-East.Yamanote ") {
				delete(lines, id)
			}
		}
		t.Logf("%s: %v", op, measure(lines, trains))
	}
}

var dumps struct {
	stationTimetables []rawStationTimetable
	arrivals          map[[3]string]int
}

// cachedDumps は、全件版の駅時刻表と列車時刻表を読む（大きいので1回だけ）。
func cachedDumps(t *testing.T, cacheDir string) ([]rawStationTimetable, map[[3]string]int) {
	t.Helper()
	if dumps.arrivals == nil {
		var trains []rawTrainTimetable
		for _, host := range []string{"basic", "challenge"} {
			if v, ok := readJSONFile[[]rawStationTimetable](t, filepath.Join(cacheDir, "dumps", host, "StationTimetable.json")); ok {
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
