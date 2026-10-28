package ai

import (
	"testing"

	"train-status-app/backend/internal/service"
)

func TestGuideText(t *testing.T) {

	tests := map[string]struct {
		guide *service.AlightGuide
		want  string
	}{
		"何両目と設備": {
			guide: &service.AlightGuide{CarFromFront: 10, Cars: 12, Via: "stairs", Target: "中央西改札", WalkMeters: 114, WalkMinutes: 2},
			want:  "前から10両目付近（12両編成） → 階段 → 中央西改札（約114m・徒歩2分）",
		},
		"行き先だけ": {
			guide: &service.AlightGuide{Target: "中央線快速 11・12番線", WalkMeters: 92, WalkMinutes: 2},
			want:  "中央線快速 11・12番線（約92m・徒歩2分）",
		},
		"降りて右へ": {
			guide: &service.AlightGuide{CarFromFront: 1, Cars: 11, Toward: "front", TowardMeters: 11, Turn: "right", Via: "stairs", Target: "南改札", WalkMeters: 41, WalkMinutes: 1},
			want:  "前から1両目付近（11両編成） → 降りて右（進行方向）へ約11m → 階段 → 南改札（約41m・徒歩1分）",
		},
		"右左が分からない": {
			guide: &service.AlightGuide{CarFromFront: 4, Cars: 12, Toward: "back", TowardMeters: 20, Via: "escalator", Target: "南改札", WalkMeters: 84, WalkMinutes: 2},
			want:  "前から4両目付近（12両編成） → 降りて進行方向と逆へ約20m → エスカレーター → 南改札（約84m・徒歩2分）",
		},
		"ホームを出る場所": {
			guide: &service.AlightGuide{ExitAt: "back", Via: "stairs", Target: "南改札", WalkMeters: 117, WalkMinutes: 2},
			want:  "ホームの後ろ寄りの階段 → 南改札（約117m・徒歩2分）",
		},
		"案内なし": {},
	}

	for name, tt := range tests {
		if got := guideText(tt.guide); got != tt.want {
			t.Errorf("%s: expected %q, got %q", name, tt.want, got)
		}
	}
}

func TestTrimJourneysGuide(t *testing.T) {

	journeys := trimJourneys([]service.Journey{{
		DepartureTime: "10:00",
		ArrivalTime:   "10:20",
		Legs: []service.JourneyLeg{
			{DepartureTime: "10:00", ArrivalTime: "10:10", AlightGuide: &service.AlightGuide{Target: "南改札", WalkMeters: 60, WalkMinutes: 1}},
			{DepartureTime: "10:15", ArrivalTime: "10:20"},
		},
	}})

	legs := journeys[0].Legs
	if legs[0].AlightGuide != "南改札（約60m・徒歩1分）" {
		t.Errorf("unexpected guide %q", legs[0].AlightGuide)
	}
	if legs[1].AlightGuide != "" {
		t.Errorf("expected no guide, got %q", legs[1].AlightGuide)
	}
}
