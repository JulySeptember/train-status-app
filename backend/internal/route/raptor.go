package route

import (
	"maps"
	"math"
	"slices"
)

const unreachable = math.MaxInt32 / 2

// readyFrom の特別な値
const (
	readyByOrigin = -2 // 出発駅
	readyByTrain  = -1 // 列車でこの駅に着いた
)

// tripLabel は、列車で駅に着いたときに乗っていた列車。
type tripLabel struct {
	round   int
	pattern int32
	trip    int32
	board   int32 // パターン内の乗った位置
	alight  int32 // パターン内の降りた位置
}

// rounds[k] は、列車に k 本まで乗ったときの状態。
type round struct {
	// arrival[s] は駅 s に列車で着ける最も早い時刻
	arrival []int32
	label   []tripLabel

	// ready[s] は駅 s で次の列車に乗れる最も早い時刻
	// （同じ駅での乗り継ぎ時間、または別の駅からの乗り換え時間を足したもの）
	ready []int32
	// readyFrom[s] は ready の理由。readyByOrigin、readyByTrain、または乗り換え元の駅
	readyFrom []int32
}

func newRound(stations int) round {
	r := round{
		arrival:   make([]int32, stations),
		label:     make([]tripLabel, stations),
		ready:     make([]int32, stations),
		readyFrom: make([]int32, stations),
	}
	for i := range stations {
		r.arrival[i] = unreachable
		r.ready[i] = unreachable
	}
	return r
}

func (r round) clone() round {
	return round{
		arrival:   append([]int32(nil), r.arrival...),
		label:     append([]tripLabel(nil), r.label...),
		ready:     append([]int32(nil), r.ready...),
		readyFrom: append([]int32(nil), r.readyFrom...),
	}
}

// search は from のいずれかの駅を時刻 t 以降に出て、to のいずれかの駅に最も早く着く経路を、
// 乗り換え回数ごとに求める。
// 逆向きの路線網では、結果を元の向き（出発と到着、時刻の符号）に戻して返す。
func (n *network) search(
	e *Engine,
	from, to []int32,
	t int,
	maxTransfers int,
	c *conditions,
) []Journey {

	stations := len(e.stationIDs)
	sameStation := int32(e.cfg.SameStationMinutes)

	// toTarget[s] は、駅 s から到着駅のいずれかまで歩く時間（到着駅なら 0。歩けなければ unreachable）。
	// 到着駅の近くの別の駅（東京に対する大手町など）で降りて歩く経路も探す
	toTarget := make([]int32, stations)
	for i := range toTarget {
		toTarget[i] = unreachable
	}
	isTarget := make([]bool, stations)
	for _, s := range to {
		isTarget[s] = true
		toTarget[s] = 0
	}
	for s, paths := range n.transfers {
		for _, f := range paths {
			if isTarget[f.to] && f.minutes < toTarget[s] {
				toTarget[s] = f.minutes
			}
		}
	}

	rounds := make([]round, 1, maxTransfers+2)
	rounds[0] = newRound(stations)
	for _, s := range from {
		rounds[0].ready[s] = int32(t)
		rounds[0].readyFrom[s] = readyByOrigin
	}

	marked := slices.Clone(from)

	// 出発駅から乗り換えの対応表で歩いて、近くの別の駅（大手町に対する東京など）から乗る経路も探す
	for _, s := range from {
		for _, f := range n.transfers[s] {
			if r := int32(t) + f.minutes; r < rounds[0].ready[f.to] {
				rounds[0].ready[f.to] = r
				rounds[0].readyFrom[f.to] = readyByOrigin
				marked = append(marked, f.to)
			}
		}
	}

	slices.Sort(marked)
	marked = slices.Compact(marked)

	// best[k] は列車に k 本まで乗ったときに、到着駅のいずれかに最も早く着く時刻と、その駅
	best := []int32{unreachable}
	bestStation := []int32{-1}

	for k := 1; k <= maxTransfers+1 && len(marked) > 0; k++ {

		prev := rounds[k-1]
		cur := prev.clone()
		bound := best[k-1]
		boundStation := bestStation[k-1]

		// 印の付いた駅を通るパターンと、その中で最初に印の付いた位置。
		// 到着時刻が同じ経路のどれを選ぶかが毎回同じになるよう、パターンの番号順に調べる
		first := make(map[int32]int32)
		for _, s := range marked {
			for _, ps := range n.stationPatterns[s] {
				if i, ok := first[ps.pattern]; !ok || ps.index < i {
					first[ps.pattern] = ps.index
				}
			}
		}
		patterns := slices.Sorted(maps.Keys(first))

		var arrived []int32
		readyMarked := make(map[int32]bool)

		for _, pi := range patterns {

			start := first[pi]

			p := &n.patterns[pi]

			shifts := c.shifts(n, p)

			trip, board := -1, int32(-1)

			for i := int(start); i < len(p.stops); i++ {

				s := p.stops[i]

				seg := p.segAt(i)
				if c.avoided(p, seg) {
					// 使わない路線の区間には乗り続けない（直通運転の列車は、その手前で降りる）
					trip = -1
					continue
				}
				sh := shifts[seg]

				if trip >= 0 && p.canAlight(i) {
					a := sh.apply(p.alightAt(trip, i))

					// 到着駅にすでにもっと早く着けるなら、この先を調べる必要はない
					if a < cur.arrival[s] && a < bound {
						cur.arrival[s] = a
						cur.label[s] = tripLabel{
							round:   k,
							pattern: pi,
							trip:    int32(trip),
							board:   board,
							alight:  int32(i),
						}
						arrived = append(arrived, s)

						if w := toTarget[s]; w < unreachable && a+w < bound {
							bound, boundStation = a+w, s
						}

						if a+sameStation < cur.ready[s] {
							cur.ready[s] = a + sameStation
							cur.readyFrom[s] = readyByTrain
							readyMarked[s] = true
						}
					}
				}

				// この駅で、今の列車より早い列車に乗れるか
				r := prev.ready[s]
				if r >= unreachable || !p.canBoard(i) {
					continue
				}

				limit := p.trips()
				if trip >= 0 {
					limit = trip
				}

				if et := p.earliestTrip(i, r, limit, sh); et >= 0 {
					trip, board = et, int32(i)
				}
			}
		}

		// 乗り換え
		slices.Sort(arrived)
		arrived = slices.Compact(arrived)

		for _, s := range arrived {
			a := cur.arrival[s]
			for _, f := range n.transfers[s] {
				if a+f.minutes < cur.ready[f.to] {
					cur.ready[f.to] = a + f.minutes
					cur.readyFrom[f.to] = s
					readyMarked[f.to] = true
				}
			}
		}

		rounds = append(rounds, cur)
		best = append(best, bound)
		bestStation = append(bestStation, boundStation)

		marked = slices.Sorted(maps.Keys(readyMarked))
	}

	var result []Journey

	for k := 1; k < len(rounds); k++ {
		if best[k] >= best[k-1] {
			continue
		}

		j := n.journey(e, c, rounds, k, bestStation[k], int32(t), toTarget[bestStation[k]])

		// 乗り換えが多いのに、少ない経路より早く着かないものは除く
		if len(result) > 0 && j.Transfers() <= result[len(result)-1].Transfers() {
			continue
		}

		result = append(result, j)
	}

	return result
}

// journey は rounds[k] で駅 to に着いた経路をさかのぼって組み立てる。
// t は探索の出発時刻、walk は駅 to から到着駅まで歩く時間。
// 経路の出発・到着の時刻は、出発駅・到着駅で歩く時間を含める（区間の時刻は列車の時刻のまま）。
func (n *network) journey(e *Engine, c *conditions, rounds []round, k int, to int32, t, walk int32) Journey {

	var legs []Leg

	l := rounds[k].label[to]

	for {
		p := &n.patterns[l.pattern]

		// さかのぼって組み立てるので、元の向きでは後の区間から足す（逆向きでは元の向きの順になる）
		parts := n.legs(e, p, c.shifts(n, p), l)
		if !n.reversed {
			slices.Reverse(parts)
		}
		legs = append(legs, parts...)

		b := p.stops[l.board]
		prev := rounds[l.round-1]

		switch from := prev.readyFrom[b]; from {
		case readyByOrigin:
			// 出発駅から乗った駅まで歩いた時間（出発駅から乗ったなら 0）
			start := int(prev.ready[b] - t)
			end := int(walk)

			if !n.reversed {
				// さかのぼって組み立てたので、出発順に並べ直す
				for i, j := 0, len(legs)-1; i < j; i, j = i+1, j-1 {
					legs[i], legs[j] = legs[j], legs[i]
				}
			} else {
				// 逆向きでは、探索の出発駅が元の向きの到着駅
				start, end = end, start
			}
			return Journey{
				Departure: legs[0].Departure - start,
				Arrival:   legs[len(legs)-1].Arrival + end,
				Legs:      legs,
			}
		case readyByTrain:
			l = prev.label[b]
		default:
			l = prev.label[from]
		}
	}
}

// legs は乗った区間を、元の向きの Leg にする。直通運転の列車は、元の列車（区間）ごとに分け、
// 2つ目からを Through にする。元の向きの順に返す。
func (n *network) legs(e *Engine, p *pattern, shifts []shift, l tripLabel) []Leg {

	size := len(p.stops)
	trip := int(l.trip)

	// 元の向きの位置 j を、パターンの位置にする
	at := func(j int) int {
		if n.reversed {
			return size - 1 - j
		}
		return j
	}

	// 元の向きの位置 j での、遅れを足した発車・到着の時刻と、時刻表の発車時刻
	departure := func(j int) (int32, int32) {
		i := at(j)
		sh := shifts[p.segAt(i)]
		if n.reversed {
			return -sh.apply(p.alightAt(trip, i)), -p.alightAt(trip, i)
		}
		return sh.apply(p.boardAt(trip, i)), p.boardAt(trip, i)
	}
	arrival := func(j int) int32 {
		i := at(j)
		sh := shifts[p.segAt(i)]
		if n.reversed {
			return -sh.apply(p.boardAt(trip, i))
		}
		return sh.apply(p.alightAt(trip, i))
	}

	from, to := int(l.board), int(l.alight)
	if n.reversed {
		from, to = at(to), at(from)
	}

	var result []Leg

	for j := from; j < to; {

		seg := p.segAt(at(j))

		// この区間で降りる位置（区間の終点か、降りる駅）
		end := j
		for end < to && p.segAt(at(end+1)) == seg {
			end++
		}

		train := e.tt.Trains[p.train(trip, seg)]
		dep, scheduled := departure(j)

		result = append(result, Leg{
			Railway:       e.tt.String(train.Railway),
			RailDirection: e.tt.String(train.RailDirection),
			Train:         e.tt.String(train.Train),
			TrainNumber:   e.tt.String(train.TrainNumber),
			TrainType:     e.tt.String(train.TrainType),
			Destination:   e.tt.String(train.Destination),
			From:          e.stationIDs[p.stops[at(j)]],
			To:            e.stationIDs[p.stops[at(end)]],
			Departure:     int(dep),
			Arrival:       int(arrival(end)),
			Delay:         int(dep - scheduled),
			Through:       len(result) > 0,
		})

		// 次の区間は、境目の次の駅（次の区間の始発駅）から
		j = end + 1
	}

	return result
}
