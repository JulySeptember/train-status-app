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
//
// 直通運転の列車（throughChains）は、境目の駅でつないだ1本の列車として入れる。
// つないだ元の列車を区間（segment）と呼び、区間の番号は元の向き（逆向きの路線網でも）の順にする。
// 境目では、前の区間の終点では乗らず、次の区間の始発駅では降りない（その駅で乗り降りするなら、
// 乗り換えの対応表で隣の駅へ歩く）。
type pattern struct {
	stops []int32

	// 区間ごとの路線・方向（Strings の番号）。直通運転でなければ1つ
	railways   []int32
	directions []int32

	// segs[i] は i 番目の停車駅の区間の番号。区間が1つなら nil
	segs []int32

	// noBoard[i]・noAlight[i] は、i 番目の停車駅で乗れない・降りられない（境目の駅）。区間が1つなら nil
	noBoard  []bool
	noAlight []bool

	// trains[k*len(railways)+s] は k 番目の列車の区間 s（slim の Trains の番号）
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

// trips は列車の本数を返す。
func (p *pattern) trips() int {
	return len(p.trains) / len(p.railways)
}

// segAt は i 番目の停車駅の区間の番号を返す。
func (p *pattern) segAt(i int) int {
	if p.segs == nil {
		return 0
	}
	return int(p.segs[i])
}

func (p *pattern) canBoard(i int) bool {
	return p.noBoard == nil || !p.noBoard[i]
}

func (p *pattern) canAlight(i int) bool {
	return p.noAlight == nil || !p.noAlight[i]
}

// train は k 番目の列車の区間 seg の、slim の Trains の番号を返す。
func (p *pattern) train(k, seg int) int32 {
	return p.trains[k*len(p.railways)+seg]
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
	// 区間ごとの列車（slim の Trains の番号。元の向きの順）
	trains []int32

	stops  []int32
	board  []int32
	alight []int32

	// 直通運転の列車だけ（pattern の同じ名前の項目を参照）
	segs     []int32
	noBoard  []bool
	noAlight []bool
}

func (e *Engine) buildNetworks(calendars []string) *networkPair {

	active := make(map[int32]bool)
	for i, s := range e.tt.Strings {
		if slices.Contains(calendars, s) {
			active[int32(i)] = true
		}
	}

	var forward, backward []tripTimes

	add := func(t tripTimes) {
		forward = append(forward, t)
		backward = append(backward, reverseTrip(t))
	}

	// 直通運転の列車は、つないだ1本の列車として入れる（元の列車は入れない）
	for _, chain := range e.chains {
		if active[e.tt.Trains[chain[0]].Calendar] {
			add(e.chainTrip(chain))
		}
	}

	for ti, train := range e.tt.Trains {

		if !active[train.Calendar] || len(train.Stops) < 2 || e.inChain[int32(ti)] {
			continue
		}

		add(e.chainTrip([]int32{int32(ti)}))
	}

	return &networkPair{
		forward:  e.buildNetwork(forward, false),
		backward: e.buildNetwork(backward, true),
	}
}

// chainTrip は、列車（直通運転なら境目でつないだ複数の列車）の停車駅と時刻を並べる。
func (e *Engine) chainTrip(chain []int32) tripTimes {

	t := tripTimes{trains: chain}

	through := len(chain) > 1

	for seg, ti := range chain {
		train := e.tt.Trains[ti]

		for i, stop := range train.Stops {
			arr, dep := int32(stop.Arrival), int32(stop.Departure)
			if arr == int32(slim.NoTime) {
				arr = dep
			}
			if dep == int32(slim.NoTime) {
				dep = arr
			}

			// 境目: 前の区間の終点では乗らず、次の区間の始発駅では降りない。
			// 時刻が逆戻りしないよう、使わない側の時刻はもう一方にそろえる
			noBoard := through && i == len(train.Stops)-1 && seg < len(chain)-1
			noAlight := through && i == 0 && seg > 0
			if noBoard {
				dep = arr
			}
			if noAlight {
				arr = dep
			}

			t.stops = append(t.stops, e.stationOf[stop.Station])
			t.board = append(t.board, dep)
			t.alight = append(t.alight, arr)

			if through {
				t.segs = append(t.segs, int32(seg))
				t.noBoard = append(t.noBoard, noBoard)
				t.noAlight = append(t.noAlight, noAlight)
			}
		}
	}

	return t
}

// reverseTrip は停車駅の並びを逆にし、時刻に -1 を掛ける。
// 逆向きでは、元の到着時刻が「乗れる時刻」、元の発車時刻が「降りられる時刻」になる
// （境目の乗れない・降りられない駅も入れ替わる）。区間の番号は元の向きのまま。
func reverseTrip(t tripTimes) tripTimes {

	n := len(t.stops)

	r := tripTimes{
		trains: t.trains,
		stops:  make([]int32, n),
		board:  make([]int32, n),
		alight: make([]int32, n),
	}

	if t.segs != nil {
		r.segs = make([]int32, n)
		r.noBoard = make([]bool, n)
		r.noAlight = make([]bool, n)
	}

	for i := range n {
		j := n - 1 - i
		r.stops[i] = t.stops[j]
		r.board[i] = -t.alight[j]
		r.alight[i] = -t.board[j]

		if t.segs != nil {
			r.segs[i] = t.segs[j]
			r.noBoard[i] = t.noAlight[j]
			r.noAlight[i] = t.noBoard[j]
		}
	}

	return r
}

func (e *Engine) buildNetwork(trips []tripTimes, reversed bool) *network {

	n := &network{
		reversed:        reversed,
		stationPatterns: make([][]patternStop, len(e.stationIDs)),
		transfers:       make([][]footpath, len(e.stationIDs)),
	}

	// 路線・方向（直通運転なら区間ごと）と停車駅の並びが同じ列車をまとめる（遅れは路線・方向ごとに足すため）
	groups := make(map[string][]tripTimes)
	var keys []string

	for _, t := range trips {

		var b strings.Builder
		for _, ti := range t.trains {
			train := e.tt.Trains[ti]
			fmt.Fprint(&b, train.Railway, ",", train.RailDirection, ";")
		}
		for i, s := range t.stops {
			fmt.Fprint(&b, ",", s)
			if t.segs != nil {
				fmt.Fprint(&b, ":", t.segs[i])
			}
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
			n.addPattern(e, s)
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

func (n *network) addPattern(e *Engine, trips []tripTimes) {

	first := trips[0]
	stops := first.stops
	size := len(stops)

	p := pattern{
		stops:    stops,
		segs:     first.segs,
		noBoard:  first.noBoard,
		noAlight: first.noAlight,
		trains:   make([]int32, 0, len(trips)*len(first.trains)),
		board:    make([]int32, 0, len(trips)*size),
		alight:   make([]int32, 0, len(trips)*size),
	}

	for _, ti := range first.trains {
		train := e.tt.Trains[ti]
		p.railways = append(p.railways, train.Railway)
		p.directions = append(p.directions, train.RailDirection)
	}

	for _, t := range trips {
		p.trains = append(p.trains, t.trains...)
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
