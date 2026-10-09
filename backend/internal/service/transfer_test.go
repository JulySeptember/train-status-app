package service

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

// testStation は、名前と座標を持つ架空の駅を返す。
func testStation(id, name string, lat, long float64, connecting ...string) model.Station {
	return model.Station{
		SameAs:            id,
		StationTitle:      model.LocalizedString{Ja: name},
		Latitude:          lat,
		Longitude:         long,
		ConnectingStation: connecting,
	}
}

// 緯度 0.001 度は約 111m
const metersPerMilliDegree = 111.2

func TestStationGroups(t *testing.T) {

	stations := []model.Station{
		testStation("A.Shibuya", "渋谷", 35.658, 139.701),
		testStation("B.Shibuya", "渋谷", 35.659, 139.701),  // 約110m
		testStation("C.Shibuya", "渋谷", 35.6585, 139.701), // 列車時刻表が無い（経路探索の対象外）
		testStation("D.Waseda", "早稲田", 35.705, 139.720),
		testStation("E.Waseda", "早稲田", 35.7116, 139.720), // 約730m
		testStation("F.NoGeo", "座標なし", 0, 0),
		testStation("G.NoGeo", "座標なし", 35.7, 139.7),
	}

	inNetwork := func(id string) bool { return id != "C.Shibuya" }

	groups := stationGroups(stations, inNetwork)

	tests := map[string][]string{
		"A.Shibuya": {"A.Shibuya", "B.Shibuya"},
		// 自分が対象外でも、近くの同じ名前の駅から探せる
		"C.Shibuya": {"A.Shibuya", "B.Shibuya"},
		// 600m より離れた同じ名前の駅はまとめない
		"D.Waseda": {"D.Waseda"},
		"E.Waseda": {"E.Waseda"},
		// 座標が無ければまとめる
		"F.NoGeo": {"F.NoGeo", "G.NoGeo"},
	}

	for id, want := range tests {
		if got := groups[id]; !slices.Equal(got, want) {
			t.Errorf("%s: expected %v, got %v", id, want, got)
		}
	}

	// 対象外で、近くに同じ名前の駅も無い駅は空になる（経路検索できない）
	groups = stationGroups([]model.Station{testStation("C.Only", "単独", 35, 139)}, func(string) bool { return false })
	if got, ok := groups["C.Only"]; !ok || len(got) != 0 {
		t.Errorf("expected an empty group, got %v %v", got, ok)
	}
}

func TestTransfers(t *testing.T) {

	stations := []model.Station{
		// 都営の駅には odpt:connectingStation が無い
		testStation("odpt.Station:Toei.Mita.Kasuga", "春日", 35.7090, 139.7530),
		testStation("odpt.Station:Toei.Oedo.Kasuga", "春日", 35.7092, 139.7531),
		// 名前の違う乗り換え駅（片方からだけ書かれている）
		testStation("odpt.Station:TokyoMetro.Marunouchi.Korakuen", "後楽園", 35.7072, 139.7520,
			"odpt.Station:Toei.Mita.Kasuga",
			"odpt.Station:Keisei.Unknown.Station", // データに無い駅は捨てる
		),
		// 同じ名前でも離れている駅は、odpt:connectingStation に無ければ乗り換えにしない
		testStation("odpt.Station:TokyoMetro.Tozai.Waseda", "早稲田", 35.705, 139.720),
		testStation("odpt.Station:Toei.Arakawa.Waseda", "早稲田", 35.7116, 139.720),
		testStation("odpt.Station:Toei.Asakusa.HigashiNihombashi", "東日本橋", 35.6925, 139.7849),
		testStation("odpt.Station:Toei.Shinjuku.BakuroYokoyama", "馬喰横山", 35.6923, 139.7830),
	}

	got := transfers(stations)

	minutes := make(map[[2]string]int)
	for _, tr := range got {
		key := [2]string{tr.From, tr.To}
		if _, dup := minutes[key]; dup {
			t.Errorf("duplicate transfer %v", key)
		}
		minutes[key] = tr.Minutes
	}

	want := [][2]string{
		{"odpt.Station:Toei.Mita.Kasuga", "odpt.Station:Toei.Oedo.Kasuga"},
		{"odpt.Station:Toei.Oedo.Kasuga", "odpt.Station:Toei.Mita.Kasuga"},
		{"odpt.Station:TokyoMetro.Marunouchi.Korakuen", "odpt.Station:Toei.Mita.Kasuga"},
		// 逆向き（都営 → 他社）も足す
		{"odpt.Station:Toei.Mita.Kasuga", "odpt.Station:TokyoMetro.Marunouchi.Korakuen"},
		{"odpt.Station:Toei.Asakusa.HigashiNihombashi", "odpt.Station:Toei.Shinjuku.BakuroYokoyama"},
		{"odpt.Station:Toei.Shinjuku.BakuroYokoyama", "odpt.Station:Toei.Asakusa.HigashiNihombashi"},
	}

	if len(got) != len(want) {
		t.Errorf("expected %d transfers, got %+v", len(want), got)
	}

	for _, key := range want {
		if _, ok := minutes[key]; !ok {
			t.Errorf("missing transfer %v", key)
		}
	}

	// 時間は距離から: 春日どうし（約24m）は 3+1 分、後楽園 ⇔ 春日（約220m）は 3+3 分
	if m := minutes[want[0]]; m != 4 {
		t.Errorf("Kasuga: expected 4 minutes, got %d", m)
	}
	if m := minutes[want[2]]; m != 6 {
		t.Errorf("Korakuen: expected 6 minutes, got %d", m)
	}

	// 並びは決まった順（探索の結果が実行ごとに変わらないように）
	if !slices.IsSortedFunc(got, func(a, b route.Transfer) int {
		return cmp.Or(strings.Compare(a.From, b.From), strings.Compare(a.To, b.To))
	}) {
		t.Error("transfers are not sorted")
	}
}

func TestTransferMinutes(t *testing.T) {

	a := testStation("a", "a", 35.0, 139.0)

	tests := []struct {
		meters float64
		want   int
	}{
		{0, 3},
		{80, 4},
		{400, 8},
		{641, 12},
	}

	for _, tt := range tests {
		b := testStation("b", "b", 35.0+tt.meters/metersPerMilliDegree/1000, 139.0)
		if got := transferMinutes(a, b); got != tt.want {
			t.Errorf("%.0fm: expected %d, got %d", tt.meters, tt.want, got)
		}
	}

	// 座標が無ければ一律
	if got := transferMinutes(a, testStation("c", "c", 0, 0)); got != defaultTransferMinutes {
		t.Errorf("no coordinates: expected %d, got %d", defaultTransferMinutes, got)
	}
}
