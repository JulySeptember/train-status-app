// Package infer は、列車時刻表の無い事業者（東急・西武・小田急・京急・ゆりかもめ）について、
// 駅時刻表の発車をつないで列車の運行を推定する（docs/design/multi-operator.md 8章）。
//
// 駅時刻表は、駅・方向・ダイヤ種別ごとの発車の一覧で、各発車には列車種別と行先がある（列車ID は無い）。
// 路線・方向・ダイヤ種別ごとに、駅の並びの順に、同じ列車種別・行先の発車を、次に停車する駅の発車へつなぐ。
package infer

import (
	"cmp"
	"slices"
	"sort"
)

// Departure は駅時刻表の1件の発車。
type Departure struct {
	Station string
	// 運行日の0時からの分（3時前は +24時間）
	Minutes int

	TrainType   string
	Destination string

	// この駅が始発（odpt:isOrigin）
	Origin bool

	// 本物の列車ID（正しさを測るときだけ。推定には使わない）
	Truth string
}

// Line は、1つの路線・方向・ダイヤ種別の駅時刻表。
type Line struct {
	// 進む向きに並べた駅
	Stations []string

	// 駅 → その駅の発車
	Departures map[string][]Departure
}

// Train は推定した列車。発車を駅の並びの順に持つ。
type Train struct {
	Stops []Departure

	// 行先が路線の駅なら、行先の駅と、その到着時刻の見込み（運行日の0時からの分）。
	// 駅時刻表には終点の到着が無いので、最後の発車に、その区間の所要時間の見込みを足す。
	// 行先が路線に無い（直通運転で他社へ行く、都外へ行く）なら空
	Terminal        string
	TerminalMinutes int
}

type key struct {
	trainType   string
	destination string
}

// chain はつないでいる途中の列車。
type chain struct {
	stops []Departure
	key   key
	last  int // 最後の発車の駅の位置
}

// runTimes は、隣どうしの駅の所要時間（分）の見込みを返す。
// runTimes[i] は駅 i-1 → 駅 i。同じ列車種別・行先の発車のうち、駅 i-1 の発車の後で最も近い駅 i の発車までの
// 時間の中央値。求められなければ 2分。
func runTimes(l Line) []int {

	result := make([]int, len(l.Stations))

	for i := 1; i < len(l.Stations); i++ {

		prev := groupByKey(l.Departures[l.Stations[i-1]])
		cur := groupByKey(l.Departures[l.Stations[i]])

		var gaps []int
		for k, ps := range prev {
			cs := cur[k]
			for _, p := range ps {
				j := sort.Search(len(cs), func(j int) bool { return cs[j].Minutes > p.Minutes })
				if j < len(cs) {
					gaps = append(gaps, cs[j].Minutes-p.Minutes)
				}
			}
		}

		if len(gaps) == 0 {
			result[i] = 2
			continue
		}
		slices.Sort(gaps)
		result[i] = gaps[len(gaps)/2]
	}

	return result
}

func groupByKey(ds []Departure) map[key][]Departure {
	result := make(map[key][]Departure)
	for _, d := range ds {
		k := key{d.TrainType, d.Destination}
		result[k] = append(result[k], d)
	}
	for _, v := range result {
		slices.SortFunc(v, func(a, b Departure) int { return cmp.Compare(a.Minutes, b.Minutes) })
	}
	return result
}

// Infer は路線の発車をつないで列車を推定する。
func Infer(l Line) []Train {

	run := runTimes(l)

	// expected は、駅 from から駅 to までの所要時間の見込み
	expected := func(from, to int) int {
		total := 0
		for i := from + 1; i <= to; i++ {
			total += run[i]
		}
		return total
	}

	var done []*chain
	active := make(map[key][]*chain)

	for i, station := range l.Stations {

		groups := groupByKey(l.Departures[station])

		keys := make([]key, 0, len(groups))
		for k := range groups {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b key) int {
			return cmp.Or(cmp.Compare(a.trainType, b.trainType), cmp.Compare(a.destination, b.destination))
		})

		for _, k := range keys {

			candidates := active[k]

			for _, d := range groups[k] {

				best := -1
				if !d.Origin {
					bestScore := 0
					for ci, c := range candidates {
						gap := d.Minutes - c.stops[len(c.stops)-1].Minutes
						if c.last == i || gap <= 0 {
							continue
						}
						want := expected(c.last, i)
						// 所要時間の見込みより大きく遅いものはつながない。
						// 見込みは各駅の所要時間の和なので、途中を通過する急行は見込みより速く着く（下限は設けない）
						if gap > want*2+10 {
							continue
						}
						score := abs(gap - want)
						if best < 0 || score < bestScore {
							best, bestScore = ci, score
						}
					}
				}

				if best < 0 {
					candidates = append(candidates, &chain{stops: []Departure{d}, key: k, last: i})
					continue
				}

				c := candidates[best]
				c.stops = append(c.stops, d)
				c.last = i
			}

			active[k] = candidates
		}

		// 行先の駅に着いた列車は終える（行先の駅では発車しない）
		for k, cs := range active {
			if k.destination != station {
				continue
			}
			done = append(done, cs...)
			delete(active, k)
		}
	}

	for _, cs := range active {
		done = append(done, cs...)
	}

	index := make(map[string]int, len(l.Stations))
	for i, s := range l.Stations {
		index[s] = i
	}

	trains := make([]Train, 0, len(done))
	for _, c := range done {
		t := Train{Stops: c.stops}
		if dest, ok := index[c.key.destination]; ok && dest > c.last {
			t.Terminal = c.key.destination
			t.TerminalMinutes = c.stops[len(c.stops)-1].Minutes + terminalRun(c, dest, index, expected)
		}
		trains = append(trains, t)
	}

	slices.SortFunc(trains, func(a, b Train) int {
		return cmp.Or(cmp.Compare(a.Stops[0].Minutes, b.Stops[0].Minutes), cmp.Compare(a.Stops[0].Station, b.Stops[0].Station))
	})

	return trains
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// terminalRun は、列車 c の最後の発車から行先の駅 dest までの所要時間の見込み（分）を返す。
// 各駅の所要時間の和を、この列車が走った区間の速さ（実際の時間 ÷ 各駅の所要時間の和）で縮める
// （途中を通過する急行は、各駅の所要時間の和より速い）。
func terminalRun(c *chain, dest int, index map[string]int, expected func(from, to int) int) int {

	want := expected(c.last, dest)

	first := c.stops[0]
	span := expected(index[first.Station], c.last)
	elapsed := c.stops[len(c.stops)-1].Minutes - first.Minutes

	if span <= 0 || elapsed <= 0 {
		return max(want, 1)
	}

	return max((want*elapsed+span/2)/span, 1)
}
