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
						// 見込みは各駅の所要時間の和なので、途中を通過する急行は見込みより速く着く（下限は設けない）。
						// 下限が無いので、途中の駅から入ってくる列車（直通運転などで始発の印が無い）を、直前に通った
						// 別の列車の続きとしてつなぐことがある（測った各社で、つなぎの約0.2%）。下限（区間ごとの最短の和）や、
						// 見込みとの差の小さい組から決める方法も試したが、急行の多い京王・東武・JR で正しさが下がった
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

// Orient は、駅の並びが列車の進む向きと逆なら、逆にした Line を返す。
// 路線の駅の並び（odpt:stationOrder）と昇る向き（odpt:ascendingRailDirection）から決めた向きが、
// 駅時刻表と合わないことがある（東急新横浜線: 並びは新横浜 → 日吉、昇る向きの Outbound は日吉 → 新横浜）。
// 列車は行先の駅へ向かって進むので、行先が路線の駅である発車のうち、行先が発車の駅より後ろにある数と
// 前にある数を比べ、前にある方が多ければ逆にする（本数の多い路線では、時刻の進み方ではどちらの向きでも
// 次の発車が見つかり、見分けられない）。
func Orient(l Line) Line {

	index := make(map[string]int, len(l.Stations))
	for i, s := range l.Stations {
		index[s] = i
	}

	ahead, behind := 0, 0
	for station, ds := range l.Departures {
		from, ok := index[station]
		if !ok {
			continue
		}
		for _, d := range ds {
			to, ok := index[d.Destination]
			switch {
			case !ok || to == from:
			case to > from:
				ahead++
			default:
				behind++
			}
		}
	}

	if behind > ahead {
		reversed := slices.Clone(l.Stations)
		slices.Reverse(reversed)
		return Line{Stations: reversed, Departures: l.Departures}
	}
	return l
}

// Unplaced は、駅の並びに無い駅の発車の数を返す（Infer はこれらの発車を使わない）。
func Unplaced(l Line) int {
	known := make(map[string]bool, len(l.Stations))
	for _, s := range l.Stations {
		known[s] = true
	}
	n := 0
	for station, ds := range l.Departures {
		if !known[station] {
			n += len(ds)
		}
	}
	return n
}

// Partner は、直通先の列車の始発（駅・発車の分・行先）。
type Partner struct {
	Station     string
	Minutes     int
	Destination string

	// 推定した列車なら、その路線（同じ路線の推定列車は直通先にしない。途中で切れた列車の始発のため）
	Railway string
}

// MaxExtendStations は、直通先を探す、最後の発車より先の駅の数の上限。
// 境目の駅は、最後の発車から多くても数駅先（小田急の急行が下北沢 → 代々木上原で6駅）
const MaxExtendStations = 8

// MaxExtendMinutes は、最後の発車から境目の駅までの所要時間の上限（分）。
// 停車の少ない区間では各駅の所要時間の見込みが膨らむので、見込みだけでは上限にならない
const MaxExtendMinutes = 20

// ExtendToPartners は、行先が路線の外の列車（直通運転で他の路線へ行く）を、直通先の列車へ渡す境目の駅まで延ばす。
//
// 直通する列車は、境目の駅（田園都市線 → 半蔵門線の渋谷、小田急 → 千代田線の代々木上原、西武池袋線 → 西武有楽町線の練馬）
// では発車しない（駅時刻表には直通先の事業者の発車として載る）ので、推定した列車は境目の手前の最後の発車で終わる。
// 最後の発車の駅から MaxExtendStations 駅先までのうち、その駅（か乗り換えでつながる駅）から同じ行先の列車が
// 出ている最初の駅を境目とし、その駅を終点にして、到着をその直通先の発車の分にする（乗り継ぐ間を0分とみなす）。
// 最後の発車の駅そのものが境目なら、その駅の発車の分を到着にする。
//
// 別の列車（1本前の列車の直通先、途中から出る別の列車）を取らないよう、
//   - 所要時間が見込み（各駅の所要時間の和）の半分以上・1.5倍＋3分以下・MaxExtendMinutes 以下の直通先だけを使う
//   - 境目の駅に着く見込みの早い列車から順に、まだ使っていない最も早い直通先を割り当てる。境目の手前は追い越しが
//     できない区間が多い（三軒茶屋 → 渋谷）ので、見込みとの差の小さい組から決めると、急行と各停の直通先が入れ替わる
//   - 1つの直通先には1本だけをつなぐ（used は路線・方向をまたいで共有する）
//   - 同じ路線の推定列車の始発（途中で切れた列車）は使わない
//
// partners は駅（とつながる駅）から出る列車の始発を返す（同じダイヤ種別のもの）。
func ExtendToPartners(l Line, railway string, trains []Train, partners func(station string) []Partner, used map[Partner]bool) []Train {

	run := runTimes(l)
	index := make(map[string]int, len(l.Stations))
	for i, s := range l.Stations {
		index[s] = i
	}

	inRange := func(gap, want int, last bool) bool {
		if last {
			// 最後の発車の駅が境目: その駅の発車と同じか少し後に出る直通先
			return gap >= 0 && gap <= 5
		}
		return gap*2 >= want && gap*2 <= want*3+6 && gap <= MaxExtendMinutes
	}

	// 列車ごとに境目の駅（同じ行先の直通先が範囲内に出ている最初の駅）と、そこに着く見込みを決める
	type target struct {
		train   int
		station int
		want    int
		arrival int
	}
	var targets []target

	for n, tr := range trains {

		last := tr.Stops[len(tr.Stops)-1]
		if tr.Terminal != "" {
			continue
		}
		if _, ok := index[last.Destination]; ok {
			continue
		}

		from := index[last.Station]
		want := 0

	search:
		for j := from; j < len(l.Stations) && j <= from+MaxExtendStations; j++ {
			if j > from {
				want += run[j]
			}
			for _, p := range partners(l.Stations[j]) {
				if p.Destination == last.Destination && (p.Railway == "" || p.Railway != railway) &&
					inRange(p.Minutes-last.Minutes, want, j == from) {
					targets = append(targets, target{train: n, station: j, want: want, arrival: last.Minutes + want})
					break search
				}
			}
		}
	}

	slices.SortStableFunc(targets, func(a, b target) int {
		return cmp.Or(cmp.Compare(a.station, b.station), cmp.Compare(a.arrival, b.arrival), cmp.Compare(a.train, b.train))
	})

	result := slices.Clone(trains)

	for _, t := range targets {

		last := result[t.train].Stops[len(result[t.train].Stops)-1]

		best := -1
		var chosen Partner
		for _, p := range partners(l.Stations[t.station]) {
			if used[p] || p.Destination != last.Destination || (p.Railway != "" && p.Railway == railway) ||
				!inRange(p.Minutes-last.Minutes, t.want, t.station == index[last.Station]) {
				continue
			}
			if best < 0 || p.Minutes < best {
				best, chosen = p.Minutes, p
			}
		}
		if best < 0 {
			continue
		}

		used[chosen] = true
		result[t.train].Terminal = l.Stations[t.station]
		result[t.train].TerminalMinutes = chosen.Minutes
		if t.station == index[last.Station] {
			// 最後の発車の駅が境目: 到着はその駅の発車（到着が発車より後にならないように）
			result[t.train].TerminalMinutes = last.Minutes
		}
	}

	return result
}
