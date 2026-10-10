package infer

import (
	"fmt"
	"slices"
	"strings"
)

// RawRailway は、ODPT の路線（odpt:Railway）のうち推定に使う項目。
type RawRailway struct {
	SameAs       string         `json:"owl:sameAs"`
	Operator     string         `json:"odpt:operator"`
	Ascending    string         `json:"odpt:ascendingRailDirection"`
	StationOrder []StationOrder `json:"odpt:stationOrder"`
}

type StationOrder struct {
	Index   int    `json:"odpt:index"`
	Station string `json:"odpt:station"`
}

// RawStationTimetable は、ODPT の駅時刻表（odpt:StationTimetable）のうち推定に使う項目。
// 始発の印（odpt:isOrigin）は model.StationTimetableEntry に無い（常駐するので項目を増やさない）ので、ここで読む。
type RawStationTimetable struct {
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

// LineKey は Line を分ける単位（路線・方向・ダイヤ種別）。
type LineKey struct {
	Railway       string
	RailDirection string
	Calendar      string
}

// Minutes は "HH:MM" を運行日の0時からの分にする（3時前は +24時間）。
func Minutes(v string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil || len(v) != 5 {
		return 0, false
	}
	if h < 3 {
		h += 24
	}
	return h*60 + m, true
}

// Lines は、駅時刻表を路線・方向・ダイヤ種別ごとの Line にする。keep が true の路線だけを使う。
// 駅の並びは路線の odpt:stationOrder で、昇る向きでなければ逆にし、行先から向きを確かめ直す（Orient）。
// 駅の並びに無い駅の発車は、並びの中の位置を推定して入れる（Place）。
func Lines(railways []RawRailway, timetables []RawStationTimetable, keep func(railway string) bool) map[LineKey]Line {

	byRailway := make(map[string]RawRailway)
	for _, r := range railways {
		byRailway[r.SameAs] = r
	}

	lines := make(map[LineKey]Line)

	for _, tt := range timetables {
		r, ok := byRailway[tt.Railway]
		if !ok || !keep(tt.Railway) {
			continue
		}

		id := LineKey{tt.Railway, tt.RailDirection, tt.Calendar}
		l, ok := lines[id]
		if !ok {
			order := slices.Clone(r.StationOrder)
			slices.SortFunc(order, func(a, b StationOrder) int { return a.Index - b.Index })
			for _, o := range order {
				l.Stations = append(l.Stations, o.Station)
			}
			if tt.RailDirection != r.Ascending {
				slices.Reverse(l.Stations)
			}
			l.Departures = make(map[string][]Departure)
		}

		for _, o := range tt.Objects {
			m, ok := Minutes(o.DepartureTime)
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
		lines[id] = Place(Orient(l))
	}

	return lines
}

// Place は、駅の並びに無い駅（東急大井町線の二子新地・高津）を、並びのどこかに入れた Line を返す。
// 入れられる位置をすべて試し、推定した列車の数が最も少なくなる（発車が最もよくつながる）位置にする。
func Place(l Line) Line {

	known := make(map[string]bool, len(l.Stations))
	for _, s := range l.Stations {
		known[s] = true
	}

	var unplaced []string
	for s := range l.Departures {
		if !known[s] {
			unplaced = append(unplaced, s)
		}
	}
	slices.Sort(unplaced)

	for _, station := range unplaced {

		// その駅だけを加えた Line で、位置ごとの列車の数を比べる
		best, bestTrains := -1, 0
		for pos := 0; pos <= len(l.Stations); pos++ {
			stations := slices.Insert(slices.Clone(l.Stations), pos, station)
			trains := len(Infer(Line{Stations: stations, Departures: l.Departures}))
			if best < 0 || trains < bestTrains {
				best, bestTrains = pos, trains
			}
		}

		l = Line{Stations: slices.Insert(slices.Clone(l.Stations), best, station), Departures: l.Departures}
	}

	return l
}

// estimatedMarker は、推定した列車の ID に入れる印（odpt.Train:<事業者>.<路線>.Estimated.<ダイヤ>.<方向>.<番号>）。
const estimatedMarker = ".Estimated."

// TrainID は、推定した列車 n 番目（Infer の結果の順）の列車ID を作る。
func TrainID(k LineKey, n int) string {
	last := func(id string) string {
		_, v, _ := strings.Cut(id, ":")
		return v
	}
	return fmt.Sprintf("odpt.Train:%s%s%s.%s.%04d",
		last(k.Railway), estimatedMarker, last(k.Calendar), last(k.RailDirection), n)
}

// IsEstimated は、列車ID が推定した列車のものかを返す。
func IsEstimated(train string) bool {
	return strings.Contains(train, estimatedMarker)
}
