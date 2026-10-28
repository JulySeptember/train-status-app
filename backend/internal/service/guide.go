package service

import (
	"math"

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
