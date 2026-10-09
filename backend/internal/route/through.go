package route

import (
	"cmp"
	"slices"

	"train-status-app/backend/assets/slim"
)

// MaxThroughGapMinutes は、直通運転の境目の駅で、前の列車の到着から次の列車の発車までの上限（分）。
const MaxThroughGapMinutes = 5

// throughChains は、直通運転で1本の列車として走る列車の並び（slim の Trains の番号。2本以上）を返す。
//
// 列車時刻表は事業者（路線）ごとに分かれていて、直通運転の列車は境目の駅で別々の列車として載っている
// （odpt:nextTrainTimetable は事業者をまたがない）。次の条件をすべて満たす2本をつなぐ
// （docs/design/multi-operator.md 7.2）:
//   - 前の列車の終点と次の列車の始発駅が、同じ駅か乗り換えの対応表でつながる駅
//   - 同じダイヤ種別で、路線が違う
//   - 前の列車の到着から次の列車の発車まで 0〜MaxThroughGapMinutes 分
//   - 次の列車の行先が前の列車の行先と同じか、次の列車が前の列車の行先に停車する
//
// 列車番号は事業者によって付け方が違う（JR 1016K と メトロ A1017K など）ので使わない。
// 条件に合う列車が複数あれば、間の短いものを選ぶ。同じ間のものが複数あれば、つながない。
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
	}
	var links []link

	for ai, a := range tt.Trains {

		if len(a.Stops) < 2 || tt.String(a.Destination) == "" {
			continue
		}

		last := a.Stops[len(a.Stops)-1]
		arrival := stopArrival(last)

		var best []link

		candidates := append([]string{tt.String(last.Station)}, junctions[tt.String(last.Station)]...)
		for _, station := range candidates {
			si, ok := stationIndex[station]
			if !ok {
				continue
			}
			for _, bi := range firsts[si] {
				b := tt.Trains[bi]
				if int(bi) == ai || b.Calendar != a.Calendar || b.Railway == a.Railway {
					continue
				}
				gap := int(stopDeparture(b.Stops[0]) - arrival)
				if gap < 0 || gap > MaxThroughGapMinutes || !sameDestination(a, b) {
					continue
				}
				l := link{from: int32(ai), to: bi, gap: gap}
				switch {
				case len(best) == 0 || gap < best[0].gap:
					best = []link{l}
				case gap == best[0].gap && !slices.Contains(best, l):
					best = append(best, l)
				}
			}
		}

		if len(best) == 1 {
			links = append(links, best[0])
		}
	}

	// 次の列車が複数の列車からつながるときは、間の短いものだけを残す（同じ間ならどれもつながない）
	slices.SortStableFunc(links, func(x, y link) int {
		return cmp.Or(cmp.Compare(x.to, y.to), cmp.Compare(x.gap, y.gap))
	})

	next := make(map[int32]int32)
	hasPrev := make(map[int32]bool)

	for i := 0; i < len(links); {
		j := i
		for j < len(links) && links[j].to == links[i].to {
			j++
		}
		if j-i == 1 || links[i].gap < links[i+1].gap {
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

// stopArrival は停車駅の到着時刻（無ければ発車時刻）を返す。
func stopArrival(s slim.Stop) int32 {
	if s.Arrival == slim.NoTime {
		return int32(s.Departure)
	}
	return int32(s.Arrival)
}

// stopDeparture は停車駅の発車時刻（無ければ到着時刻）を返す。
func stopDeparture(s slim.Stop) int32 {
	if s.Departure == slim.NoTime {
		return int32(s.Arrival)
	}
	return int32(s.Departure)
}
