package main

import (
	"slices"
	"testing"

	"train-status-app/backend/internal/client"
)

func TestOdptSources(t *testing.T) {

	got := odptSources([]string{"TokyoMetro", "Unknown", "Toei", "TokyoMetro", "JR-East"})

	var names []string
	for _, s := range got {
		names = append(names, s.Name)
	}

	// 都営は常に先頭に1つ。未知の名前と重複は捨てる
	if want := []string{"Toei", "TokyoMetro", "JR-East"}; !slices.Equal(names, want) {
		t.Fatalf("expected %v, got %v", want, names)
	}

	if got := odptSources(nil); len(got) != 1 || got[0] != client.Sources["Toei"] {
		t.Fatalf("expected only Toei, got %+v", got)
	}
}
