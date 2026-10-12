// Package route は列車時刻表から経路を探す（RAPTOR）。
//
// 乗り換え回数を1回ずつ増やしながら各駅に最も早く着ける時刻を求め、
// 乗り換え回数ごとに最も早く着く経路を返す。
// 到着時刻を指定した場合は、時刻を反転させた逆向きの路線網で同じ探索を行う。
//
// 駅・路線・ダイヤ種別は ID の文字列のまま扱い、事業者に依存した処理は持たない。
// 乗り換えられる駅の対応表は、呼び出し側が Transfer として渡す。
package route

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"sync"

	"train-status-app/backend/assets/slim"
)

var (
	ErrUnknownStation = errors.New("unknown station")
	ErrSameStation    = errors.New("from and to are the same station")
)

const (
	// DefaultSameStationMinutes は、同じ駅で同じ路線の別の列車に乗り継ぐときに空ける時間（分）。
	// 同じホームか向かいのホームで乗り継げるので、別の路線への乗り換えより短くする。
	DefaultSameStationMinutes = 1

	// DefaultMaxTransfers は乗り換え回数の上限。
	DefaultMaxTransfers = 3
)

type Config struct {
	SameStationMinutes int
	MaxTransfers       int
}

func DefaultConfig() Config {
	return Config{
		SameStationMinutes: DefaultSameStationMinutes,
		MaxTransfers:       DefaultMaxTransfers,
	}
}

// Transfer は、駅 From で降りて駅 To で別の列車に乗れることを表す（片方向）。
// Minutes は歩く時間と余裕を合わせた時間。
type Transfer struct {
	From    string
	To      string
	Minutes int
}

// Query は探索の条件。時刻は運行日の0時からの分（3時前は +24時間）。
type Query struct {
	// From のどの駅から出てもよく、To のどの駅に着いてもよい。
	// 同じ名前で路線ごとに駅が分かれている駅（新宿など）を、まとめて指定するのに使う。
	// From・To から乗り換えの対応表で歩ける駅でも乗り降りする（歩く時間は経路の時刻に含める）
	From []string
	To   []string

	// その日に適用されるダイヤ種別（calendar.Calendars の結果）
	Calendars []string

	// Time は ArriveBy が false なら出発時刻、true なら到着時刻
	Time     int
	ArriveBy bool

	// MaxTransfers は乗り換え回数の上限。Config.MaxTransfers を超える値は丸める
	MaxTransfers int

	// Avoid に含まれる路線は使わない
	Avoid []string

	// Delays は路線・方向ごとの遅れ。DelayUntil（運行日の0時からの分）までの時刻に遅れを足す。
	// それより後の時刻は、DelayUntil + 遅れ より前にならないように後ろへずらす
	// （遅れは現時点の値なので、先の列車ほど元のダイヤに戻っていくとみなす）
	Delays     []Delay
	DelayUntil int

	// Count は返す経路の数の目安。0 なら指定した時刻で1回だけ探す。
	// 1以上なら、見つけた経路の次の列車（到着時刻の指定では前の列車）でも探し直し、Count 本まで集める
	Count int
}

// Delay は、路線・方向の列車がどれだけ遅れているか。
type Delay struct {
	Railway       string
	RailDirection string
	Minutes       int
}

// Journey は経路1本。Departure と Arrival は、出発駅から乗る駅まで・降りる駅から到着駅まで
// 歩く時間を含める（Legs の時刻は列車の時刻）。
type Journey struct {
	Departure int
	Arrival   int
	Legs      []Leg
}

// Transfers は乗り換えの回数を返す。直通運転で乗り続ける区間（Leg.Through）は数えない。
func (j Journey) Transfers() int {
	n := -1
	for _, l := range j.Legs {
		if !l.Through {
			n++
		}
	}
	return n
}

// Leg は1本の列車に乗る区間。
type Leg struct {
	Railway       string
	RailDirection string
	Train         string
	TrainNumber   string
	TrainType     string
	Destination   string

	From      string
	To        string
	Departure int
	Arrival   int

	// Delay は、乗る駅での発車時刻の遅れ（分）。Departure と Arrival は遅れを足した時刻
	Delay int

	// Through は、前の区間の列車から直通運転で乗り続ける区間か（乗り換えではない）。
	// 前の区間の To と、この区間の From は、境目の駅の事業者ごとの駅になる
	Through bool
}

type Engine struct {
	tt  *slim.TrainTimetables
	cfg Config

	// 駅の通し番号。stationOf は Strings の番号 → 駅の番号（駅でなければ -1）
	stationIDs []string
	stations   map[string]int32
	stationOf  []int32

	transfers []Transfer

	// 直通運転で1本の列車として走る列車の並び（throughChains）と、そのどれかに入っている列車
	chains  [][]int32
	inChain map[int32]bool

	mu       sync.Mutex
	networks map[string]*networkPair
}

type networkPair struct {
	forward  *network
	backward *network
}

// New は列車時刻表と乗り換えの対応表から探索エンジンを作る。
// 路線網はダイヤ種別の組み合わせごとに、初めて探索するときに作る。
func New(tt *slim.TrainTimetables, transfers []Transfer, cfg Config) *Engine {

	e := &Engine{
		tt:        tt,
		cfg:       cfg,
		stations:  make(map[string]int32),
		stationOf: make([]int32, len(tt.Strings)),
		transfers: transfers,
		networks:  make(map[string]*networkPair),
	}

	for i := range e.stationOf {
		e.stationOf[i] = -1
	}

	for _, train := range tt.Trains {
		for _, stop := range train.Stops {
			e.station(tt.String(stop.Station))
			e.stationOf[stop.Station] = e.stations[tt.String(stop.Station)]
		}
	}

	e.chains = throughChains(tt, transfers)
	e.inChain = make(map[int32]bool)
	for _, chain := range e.chains {
		for _, t := range chain {
			e.inChain[t] = true
		}
	}

	return e
}

// station は駅の番号を返す。まだ無ければ割り当てる。
func (e *Engine) station(id string) int32 {
	if i, ok := e.stations[id]; ok {
		return i
	}

	i := int32(len(e.stationIDs))
	e.stationIDs = append(e.stationIDs, id)
	e.stations[id] = i

	return i
}

// HasStation は、時刻表に駅 id に停車する列車があるかを返す。
func (e *Engine) HasStation(id string) bool {
	_, ok := e.stations[id]
	return ok
}

func (e *Engine) networkPair(calendars []string) *networkPair {

	key := slices.Clone(calendars)
	slices.Sort(key)
	k := strings.Join(key, ",")

	e.mu.Lock()
	defer e.mu.Unlock()

	if p, ok := e.networks[k]; ok {
		return p
	}

	p := e.buildNetworks(calendars)
	e.networks[k] = p

	return p
}

// Search は条件に合う経路を返す。
//
// 1回の探索では、乗り換え回数ごとに最も早く着く経路を、乗り換えの少ない順に返す。
// 乗り換えが多い経路は、少ない経路より早く着く（到着時刻の指定では遅く出る）ものだけを返す。
// Query.Count が1以上なら、次の列車（到着時刻の指定では前の列車）でも探し直して Count 本まで足し、
// 早く着く順（到着時刻の指定では遅く出る順）に並べる。最初の探索の経路は Count を超えても残す。
// 経路が無い場合は空を返す。
func (e *Engine) Search(q Query) ([]Journey, error) {

	from, err := e.stationGroup(q.From)
	if err != nil {
		return nil, err
	}

	to, err := e.stationGroup(q.To)
	if err != nil {
		return nil, err
	}

	for _, s := range from {
		if slices.Contains(to, s) {
			return nil, ErrSameStation
		}
	}

	maxTransfers := min(max(q.MaxTransfers, 0), e.cfg.MaxTransfers)

	nets := e.networkPair(q.Calendars)

	c := e.conditions(q)

	first := e.searchAt(nets, from, to, q.Time, q.ArriveBy, maxTransfers, c)
	if q.Count <= 0 || len(first) == 0 {
		return first, nil
	}

	better := func(a, b Journey) int {
		if q.ArriveBy {
			return cmp.Or(cmp.Compare(b.Departure, a.Departure), cmp.Compare(b.Arrival, a.Arrival), cmp.Compare(a.Transfers(), b.Transfers()))
		}
		return cmp.Or(cmp.Compare(a.Arrival, b.Arrival), cmp.Compare(b.Departure, a.Departure), cmp.Compare(a.Transfers(), b.Transfers()))
	}

	// 見つけた経路は全部集め、ほかの経路より良いところの無いものを除いてから数える
	all := first
	last := first
	t := q.Time

	// 同じ経路しか見つからないときに探し続けないよう、回数に上限を設ける
	for range maxRepeats {
		if len(removeDominated(all)) >= q.Count {
			break
		}

		t = nextTime(last, t, q.ArriveBy)

		found := e.searchAt(nets, from, to, t, q.ArriveBy, maxTransfers, c)
		if len(found) == 0 {
			break
		}
		last = found
		for _, j := range found {
			if !slices.ContainsFunc(all, func(r Journey) bool { return sameJourney(r, j) }) {
				all = append(all, j)
			}
		}
	}

	candidates := removeDominated(all)
	slices.SortStableFunc(candidates, better)

	// 最初の探索の経路は残し、ほかは良い順に Count 本まで足す
	isFirst := func(j Journey) bool {
		return slices.ContainsFunc(first, func(f Journey) bool { return sameJourney(f, j) })
	}
	extra := q.Count - len(slices.DeleteFunc(slices.Clone(candidates), func(j Journey) bool { return !isFirst(j) }))

	result := make([]Journey, 0, len(candidates))
	for _, j := range candidates {
		switch {
		case isFirst(j):
			result = append(result, j)
		case extra > 0:
			result = append(result, j)
			extra--
		}
	}

	return result, nil
}

// maxRepeats は、Query.Count の経路を集めるために探し直す回数の上限
const maxRepeats = 6

// searchAt は時刻 t で1回探す（Search の1回分）。
func (e *Engine) searchAt(
	nets *networkPair,
	from, to []int32,
	t int,
	arriveBy bool,
	maxTransfers int,
	c *conditions,
) []Journey {

	if arriveBy {
		// 最も遅く出る経路を逆向きに探し、出発時刻から最も早く着く経路に絞る
		latest := nets.backward.search(e, to, from, -t, maxTransfers, c)

		result := make([]Journey, 0, len(latest))
		for _, j := range latest {
			best := nets.forward.search(e, from, to, j.Departure, j.Transfers(), c)
			result = append(result, pick(best, j))
		}

		return result
	}

	earliest := nets.forward.search(e, from, to, t, maxTransfers, c)

	// 同じ時刻に着く経路のうち、最も遅く出るものに絞る（乗り換え駅で長く待たないように）
	result := make([]Journey, 0, len(earliest))
	for _, j := range earliest {
		latest := nets.backward.search(e, to, from, -j.Arrival, j.Transfers(), c)
		result = append(result, pick(latest, j))
	}

	return result
}

// nextTime は、次に探す時刻を返す。出発時刻の指定では、前回の探索で見つけた経路の最も早い出発の1分後
// （その列車より後に出る経路を探す）。到着時刻の指定では、最も遅い到着の1分前。
// 前に探した時刻 prev から必ず1分以上進める。journeys は空でないこと。
func nextTime(journeys []Journey, prev int, arriveBy bool) int {

	if arriveBy {
		latest := journeys[0].Arrival
		for _, j := range journeys[1:] {
			latest = max(latest, j.Arrival)
		}
		return min(latest, prev) - 1
	}

	earliest := journeys[0].Departure
	for _, j := range journeys[1:] {
		earliest = min(earliest, j.Departure)
	}
	return max(earliest, prev) + 1
}

// sameJourney は、2つの経路が同じ列車に同じ区間で乗るかを返す。
func sameJourney(a, b Journey) bool {
	return slices.EqualFunc(a.Legs, b.Legs, func(x, y Leg) bool {
		return x.Train == y.Train && x.From == y.From && x.To == y.To
	})
}

// removeDominated は、ほかの経路より良いところの無い経路を除く。
// 出発が遅くなく（到着時刻の指定では早くなく）、到着が早くなく、乗り換えも少なくない経路は、
// ほかの経路の方が良いので除く。
func removeDominated(journeys []Journey) []Journey {

	dominates := func(a, b Journey) bool {
		return a.Departure >= b.Departure && a.Arrival <= b.Arrival && a.Transfers() <= b.Transfers()
	}

	result := make([]Journey, 0, len(journeys))
	for i, j := range journeys {
		dominated := false
		for k, other := range journeys {
			if i == k || !dominates(other, j) {
				continue
			}
			// 条件がまったく同じなら、先に見つけた方を残す
			if other.Departure == j.Departure && other.Arrival == j.Arrival && other.Transfers() == j.Transfers() && k > i {
				continue
			}
			dominated = true
			break
		}
		if !dominated {
			result = append(result, j)
		}
	}

	return result
}

// conditions は、探索のあいだ変わらない条件（使わない路線と遅れ）。路線・方向は Strings の番号で持つ。
type conditions struct {
	avoid  map[int32]bool
	delays map[[2]int32]int32
	until  int32

	// shifts の結果を入れる領域（パターンを調べるたびに割り当てないよう使い回す）
	buf []shift
}

func (e *Engine) conditions(q Query) *conditions {

	c := &conditions{
		avoid:  make(map[int32]bool),
		delays: make(map[[2]int32]int32),
		until:  int32(q.DelayUntil),
	}

	index := make(map[string]int32, len(e.tt.Strings))
	for i, s := range e.tt.Strings {
		index[s] = int32(i)
	}

	for _, id := range q.Avoid {
		if i, ok := index[id]; ok {
			c.avoid[i] = true
		}
	}

	for _, d := range q.Delays {
		railway, ok := index[d.Railway]
		if !ok || d.Minutes <= 0 {
			continue
		}
		direction, ok := index[d.RailDirection]
		if !ok {
			continue
		}
		c.delays[[2]int32{railway, direction}] = int32(d.Minutes)
	}

	return c
}

// shifts は、パターンの列車の時刻に足す遅れを区間ごとに返す。
// 直通運転の列車は、前の区間の遅れを持ち越すとみなし、それまでの区間の遅れの最大を足す
// （区間ごとに違う遅れを足すと、境目で時刻が逆戻りすることがあるため）。
// 返す領域は次の呼び出しで上書きされる。
func (c *conditions) shifts(n *network, p *pattern) []shift {
	if cap(c.buf) < len(p.railways) {
		c.buf = make([]shift, len(p.railways))
	}
	result := c.buf[:len(p.railways)]
	var minutes int32
	for s := range p.railways {
		minutes = max(minutes, c.delays[[2]int32{p.railways[s], p.directions[s]}])
		result[s] = shift{
			minutes:  minutes,
			until:    c.until,
			reversed: n.reversed,
		}
	}
	return result
}

// avoided は、パターンの区間 s の路線を使わないかを返す。
func (c *conditions) avoided(p *pattern, s int) bool {
	return c.avoid[p.railways[s]]
}

// shift は時刻表の時刻を、遅れを足した時刻にする。
// 時刻の前後を入れ替えない（単調に増える）ので、列車の順番や追い越しの判定はそのまま使える。
type shift struct {
	minutes  int32
	until    int32
	reversed bool
}

func (s shift) apply(t int32) int32 {
	if s.minutes == 0 {
		return t
	}
	// 逆向きの路線網では時刻に -1 を掛けているので、元の向きに戻してから足す
	if s.reversed {
		return -s.forward(-t)
	}
	return s.forward(t)
}

func (s shift) forward(t int32) int32 {
	if t <= s.until {
		return t + s.minutes
	}
	return max(t, s.until+s.minutes)
}

func (e *Engine) stationGroup(ids []string) ([]int32, error) {

	if len(ids) == 0 {
		return nil, ErrUnknownStation
	}

	result := make([]int32, 0, len(ids))

	for _, id := range ids {
		s, ok := e.stations[id]
		if !ok {
			return nil, ErrUnknownStation
		}
		result = append(result, s)
	}

	return result, nil
}

// pick は、絞り込みの探索結果のうち乗り換え回数が最も多い（＝条件が最も良い）経路を返す。
// 見つからない場合は元の経路を返す。
func pick(candidates []Journey, original Journey) Journey {
	if len(candidates) == 0 {
		return original
	}
	return candidates[len(candidates)-1]
}
