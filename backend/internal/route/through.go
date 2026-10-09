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
//   - 行先が同じ（次の列車が前の列車の行先を通るだけの組は、前の列車の行先より先まで乗り続けさせる
//     誤った組しか無かった）
//
// 境目の駅には、近くの別の駅（京王線と京王新線の新宿など）も入るので、間の短さだけで選ぶと、
// 境目で数分停車する本当の直通先より、隣の駅をすぐに出る別の列車を選んでしまう。候補が複数あれば、
// 列車番号の数字が同じもの（同じ事業者の中や、JR・東武 → メトロなどで多くが一致する。都営 → 京王の
// ように合わないこともある）、間が短いもの、の順に選ぶ。同じ順位のものが複数あれば、つながない。
// それでも、行先も同じで隣の駅をすぐに出る列車を選ぶことがある（都営新宿線の新宿 → 京王線の新宿）。
// そこで一度つないだあと、前の列車の路線と終点ごとに、直通先の路線が最も多い路線の MinThroughShare
// 未満しかない路線を外して、つなぎ直す。
// 前の列車の終点に到着時刻が無ければ（都内に絞った切れ目など）、つながない。
//
// 起動のたびに（Lambda のコールドスタートで）作るので、始発駅ごとの列車を発車時刻で並べ、
// 間の範囲の列車だけを調べる。
func throughChains(tt *slim.TrainTimetables, transfers []Transfer) [][]int32 {

	stationIndex := make(map[string]int32, len(tt.Strings))
	for i, s := range tt.Strings {
		stationIndex[s] = int32(i)
	}

	// 駅 → 境目としてつながる駅（自分を含む）
	junctions := make(map[int32][]int32)
	for _, tr := range transfers {
		from, ok1 := stationIndex[tr.From]
		to, ok2 := stationIndex[tr.To]
		if ok1 && ok2 {
			junctions[from] = append(junctions[from], to)
		}
	}

	// 始発駅 → その駅から出る列車（始発駅の発車時刻の順）
	type first struct {
		train     int32
		departure int32
	}
	firsts := make(map[int32][]first)
	for i, train := range tt.Trains {
		if len(train.Stops) >= 2 {
			s := train.Stops[0]
			firsts[s.Station] = append(firsts[s.Station], first{int32(i), stopDeparture(s)})
		}
	}
	for _, f := range firsts {
		slices.SortFunc(f, func(x, y first) int { return cmp.Compare(x.departure, y.departure) })
	}

	// 列車番号の数字（何度も比べるので先に求める）
	numbers := make([]string, len(tt.Trains))
	for i, train := range tt.Trains {
		numbers[i] = trainNumberDigits(tt.String(train.TrainNumber))
	}

	type link struct {
		from, to int32
		gap      int32

		// 列車番号の数字が違えば 1（小さいほど良い）
		rank int
	}

	// better は、候補 x が y より良いか（順位、間の順）
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

			var best []link

			for _, station := range append([]int32{last.Station}, junctions[last.Station]...) {

				f := firsts[station]
				lo, _ := slices.BinarySearchFunc(f, arrival, func(x first, t int32) int {
					return cmp.Compare(x.departure, t)
				})

				for _, c := range f[lo:] {
					gap := c.departure - arrival
					if gap > MaxThroughGapSameNumberMinutes {
						break
					}

					b := tt.Trains[c.train]
					if int(c.train) == ai || b.Destination != a.Destination || b.Calendar != a.Calendar || b.Railway == a.Railway {
						continue
					}

					sameNumber := numbers[ai] != "" && numbers[ai] == numbers[c.train]
					if !sameNumber && gap > MaxThroughGapMinutes || !allowed(a, b) {
						continue
					}

					l := link{from: int32(ai), to: c.train, gap: gap}
					if !sameNumber {
						l.rank = 1
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

// stopDeparture は停車駅の発車時刻（無ければ到着時刻）を返す。
func stopDeparture(s slim.Stop) int32 {
	if s.Departure == slim.NoTime {
		return int32(s.Arrival)
	}
	return int32(s.Departure)
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
