package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInTokyo(t *testing.T) {
	for code, want := range map[string]bool{
		"13101": true,  // 千代田区
		"13123": true,  // 江戸川区
		"13201": true,  // 八王子市
		"13308": true,  // 奥多摩町
		"13000": false, // 所属未定地
		"13361": false, // 大島町（島しょ部）
		"13421": false, // 小笠原村
		"14101": false, // 横浜市鶴見区
	} {
		if got := inTokyo(code); got != want {
			t.Errorf("%s: expected %v, got %v", code, want, got)
		}
	}
}

func TestPolygonContains(t *testing.T) {

	// 外側 0〜10 の正方形に、4〜6 の穴
	p := polygon{
		{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
		{{4, 4}, {6, 4}, {6, 6}, {4, 6}, {4, 4}},
	}

	for _, c := range []struct {
		lon, lat float64
		want     bool
	}{
		{1, 1, true},
		{5, 5, false}, // 穴の中
		{11, 5, false},
		{5, -1, false},
	} {
		if got := p.contains(c.lon, c.lat); got != c.want {
			t.Errorf("(%v, %v): expected %v, got %v", c.lon, c.lat, c.want, got)
		}
	}
}

func TestTokyoStationsAndArea(t *testing.T) {

	dir := t.TempDir()

	geojson := filepath.Join(dir, "n03.geojson")
	writeFile(t, geojson, `{"features": [
		{"properties": {"N03_007": "13101"}, "geometry": {"type": "Polygon", "coordinates": [[[139,35],[140,35],[140,36],[139,36],[139,35]]]}},
		{"properties": {"N03_007": "14101"}, "geometry": {"type": "Polygon", "coordinates": [[[140,35],[141,35],[141,36],[140,36],[140,35]]]}}
	]}`)

	stations := filepath.Join(dir, "station.json")
	writeFile(t, stations, `[
		{"owl:sameAs": "odpt.Station:B", "geo:lat": 35.5, "geo:long": 139.5},
		{"owl:sameAs": "odpt.Station:A", "geo:lat": 35.5, "geo:long": 139.6},
		{"owl:sameAs": "odpt.Station:Kanagawa", "geo:lat": 35.5, "geo:long": 140.5},
		{"owl:sameAs": "odpt.Station:NoGeo"}
	]`)

	polygons, err := loadPolygons(geojson)
	if err != nil {
		t.Fatal(err)
	}

	got, err := tokyoStations(polygons, []string{stations, stations})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"odpt.Station:A", "odpt.Station:B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	var buf strings.Builder
	if err := writeArea(&buf, got); err != nil {
		t.Fatal(err)
	}

	area := filepath.Join(dir, "tokyo_stations.txt")
	writeFile(t, area, buf.String())

	read, err := readArea(area)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(read, map[string]bool{"odpt.Station:A": true, "odpt.Station:B": true}) {
		t.Fatalf("unexpected area %v", read)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
