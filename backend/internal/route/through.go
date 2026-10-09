package route

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"train-status-app/backend/assets/slim"
)

// MaxThroughGapMinutes は、直通運転の境目の駅で、前の列車の到着から次の列車の発車までの上限（分）。
// 列車番号の数字が同じなら MaxThroughGapSameNumberMinutes まで認める（京王線 4831 → 相模原線 4831 は6分）
const (
	MaxThroughGapMinutes           = 5
	MaxThroughGapSameNumberMinutes = 10

	// MinThroughShare は、境目で直通先として認める路線の、最も多い路線に対する割合
	MinThroughShare = 0.1
)

// throughChains は、直通運転で1本の列車として走る列車の並び（slim の Trains の番号。2本以上）を返す。
//
// 列車時刻表は事業者（路線）ごとに分かれていて、直通運転の列車は境目の駅で別々の列車として載っている
// （odpt:nextTrainTimetable は事業者をまたがない）。次の条件をすべて満たす2本をつなぐ
// （docs/design/multi-operator.md 7.2）:
//   - 前の列車の終点と次の列車の始発駅が、同じ駅か乗り換えの対応表でつながる駅
//   - 同じダイヤ種別で、路線が違う
//   - 前の列車の到着から次の列車の発車まで 0〜MaxThroughGapMinutes 分（列車番号の数字が同じなら
//     MaxThroughGapSameNumberMinutes 分）
//   - 次の列車の行先が前の列車の行先と同じか、次の列車が前の列車の行先に停車する
//
// 境目の駅には、近くの別の駅（京王線と京王新線の新宿など）も入るので、間の短さだけで選ぶと、
// 境目で数分停車する本当の直通先より、隣の駅をすぐに出る別の列車を選んでしまう。候補が複数あれば、
//  1. 列車番号の数字が同じ（同じ事業者の中や、JR・東武 → メトロなど。事業者によって付け方が違い、
//     都営 → 京王のように合わないこともある）
//  2. 行先が同じ（次の列車が前の列車の行先を通るだけではない）
//  3. 間が短い
//
// の順に選ぶ。同じ順位のものが複数あれば、つながない。
// それでも、行先も同じで隣の駅をすぐに出る列車を選ぶことがある（都営新宿線の新宿 → 京王線の新宿、
// 横須賀線の東京 → 京葉線）。そこで一度つないだあと、前の列車の路線と終点ごとに、直通先の路線が
// 最も多い路線の MinThroughShare 未満しかない路線を外して、つなぎ直す。
// 前の列車の終点に到着時刻が無ければ（都内に絞った切れ目など）、つながない。
func throughChains(tt *slim.TrainTimetables, transfers []Transfer) [][]int32 {

	// 駅 → 境目としてつながる駅（自分を含む）
	junctions := make(map[string][]string)
	for _, tr := range transfers {
		junctions[tr.From] = append(junctions[tr.From], tr.To)
	}

	// 始発駅 → その駅から出る列車
	firsts := make(map[int32][]int32)
	for i, train := range tt.Trains {
		if len(train.Stops) >= 2 {
			firsts[train.Stops[0].Station] = append(firsts[train.Stops[0].Station], int32(i))
		}
	}

	stationIndex := make(map[string]int32, len(tt.Strings))
	for i, s := range tt.Strings {
		stationIndex[s] = int32(i)
	}

	type link struct {
		from, to int32
		gap      int

		// 順位（小さいほど良い）: 列車番号の数字が違えば +2、行先が同じでなければ +1
		rank int
	}

	// better は、同じ前の列車からの候補 x が y より良いか（順位、間の順）
	better := func(x, y link) int {
		return cmp.Or(cmp.Compare(x.rank, y.rank), cmp.Compare(x.gap, y.gap))
	}
	// findLinks は、前の列車ごとに最も良い次の列車を探す。allowed が false の組は使わない
	findLinks := func(allowed func(a, b slim.Train) bool) []link {

		var links []link

		for ai, a := range tt.Trains {

			if len(a.Stops) < 2 || tt.String(a.Destination) == "" {
				continue
			}

			last := a.Stops[len(a.Stops)-1]
			if last.Arrival == slim.NoTime {
				continue
			}
			arrival := int32(last.Arrival)
			number := trainNumberDigits(tt.String(a.TrainNumber))

			var best []link

			candidates := append([]string{tt.String(last.Station)}, junctions[tt.String(last.Station)]...)
			for _, station := range candidates {
				si, ok := stationIndex[station]
				if !ok {
					continue
				}
				for _, bi := range firsts[si] {
					b := tt.Trains[bi]
					if int(bi) == ai || b.Calendar != a.Calendar || b.Railway == a.Railway || !allowed(a, b) {
						continue
					}
					sameNumber := number != "" && number == trainNumberDigits(tt.String(b.TrainNumber))
					limit := MaxThroughGapMinutes
					if sameNumber {
						limit = MaxThroughGapSameNumberMinutes
					}
					gap := int(stopDeparture(b.Stops[0]) - arrival)
					if gap < 0 || gap > limit || !sameDestination(a, b) {
						continue
					}
					l := link{from: int32(ai), to: bi, gap: gap}
					if !sameNumber {
						l.rank += 2
					}
					if a.Destination != b.Destination {
						l.rank++
					}
					switch {
					case len(best) == 0 || better(l, best[0]) < 0:
						best = []link{l}
					case better(l, best[0]) == 0 && !slices.Contains(best, l):
						best = append(best, l)
					}
				}
			}

			if len(best) == 1 {
				links = append(links, best[0])
			}
		}

		return links
	}

	links := findLinks(func(a, b slim.Train) bool { return true })

	// 前の列車の路線と終点ごとに、直通先の路線の数を数え、少ない路線を外してつなぎ直す
	type boundary struct{ railway, station int32 }
	boundaryOf := func(a slim.Train) boundary {
		return boundary{a.Railway, a.Stops[len(a.Stops)-1].Station}
	}
	counts := make(map[boundary]map[int32]int)
	for _, l := range links {
		k := boundaryOf(tt.Trains[l.from])
		if counts[k] == nil {
			counts[k] = make(map[int32]int)
		}
		counts[k][tt.Trains[l.to].Railway]++
	}
	most := make(map[boundary]int)
	for k, c := range counts {
		for _, n := range c {
			most[k] = max(most[k], n)
		}
	}

	links = findLinks(func(a, b slim.Train) bool {
		k := boundaryOf(a)
		return float64(counts[k][b.Railway]) >= MinThroughShare*float64(most[k])
	})

	// 次の列車が複数の列車からつながるときは、最も良いものだけを残す（同じ順位・間ならどれもつながない）
	slices.SortStableFunc(links, func(x, y link) int {
		return cmp.Or(cmp.Compare(x.to, y.to), better(x, y))
	})

	next := make(map[int32]int32)
	hasPrev := make(map[int32]bool)

	for i := 0; i < len(links); {
		j := i
		for j < len(links) && links[j].to == links[i].to {
			j++
		}
		if j-i == 1 || better(links[i], links[i+1]) < 0 {
			next[links[i].from] = links[i].to
			hasPrev[links[i].to] = true
		}
		i = j
	}

	var chains [][]int32

	heads := make([]int32, 0, len(next))
	for from := range next {
		if !hasPrev[from] {
			heads = append(heads, from)
		}
	}
	slices.Sort(heads)

	for _, head := range heads {
		chain := []int32{head}
		for t, ok := next[head]; ok; t, ok = next[t] {
			chain = append(chain, t)
		}
		chains = append(chains, chain)
	}

	return chains
}

// sameDestination は、列車 b が列車 a と同じ行先に向かうか（行先が同じか、b が a の行先に停車する）を返す。
func sameDestination(a, b slim.Train) bool {
	if a.Destination == b.Destination {
		return true
	}
	for _, s := range b.Stops {
		if s.Station == a.Destination {
			return true
		}
	}
	return false
}

// trainNumberDigits は、列車番号の数字の部分（A1017K → 1017）を返す。
func trainNumberDigits(n string) string {
	start := strings.IndexFunc(n, unicode.IsDigit)
	if start < 0 {
		return ""
	}
	end := strings.IndexFunc(n[start:], func(r rune) bool { return !unicode.IsDigit(r) })
	if end < 0 {
		return n[start:]
	}
	return n[start : start+end]
}

// stopDeparture は停車駅の発車時刻（無ければ到着時刻）を返す。
func stopDeparture(s slim.Stop) int32 {
	if s.Departure == slim.NoTime {
		return int32(s.Arrival)
	}
	return int32(s.Departure)
}
