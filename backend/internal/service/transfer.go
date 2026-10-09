package service

import (
	"cmp"
	"math"
	"slices"

	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

const (
	// sameStationMeters は、同じ名前の駅を1つの駅として扱う距離の上限。
	// 同じ名前でも離れている駅（早稲田: 東西線 ⇔ 荒川線 733m、浅草: TX ⇔ 都営・メトロ 600〜680m）は分ける
	sameStationMeters = 600

	// 乗り換えにかかる時間は、駅の座標の直線距離から transferBaseMinutes + 距離 ÷ walkMetersPerMinute（切り上げ）とする。
	// 直線距離は実際の通路より短いので、歩く速さを遅めにしている（docs/design/multi-operator.md 6.2）
	transferBaseMinutes = 3
	walkMetersPerMinute = 80

	// defaultTransferMinutes は、駅の座標が無いときの乗り換えの時間
	defaultTransferMinutes = 5
)

// differentNameTransfers は、名前が違うが乗り換えられる都営の駅の組。
// 他社の駅は駅データの odpt:connectingStation から作る（都営の駅データにはこの項目が無い）。
var differentNameTransfers = [][2]string{
	{"odpt.Station:Toei.Asakusa.HigashiNihombashi", "odpt.Station:Toei.Shinjuku.BakuroYokoyama"},
}

// stationGroups は、駅ID → 経路検索の出発駅・到着駅としてまとめる駅の ID（ID 順）を返す。
// まとめるのは、同じ名前で sameStationMeters 以内にある駅のうち、経路探索の対象の駅（inNetwork）。
// 自分が対象でなければ自分は含めない（列車時刻表の無い東急の渋谷を指定すると、ほかの社の渋谷から探す）。
func stationGroups(stations []model.Station, inNetwork func(string) bool) map[string][]string {

	byName := make(map[string][]model.Station)
	for _, st := range stations {
		byName[st.StationTitle.Ja] = append(byName[st.StationTitle.Ja], st)
	}

	result := make(map[string][]string, len(stations))

	for _, st := range stations {
		ids := []string{}
		for _, other := range byName[st.StationTitle.Ja] {
			if inNetwork(other.SameAs) && (other.SameAs == st.SameAs || sameStation(st, other)) {
				ids = append(ids, other.SameAs)
			}
		}
		slices.Sort(ids)
		result[st.SameAs] = slices.Compact(ids)
	}

	return result
}

// sameStation は、同じ名前の2つの駅を1つの駅として扱うかを返す。座標の無い駅はまとめる。
func sameStation(a, b model.Station) bool {
	d, ok := distance(a, b)
	return !ok || d <= sameStationMeters
}

// transfers は乗り換えの対応表を作る（両方向）。
//   - 同じ名前で sameStationMeters 以内の駅どうし
//   - 駅データの odpt:connectingStation（都営の駅への乗り換えは逆向きにも足す）
//   - differentNameTransfers
//
// 時間は駅の座標の距離から決める（transferMinutes）。
func transfers(stations []model.Station) []route.Transfer {

	byID := make(map[string]model.Station, len(stations))
	for _, st := range stations {
		byID[st.SameAs] = st
	}

	type pair struct{ from, to string }
	pairs := make(map[pair]bool)

	add := func(a, b string) {
		_, okA := byID[a]
		_, okB := byID[b]
		if a == b || !okA || !okB {
			return
		}
		pairs[pair{a, b}] = true
		pairs[pair{b, a}] = true
	}

	byName := make(map[string][]model.Station)
	for _, st := range stations {
		byName[st.StationTitle.Ja] = append(byName[st.StationTitle.Ja], st)
	}

	for _, group := range byName {
		for i, a := range group {
			for _, b := range group[i+1:] {
				if sameStation(a, b) {
					add(a.SameAs, b.SameAs)
				}
			}
		}
	}

	for _, st := range stations {
		for _, to := range st.ConnectingStation {
			add(st.SameAs, to)
		}
	}

	for _, p := range differentNameTransfers {
		add(p[0], p[1])
	}

	result := make([]route.Transfer, 0, len(pairs))
	for p := range pairs {
		result = append(result, route.Transfer{
			From:    p.from,
			To:      p.to,
			Minutes: transferMinutes(byID[p.from], byID[p.to]),
		})
	}

	slices.SortFunc(result, func(a, b route.Transfer) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})

	return result
}

// transferMinutes は、駅 a から駅 b に乗り換えるのにかかる時間（歩く時間＋余裕）。
func transferMinutes(a, b model.Station) int {
	d, ok := distance(a, b)
	if !ok {
		return defaultTransferMinutes
	}
	return transferBaseMinutes + int(math.Ceil(d/walkMetersPerMinute))
}

// distance は2つの駅の直線距離（m）を返す。どちらかの座標が無ければ ok は false。
// 都内の短い距離なので、正距円筒図法の近似で求める。
func distance(a, b model.Station) (float64, bool) {

	if (a.Latitude == 0 && a.Longitude == 0) || (b.Latitude == 0 && b.Longitude == 0) {
		return 0, false
	}

	const earthRadius = 6_371_000.0

	lat1, lat2 := a.Latitude*math.Pi/180, b.Latitude*math.Pi/180
	dLat := lat2 - lat1
	dLong := (b.Longitude - a.Longitude) * math.Pi / 180 * math.Cos((lat1+lat2)/2)

	return earthRadius * math.Hypot(dLat, dLong), true
}
