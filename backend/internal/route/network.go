package route

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"train-status-app/backend/assets/slim"
)

// network は、ある日のダイヤの列車を路線パターンごとにまとめたもの。
// backward は時刻を反転させた（時刻に -1 を掛け、停車駅の並びを逆にした）路線網で、
// 到着時刻からさかのぼって探すときに使う。
type network struct {
	reversed bool

	patterns []pattern

	// 駅 → その駅を通るパターンと、パターン内の位置
	stationPatterns [][]patternStop

	// 駅 → 乗り換えられる駅
	transfers [][]footpath
}

// pattern は、停車駅の並びが同じ列車をまとめたもの。
// 列車は追い越しが無いように並べてあり、どの停車駅でも発車時刻の順になっている。
type pattern struct {
	railway   int32 // Strings の番号
	direction int32 // Strings の番号
	stops     []int32

	// trains[k] は k 番目の列車（slim の Trains の番号）
	trains []int32

	// 列車 k の i 番目の停車駅での時刻は board[k*len(stops)+i] など
	board  []int32 // 乗れる時刻（発車時刻。無ければ到着時刻）
	alight []int32 // 降りられる時刻（到着時刻。無ければ発車時刻）
}

type patternStop struct {
	pattern int32
	index   int32
}

type footpath struct {
	to      int32
	minutes int32
}

func (p *pattern) boardAt(trip, i int) int32 {
	return p.board[trip*len(p.stops)+i]
}

func (p *pattern) alightAt(trip, i int) int32 {
	return p.alight[trip*len(p.stops)+i]
}

// earliestTrip は、i 番目の停車駅で時刻 t 以降に乗れる最初の列車を、
// limit 番より前から探す。無ければ -1。時刻は sh で遅れを足して比べる。
func (p *pattern) earliestTrip(i int, t int32, limit int, sh shift) int {
	lo, hi := 0, limit
	for lo < hi {
		mid := (lo + hi) / 2
		if sh.apply(p.boardAt(mid, i)) < t {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	if lo == limit {
		return -1
	}
	return lo
}

type tripTimes struct {
	train  int32
	stops  []int32
	board  []int32
	alight []int32
}

func (e *Engine) buildNetworks(calendars []string) *networkPair {

	active := make(map[int32]bool)
	for i, s := range e.tt.Strings {
		if slices.Contains(calendars, s) {
			active[int32(i)] = true
		}
	}

	var forward, backward []tripTimes

	for ti, train := range e.tt.Trains {

		if !active[train.Calendar] || len(train.Stops) < 2 {
			continue
		}

		t := tripTimes{
			train:  int32(ti),
			stops:  make([]int32, len(train.Stops)),
			board:  make([]int32, len(train.Stops)),
			alight: make([]int32, len(train.Stops)),
		}

		for i, stop := range train.Stops {
			arr, dep := int32(stop.Arrival), int32(stop.Departure)
			if arr == int32(slim.NoTime) {
				arr = dep
			}
			if dep == int32(slim.NoTime) {
				dep = arr
			}

			t.stops[i] = e.stationOf[stop.Station]
			t.board[i] = dep
			t.alight[i] = arr
		}

		forward = append(forward, t)
		backward = append(backward, reverseTrip(t))
	}

	return &networkPair{
		forward:  e.buildNetwork(forward, false),
		backward: e.buildNetwork(backward, true),
	}
}

// reverseTrip は停車駅の並びを逆にし、時刻に -1 を掛ける。
// 逆向きでは、元の到着時刻が「乗れる時刻」、元の発車時刻が「降りられる時刻」になる。
func reverseTrip(t tripTimes) tripTimes {

	n := len(t.stops)

	r := tripTimes{
		train:  t.train,
		stops:  make([]int32, n),
		board:  make([]int32, n),
		alight: make([]int32, n),
	}

	for i := range n {
		j := n - 1 - i
		r.stops[i] = t.stops[j]
		r.board[i] = -t.alight[j]
		r.alight[i] = -t.board[j]
	}

	return r
}

func (e *Engine) buildNetwork(trips []tripTimes, reversed bool) *network {

	n := &network{
		reversed:        reversed,
		stationPatterns: make([][]patternStop, len(e.stationIDs)),
		transfers:       make([][]footpath, len(e.stationIDs)),
	}

	// 路線・方向と停車駅の並びが同じ列車をまとめる（遅れは路線・方向ごとに足すため）
	groups := make(map[string][]tripTimes)
	var keys []string

	for _, t := range trips {
		train := e.tt.Trains[t.train]

		var b strings.Builder
		fmt.Fprint(&b, train.Railway, ",", train.RailDirection)
		for _, s := range t.stops {
			fmt.Fprint(&b, ",", s)
		}
		key := b.String()

		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], t)
	}

	for _, key := range keys {

		group := groups[key]

		slices.SortStableFunc(group, func(a, b tripTimes) int {
			return cmp.Compare(a.board[0], b.board[0])
		})

		// 追い越しがあると、どの停車駅でも発車時刻の順という前提が崩れるので、別のパターンに分ける
		var split [][]tripTimes

	next:
		for _, t := range group {
			for i, s := range split {
				if !overtakes(s[len(s)-1], t) {
					split[i] = append(s, t)
					continue next
				}
			}
			split = append(split, []tripTimes{t})
		}

		for _, s := range split {
			train := e.tt.Trains[s[0].train]
			n.addPattern(train.Railway, train.RailDirection, s)
		}
	}

	for _, tr := range e.transfers {

		from, ok := e.stations[tr.From]
		if !ok {
			continue
		}
		to, ok := e.stations[tr.To]
		if !ok {
			continue
		}

		// 逆向きでは、乗り換えの向きも逆になる
		if reversed {
			from, to = to, from
		}

		n.transfers[from] = append(n.transfers[from], footpath{
			to:      to,
			minutes: int32(tr.Minutes),
		})
	}

	return n
}

// overtakes は、後に出る列車 b が、どこかの駅で先に出た列車 a 以前に着くか発車するかを返す。
func overtakes(a, b tripTimes) bool {
	for i := range a.stops {
		if b.board[i] < a.board[i] || b.alight[i] < a.alight[i] {
			return true
		}
	}
	return false
}

func (n *network) addPattern(railway, direction int32, trips []tripTimes) {

	stops := trips[0].stops
	size := len(stops)

	p := pattern{
		railway:   railway,
		direction: direction,
		stops:     stops,
		trains:    make([]int32, len(trips)),
		board:     make([]int32, 0, len(trips)*size),
		alight:    make([]int32, 0, len(trips)*size),
	}

	for k, t := range trips {
		p.trains[k] = t.train
		p.board = append(p.board, t.board...)
		p.alight = append(p.alight, t.alight...)
	}

	index := int32(len(n.patterns))
	n.patterns = append(n.patterns, p)

	// 大江戸線の環状部のように同じ駅を2回通るパターンでは、駅ごとに2件入る
	for i, s := range stops {
		n.stationPatterns[s] = append(n.stationPatterns[s], patternStop{
			pattern: index,
			index:   int32(i),
		})
	}
}
