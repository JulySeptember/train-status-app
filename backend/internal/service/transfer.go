package service

import (
	"maps"
	"slices"

	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

// transferMinutes は、別の路線の駅に乗り換えるのにかかる時間（歩く時間＋余裕）。
// データに無いので一律にしている。通路の長い駅（新宿・春日など）で足りなければ、
// 駅ごとの時間を対応表に持たせる。
const transferMinutes = 5

// differentNameTransfers は、名前が違うが乗り換えられる駅の組（片方向ずつ書く）。
// 同じ名前の駅（新宿・大門・春日など）は sameNameStations から作るので、ここには書かない。
var differentNameTransfers = [][2]string{
	{"odpt.Station:Toei.Asakusa.HigashiNihombashi", "odpt.Station:Toei.Shinjuku.BakuroYokoyama"},
	{"odpt.Station:Toei.Shinjuku.BakuroYokoyama", "odpt.Station:Toei.Asakusa.HigashiNihombashi"},
}

// sameNameStations は、駅ID → 同じ名前の駅の ID（自分を含む。ID 順）を返す。
// 路線ごとに分かれている同じ名前の駅は、経路検索の出発駅・到着駅としてまとめて扱う。
func sameNameStations(stations []model.Station) map[string][]string {

	byName := make(map[string][]string)

	for _, st := range stations {
		name := st.StationTitle.Ja
		byName[name] = append(byName[name], st.SameAs)
	}

	result := make(map[string][]string, len(stations))

	for _, ids := range byName {
		slices.Sort(ids)
		for _, id := range ids {
			result[id] = ids
		}
	}

	return result
}

// transfers は乗り換えの対応表を作る。同じ名前の駅どうしと、differentNameTransfers。
func transfers(groups map[string][]string, minutes int) []route.Transfer {

	var result []route.Transfer

	for _, from := range slices.Sorted(maps.Keys(groups)) {
		for _, to := range groups[from] {
			if from != to {
				result = append(result, route.Transfer{
					From:    from,
					To:      to,
					Minutes: minutes,
				})
			}
		}
	}

	for _, pair := range differentNameTransfers {
		result = append(result, route.Transfer{
			From:    pair[0],
			To:      pair[1],
			Minutes: minutes,
		})
	}

	return result
}
