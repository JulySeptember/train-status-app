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
		"案内なし": {},
	}

	for name, tt := range tests {
		if got := guideText(tt.guide); got != tt.want {
			t.Errorf("%s: expected %q, got %q", name, tt.want, got)
		}
	}
}
