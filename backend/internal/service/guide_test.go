package service

import (
	"testing"

	"train-status-app/backend/assets/guide"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
)

func TestGuideAlight(t *testing.T) {

	table := &guide.Table{Stations: []guide.Station{{
		StationIDs: []string{"JR.Yamanote.Shinjuku", "JR.Chuo.Shinjuku"},
		Transfers:  map[string]string{"Odakyu.Shinjuku": "小田急口"},
		Gates: []guide.Gate{
			{Name: "東口", Lat: 35.691, Lon: 139.701},
			{Name: "南口", Lat: 35.688, Lon: 139.700},
			{Name: "小田急口", Lat: 35.689, Lon: 139.699},
		},
		Entries: []guide.Entry{
			{
				Railway: "JR.Yamanote", Direction: "Inner",
				Target:       guide.Target{Railway: "JR.Chuo", Direction: "Out", Track: "11・12"},
				CarFromFront: 3, Cars: 11, Via: guide.ViaStairs, WalkMeters: 120, WalkMinutes: 2,
				Toward: guide.TowardFront, TowardMeters: 30, Turn: guide.TurnRight,
			},
			{
				Railway: "JR.Yamanote", Direction: "Inner",
				Target: guide.Target{Gate: "小田急口"}, Via: guide.ViaEscalator, WalkMeters: 90, WalkMinutes: 2,
				ExitAt: guide.ExitBack,
			},
			{
				Railway: "JR.Yamanote", Direction: "Inner",
				Target:       guide.Target{Gate: "南口"},
				CarFromFront: 8, Cars: 11, WalkMeters: 60, WalkMinutes: 1,
			},
		},
	}}}

	stations := []model.Station{
		// 南口の約110m 南
		testStation("Toei.Shinjuku", "新宿", 35.687, 139.700),
		// どの改札からも 1km より遠い
		testStation("Far.Station", "遠い駅", 35.70, 139.72),
	}

	idx := newGuideIndex(table, stations, map[string]string{"JR.Chuo": "中央線快速"})

	ride := func(railway, direction, from, to string) route.Leg {
		return route.Leg{Railway: railway, RailDirection: direction, From: from, To: to}
	}
	yamanote := ride("JR.Yamanote", "Inner", "JR.Yamanote.Shibuya", "JR.Yamanote.Shinjuku")

	tests := map[string]struct {
		legs []route.Leg
		want *AlightGuide
	}{
		"JR の別のホーム": {
			legs: []route.Leg{yamanote, ride("JR.Chuo", "Out", "JR.Chuo.Shinjuku", "JR.Chuo.Nakano")},
			want: &AlightGuide{CarFromFront: 3, Cars: 11, Via: guide.ViaStairs, Toward: guide.TowardFront, TowardMeters: 30, Turn: guide.TurnRight, Target: "中央線快速 11・12番線", WalkMeters: 120, WalkMinutes: 2},
		},
		"対応表にある他社の駅": {
			legs: []route.Leg{yamanote, ride("Odakyu", "Down", "Odakyu.Shinjuku", "Odakyu.Yoyogi")},
			want: &AlightGuide{Via: guide.ViaEscalator, ExitAt: guide.ExitBack, Target: "小田急口", WalkMeters: 90, WalkMinutes: 2},
		},
		"対応表に無い他社の駅は最も近い改札": {
			legs: []route.Leg{yamanote, ride("Toei", "East", "Toei.Shinjuku", "Toei.Akebonobashi")},
			want: &AlightGuide{CarFromFront: 8, Cars: 11, Target: "南口", WalkMeters: 60, WalkMinutes: 1},
		},
		"近い改札が無い": {
			legs: []route.Leg{yamanote, ride("Far", "East", "Far.Station", "Far.Next")},
		},
		"座標の分からない駅": {
			legs: []route.Leg{yamanote, ride("Unknown", "East", "Unknown.Station", "Unknown.Next")},
		},
		"表に無い組み合わせ": {
			legs: []route.Leg{yamanote, ride("JR.Yamanote", "Outer", "JR.Yamanote.Shinjuku", "JR.Yamanote.Shibuya")},
		},
		"直通運転": {
			legs: []route.Leg{yamanote, {Railway: "JR.Chuo", RailDirection: "Out", From: "JR.Chuo.Shinjuku", To: "JR.Chuo.Nakano", Through: true}},
		},
		"最後の区間": {
			legs: []route.Leg{yamanote},
		},
		"案内の無い駅": {
			legs: []route.Leg{ride("JR.Yamanote", "Inner", "JR.Yamanote.Shibuya", "JR.Yamanote.Harajuku"), ride("X", "Y", "X.Harajuku", "X.Next")},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := idx.alight(tt.legs, 0)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("expected %+v, got %+v", tt.want, got)
			}
		})
	}

	t.Run("表が無い", func(t *testing.T) {
		idx := newGuideIndex(nil, stations, nil)
		if got := idx.alight([]route.Leg{yamanote, ride("Odakyu", "Down", "Odakyu.Shinjuku", "Odakyu.Yoyogi")}, 0); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}
