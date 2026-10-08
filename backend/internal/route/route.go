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
	// 同じ名前で路線ごとに駅が分かれている駅（新宿など）を、まとめて指定するのに使う
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
}

// Delay は、路線・方向の列車がどれだけ遅れているか。
type Delay struct {
	Railway       string
	RailDirection string
	Minutes       int
}

// Journey は経路1本。
type Journey struct {
	Departure int
	Arrival   int
	Legs      []Leg
}

func (j Journey) Transfers() int {
	return len(j.Legs) - 1
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
}

type Engine struct {
	tt  *slim.TrainTimetables
	cfg Config

	// 駅の通し番号。stationOf は Strings の番号 → 駅の番号（駅でなければ -1）
	stationIDs []string
	stations   map[string]int32
	stationOf  []int32

	transfers []Transfer

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

// Search は条件に合う経路を、乗り換えの少ない順に返す。
// 乗り換えが多い経路は、少ない経路より早く着く（到着時刻の指定では遅く出る）ものだけを返す。
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

	if q.ArriveBy {
		// 最も遅く出る経路を逆向きに探し、出発時刻から最も早く着く経路に絞る
		latest := nets.backward.search(e, to, from, -q.Time, maxTransfers, c)

		result := make([]Journey, 0, len(latest))
		for _, j := range latest {
			best := nets.forward.search(e, from, to, j.Departure, j.Transfers(), c)
			result = append(result, pick(best, j))
		}

		return result, nil
	}

	earliest := nets.forward.search(e, from, to, q.Time, maxTransfers, c)

	// 同じ時刻に着く経路のうち、最も遅く出るものに絞る（乗り換え駅で長く待たないように）
	result := make([]Journey, 0, len(earliest))
	for _, j := range earliest {
		latest := nets.backward.search(e, to, from, -j.Arrival, j.Transfers(), c)
		result = append(result, pick(latest, j))
	}

	return result, nil
}

// conditions は、探索のあいだ変わらない条件（使わない路線と遅れ）。路線・方向は Strings の番号で持つ。
type conditions struct {
	avoid  map[int32]bool
	delays map[[2]int32]int32
	until  int32
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

// shift は、パターンの列車の時刻に足す遅れ。
func (c *conditions) shift(n *network, p *pattern) shift {
	return shift{
		minutes:  c.delays[[2]int32{p.railway, p.direction}],
		until:    c.until,
		reversed: n.reversed,
	}
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
