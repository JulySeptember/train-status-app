package service

import (
	"math"
	"slices"

	"train-status-app/backend/assets/guide"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

// 駅の中の案内（docs/design/station-guide.md 8章）。
// 経路の区間で降りる駅が案内のある駅なら、次の区間に乗るホーム（同じ駅の JR の別のホーム）か、
// 他社へ乗り換えるときに使う改札までの案内を付ける。

// maxTransferGateMeters は、対応表に無い他社の駅へ乗り換えるとき、最も近い改札を使う距離の上限
const maxTransferGateMeters = 1000

type guideIndex struct {
	// 駅ID → その駅の案内
	stations map[string]*guideStation

	// 駅ID → 緯度・経度だけの駅（対応表に無い乗り換え先の駅に近い改札を選ぶ）
	coords map[string]model.Station

	// 路線ID → 路線名（行き先のホームの名前）
	railwayNames map[string]string
}

type guideStation struct {
	station *guide.Station
	entries map[guideKey]guide.Entry
}

type guideKey struct {
	railway, direction string

	// 行き先。ホームなら路線・方向、改札なら名前
	toRailway, toDirection string
	gate                   string
}

func newGuideIndex(t *guide.Table, stations []model.Station, railwayNames map[string]string) guideIndex {

	idx := guideIndex{
		stations:     map[string]*guideStation{},
		coords:       map[string]model.Station{},
		railwayNames: railwayNames,
	}

	if t == nil {
		return idx
	}

	for _, st := range stations {
		idx.coords[st.SameAs] = model.Station{Latitude: st.Latitude, Longitude: st.Longitude}
	}

	for i := range t.Stations {
		s := &t.Stations[i]

		g := &guideStation{station: s, entries: map[guideKey]guide.Entry{}}
		for _, e := range s.Entries {
			g.entries[guideKey{
				railway:     e.Railway,
				direction:   e.Direction,
				toRailway:   e.Target.Railway,
				toDirection: e.Target.Direction,
				gate:        e.Target.Gate,
			}] = e
		}

		for _, id := range s.StationIDs {
			idx.stations[id] = g
		}
	}

	return idx
}

// alight は、legs[i] で降りる駅の案内を返す。案内が無ければ nil。
// 経路の最後の区間（行き先の出口が分からない）・次の区間へ直通運転で乗り続けるとき・
// 同じホームで乗り継ぐとき（表に無い）は案内しない。
func (idx guideIndex) alight(legs []route.Leg, i int) *AlightGuide {

	if i+1 >= len(legs) {
		return nil
	}

	cur, next := legs[i], legs[i+1]
	if next.Through {
		return nil
	}

	g, ok := idx.stations[cur.To]
	if !ok {
		return nil
	}

	key := guideKey{railway: cur.Railway, direction: cur.RailDirection}

	if idx.stations[next.From] == g {
		key.toRailway, key.toDirection = next.Railway, next.RailDirection
	} else {
		key.gate = idx.transferGate(g, next.From)
		if key.gate == "" {
			return nil
		}
	}

	e, ok := g.entries[key]
	if !ok {
		return nil
	}

	result := &AlightGuide{
		CarFromFront: e.CarFromFront,
		Cars:         e.Cars,
		Via:          e.Via,
		Toward:       e.Toward,
		TowardMeters: e.TowardMeters,
		Turn:         e.Turn,
		ExitAt:       e.ExitAt,
		Target:       e.Target.Gate,
		WalkMeters:   e.WalkMeters,
		WalkMinutes:  e.WalkMinutes,
	}
	if e.Target.Gate == "" {
		result.Target = idx.railwayNames[e.Target.Railway] + " " + e.Target.Track + "番線"
	}

	return result
}

// transferGate は、案内のある駅 g から他社の駅 to へ乗り換えるときに使う改札の名前を返す。
// 対応表に無ければ、to に最も近い改札（maxTransferGateMeters 以内）。
func (idx guideIndex) transferGate(g *guideStation, to string) string {

	if gate, ok := g.station.Transfers[to]; ok {
		return gate
	}

	st, ok := idx.coords[to]
	if !ok {
		return ""
	}

	best, bestD := "", math.Inf(1)
	for _, gate := range g.station.Gates {
		d, ok := distance(st, model.Station{Latitude: gate.Lat, Longitude: gate.Lon})
		if ok && d < bestD {
			best, bestD = gate.Name, d
		}
	}

	if bestD > maxTransferGateMeters {
		return ""
	}

	return best
}

// guideTransferMarginMinutes は、案内の歩く時間に足す余裕（列車を降りる・人の流れに合わせて歩くなど）
const guideTransferMarginMinutes = 2

// withGuideTransfers は、乗り換えの対応表のうち、案内のある駅の中で JR の別の路線の駅へ乗り換える組の時間を、
// 案内の歩く時間に guideTransferMarginMinutes を足した時間に置き換える。駅の座標の直線距離から決めた時間
// （transferMinutes）より、通路の長さに合う。対応表は駅の組ごとなので、降りる列車・乗る列車の向きのうち
// 最も長い時間を使う（向きによる違いは6駅で1分以内）。他社への乗り換え（改札までしか分からない）と、
// 同じ駅で乗り継ぐ組（対応表に無い）は変えない。
func withGuideTransfers(ts []route.Transfer, t *guide.Table, stations []model.Station) []route.Transfer {

	if t == nil {
		return ts
	}

	railways := make(map[string]string, len(stations))
	for _, st := range stations {
		railways[st.SameAs] = st.Railway
	}

	type pair struct{ from, to string }
	walks := map[pair]int{}

	for _, s := range t.Stations {

		// 路線 → この案内を使う駅
		byRailway := map[string]string{}
		for _, id := range s.StationIDs {
			if r, ok := railways[id]; ok {
				byRailway[r] = id
			}
		}

		for _, e := range s.Entries {
			if e.Target.Railway == "" {
				continue
			}
			from, to := byRailway[e.Railway], byRailway[e.Target.Railway]
			if from == "" || to == "" || from == to {
				continue
			}
			p := pair{from, to}
			walks[p] = max(walks[p], e.WalkMinutes)
		}
	}

	result := slices.Clone(ts)
	for i, tr := range result {
		if w, ok := walks[pair{tr.From, tr.To}]; ok {
			result[i].Minutes = w + guideTransferMarginMinutes
		}
	}

	return result
}
