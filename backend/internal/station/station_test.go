package station

import (
	"testing"

	"train-status-app/backend/assets"
)

func newIndex(t *testing.T) *Index {
	t.Helper()

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}
	return New(loader.Stations())
}

func names(stations []Station) []string {
	var result []string
	for _, s := range stations {
		result = append(result, s.Name+"/"+s.Railway)
	}
	return result
}

func TestFindSameName(t *testing.T) {
	idx := newIndex(t)

	// 春日は三田線と大江戸線に同じ名前の駅がある
	got := idx.Find("春日")
	if len(got) != 2 {
		t.Fatalf("expected 2 stations, got %v", names(got))
	}
	for _, s := range got {
		if s.Name != "春日" {
			t.Fatalf("unexpected station %v", names(got))
		}
	}
}

func TestFindVariants(t *testing.T) {
	idx := newIndex(t)

	tests := []struct {
		input string
		want  string
	}{
		{"浅草駅", "浅草"},
		{" 浅 草 ", "浅草"},
		{"市ケ谷", "市ヶ谷"},
		{"市ヶ谷駅", "市ヶ谷"},
		{"Ichigaya", "市ヶ谷"},
		{"ＩＣＨＩＧＡＹＡ", "市ヶ谷"},
		{"nishi-magome", "西馬込"},
	}

	for _, tt := range tests {
		got := idx.Find(tt.input)
		if len(got) == 0 || got[0].Name != tt.want {
			t.Errorf("Find(%q) = %v, want %s", tt.input, names(got), tt.want)
		}
		for _, s := range got {
			if s.Name != tt.want {
				t.Errorf("Find(%q) returned extra station %v", tt.input, names(got))
			}
		}
	}
}

func TestFindPartial(t *testing.T) {
	idx := newIndex(t)

	// 「新宿」は同じ名前の駅があるので、新宿三丁目・新宿西口などは返さない
	for _, s := range idx.Find("新宿") {
		if s.Name != "新宿" {
			t.Fatalf("unexpected station %s", s.Name)
		}
	}

	// 一致する駅が無ければ、名前の一部に一致する駅を短い順に返す
	got := idx.Find("三丁目")
	if len(got) == 0 {
		t.Fatal("expected partial matches")
	}
	if len(got) > maxCandidates {
		t.Fatalf("expected at most %d candidates, got %d", maxCandidates, len(got))
	}
}

func TestFindNotFound(t *testing.T) {
	idx := newIndex(t)

	for _, input := range []string{"渋谷", "", "駅", "  "} {
		if got := idx.Find(input); len(got) != 0 {
			t.Errorf("Find(%q) = %v, want none", input, names(got))
		}
	}
}
