package main

import (
	"archive/zip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"train-status-app/backend/assets/guide"
	"train-status-app/backend/internal/model"
)

// 架空の駅: 南北に長いホーム A（x=0）と B（x=23）。どちらも長さ 200m（y=0〜200）で1階。
// 北の端（y=190）に、A・B それぞれ2階へ上がる階段があり、2階でつながっている。2階の北に改札がある。
//
//	2階:  S(0,190) ──23m── T(23,190)     G(0,195) 改札
//	         │階段            │エスカレーター
//	1階:  A190 … A10        B190 … B10
const (
	testLon0 = 139.7
	testLat0 = 35.69
)

func testLonLat(x, y float64) (float64, float64) {
	return testLon0 + x/(111320*math.Cos(testLat0*math.Pi/180)), testLat0 + y/110540
}

func testRect(x0, x1, y0, y1 float64) ring {
	var r ring
	for _, p := range [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}, {x0, y0}} {
		lon, lat := testLonLat(p[0], p[1])
		r = append(r, [2]float64{lon, lat})
	}
	return r
}

func testNetwork() *network {

	nw := &network{}
	add := func(x, y, floor float64) int {
		lon, lat := testLonLat(x, y)
		nw.nodes = append(nw.nodes, gNode{id: len(nw.nodes), lon: lon, lat: lat, floor: floor})
		return len(nw.nodes) - 1
	}
	link := func(a, b int, meters float64, routeType int) {
		nw.links = append(nw.links, gLink{from: a, to: b, meters: meters, routeType: routeType})
	}

	// ホームの上の点を 20m ごとにつなぐ
	chain := func(x float64) (top int) {
		prev := -1
		for y := 10.0; y <= 190; y += 20 {
			n := add(x, y, 1)
			if prev >= 0 {
				link(prev, n, 20, routeFlat)
			}
			prev = n
		}
		return prev
	}

	a190 := chain(0)
	b190 := chain(23)

	s := add(0, 190, 2)
	tt := add(23, 190, 2)
	g := add(0, 195, 2)

	link(a190, s, 5, routeStairs)
	link(b190, tt, 5, routeEscalator)
	link(s, tt, 23, routeFlat)
	link(s, g, 5, routeFlat)

	return nw
}

func testLines() *jrLines {

	place := func(y float64) [2]float64 {
		lon, lat := testLonLat(10, y)
		return [2]float64{lon, lat}
	}

	return &jrLines{
		railways: map[string]model.Railway{
			"odpt.Railway:Test.Line": {
				SameAs: "odpt.Railway:Test.Line",
				StationOrder: []model.StationOrder{
					{Station: "odpt.Station:Test.Line.South"},
					{Station: "odpt.Station:Test.Line.Here"},
					{Station: "odpt.Station:Test.Line.North"},
				},
				AscendingRailDirection:  "odpt.RailDirection:Test.Up",
				DescendingRailDirection: "odpt.RailDirection:Test.Down",
			},
			// Here が終点（最初の駅）の路線
			"odpt.Railway:Test.Branch": {
				SameAs: "odpt.Railway:Test.Branch",
				StationOrder: []model.StationOrder{
					{Station: "odpt.Station:Test.Branch.Here"},
					{Station: "odpt.Station:Test.Branch.North"},
				},
				AscendingRailDirection:  "odpt.RailDirection:Test.Up",
				DescendingRailDirection: "odpt.RailDirection:Test.Down",
			},
		},
		stations: map[string][2]float64{
			"odpt.Station:Test.Line.South":   place(-1000),
			"odpt.Station:Test.Line.Here":    place(100),
			"odpt.Station:Test.Line.North":   place(1000),
			"odpt.Station:Test.Branch.Here":  place(100),
			"odpt.Station:Test.Branch.North": place(1000),
		},
	}
}

func testLabels() *guideLabels {
	return &guideLabels{
		Name:     "test",
		Stations: []string{"odpt.Station:Test.Line.Here", "odpt.Station:Test.Branch.Here"},
		Platforms: []labelPlatform{
			{
				Space: "A",
				Floor: 1,
				Sides: []labelSide{
					{Track: "1", Railway: "odpt.Railway:Test.Line", Direction: "odpt.RailDirection:Test.Up"},
					{Track: "2", Railway: "odpt.Railway:Test.Line", Direction: "odpt.RailDirection:Test.Down"},
				},
			},
			{
				Space: "B",
				Floor: 1,
				Sides: []labelSide{
					{Track: "3", Railway: "odpt.Railway:Test.Branch", Direction: "odpt.RailDirection:Test.Down"},
				},
			},
		},
		Cars:      map[string]int{"odpt.Railway:Test.Line": 10},
		Gates:     []labelGate{{Name: "北改札", Floor: 2, Lat: testLat0 + 196/110540.0, Lon: testLon0}},
		Transfers: map[string]string{"odpt.Station:Other.Here": "北改札"},
	}
}

func testSpaces() map[string]ring {
	return map[string]ring{
		"A": testRect(-3, 3, 0, 200),
		"B": testRect(20, 26, 0, 200),
	}
}

func findEntry(t *testing.T, s guide.Station, track string, target guide.Target) guide.Entry {
	t.Helper()

	labels := testLabels()
	var railway, direction string
	for _, p := range labels.Platforms {
		for _, side := range p.Sides {
			if side.Track == track {
				railway, direction = side.Railway, side.Direction
			}
		}
	}

	for _, e := range s.Entries {
		if e.Railway == railway && e.Direction == direction && e.Target == target {
			return e
		}
	}

	t.Fatalf("no entry for track %s → %+v", track, target)
	return guide.Entry{}
}

func TestBuildStationGuide(t *testing.T) {

	g, err := buildStationGuide(testLabels(), testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}

	s := g.station
	gate := guide.Target{Gate: "北改札"}

	t.Run("front is the end toward the next station", func(t *testing.T) {
		// 北へ進む列車は北の端が前なので、北の階段に近いのは1両目
		e := findEntry(t, s, "1", gate)
		if e.CarFromFront != 1 || e.Cars != 10 {
			t.Errorf("expected car 1 of 10, got %d of %d", e.CarFromFront, e.Cars)
		}
		if e.Via != guide.ViaStairs {
			t.Errorf("expected stairs, got %q", e.Via)
		}
		// 階段 5m + 2階 5m
		if e.WalkMeters != 10 {
			t.Errorf("expected 10m, got %d", e.WalkMeters)
		}

		// 南へ進む列車は南の端が前なので、10両目
		if e := findEntry(t, s, "2", gate); e.CarFromFront != 10 {
			t.Errorf("expected car 10, got %d", e.CarFromFront)
		}
	})

	t.Run("to another platform", func(t *testing.T) {
		e := findEntry(t, s, "1", guide.Target{Railway: "odpt.Railway:Test.Branch", Direction: "odpt.RailDirection:Test.Down", Track: "3"})
		// 階段 5m + 2階 23m + エスカレーター 5m
		if e.CarFromFront != 1 || e.WalkMeters != 33 || e.Via != guide.ViaStairs {
			t.Errorf("unexpected entry: %+v", e)
		}
		// 33m + 1階上り下り2回分（20m × 2）を 80m/分で切り上げ
		if e.WalkMinutes != 1 {
			t.Errorf("expected 1 minute, got %d", e.WalkMinutes)
		}
	})

	t.Run("unknown number of cars", func(t *testing.T) {
		e := findEntry(t, s, "3", gate)
		if e.CarFromFront != 0 || e.Cars != 0 {
			t.Errorf("expected no car, got %d of %d", e.CarFromFront, e.Cars)
		}
		if e.Via != guide.ViaEscalator || e.WalkMeters != 33 {
			t.Errorf("unexpected entry: %+v", e)
		}
	})

	t.Run("no entries within the same platform", func(t *testing.T) {
		for _, e := range s.Entries {
			if e.Target.Track == "2" && e.Direction == "odpt.RailDirection:Test.Up" {
				t.Errorf("unexpected entry to the same platform: %+v", e)
			}
		}
	})

	t.Run("count", func(t *testing.T) {
		// 番線3つ × 改札1つ + 別のホームへ（A の2つ → B の1つ、B の1つ → A の2つ）
		if len(s.Entries) != 3+2+2 {
			t.Errorf("expected 7 entries, got %d", len(s.Entries))
		}
	})

	if s.Transfers["odpt.Station:Other.Here"] != "北改札" {
		t.Errorf("unexpected transfers: %v", s.Transfers)
	}
}

func TestBuildStationGuideTerminal(t *testing.T) {

	g, err := buildStationGuide(testLabels(), testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}

	// Test.Branch の Down は、Here が終点で次の駅が無い。前の駅（北）の反対の、南の端が前になる
	for _, side := range g.sides {
		if side.Track == "3" && side.frontHigh {
			t.Error("expected the south end to be the front at the terminal")
		}
	}
}

func TestBuildStationGuideErrors(t *testing.T) {

	for name, edit := range map[string]func(*guideLabels){
		"unknown space":         func(l *guideLabels) { l.Platforms[0].Space = "X" },
		"no nodes on the floor": func(l *guideLabels) { l.Platforms[0].Floor = 3 },
		"gate too far":          func(l *guideLabels) { l.Gates[0].Lat += 100 / 110540.0 },
		"unknown transfer gate": func(l *guideLabels) { l.Transfers["odpt.Station:Other.Here"] = "南改札" },
		"unknown direction":     func(l *guideLabels) { l.Platforms[0].Sides[0].Direction = "odpt.RailDirection:Test.Left" },
	} {
		t.Run(name, func(t *testing.T) {
			labels := testLabels()
			edit(labels)
			if _, err := buildStationGuide(labels, testNetwork(), testSpaces(), testLines()); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestNeighborStation(t *testing.T) {

	order := func(names ...string) []model.StationOrder {
		var o []model.StationOrder
		for _, n := range names {
			o = append(o, model.StationOrder{Station: "odpt.Station:Test.Loop." + n})
		}
		return o
	}

	l := &jrLines{
		railways: map[string]model.Railway{
			"odpt.Railway:Test.Loop": {
				// 山手線と同じく、最初と最後が同じ駅
				StationOrder:            order("A", "B", "C", "A"),
				AscendingRailDirection:  "odpt.RailDirection:Test.Outer",
				DescendingRailDirection: "odpt.RailDirection:Test.Inner",
			},
		},
		stations: map[string][2]float64{
			"odpt.Station:Test.Loop.A": {1, 1},
			"odpt.Station:Test.Loop.B": {2, 2},
			"odpt.Station:Test.Loop.C": {3, 3},
		},
	}

	c, ahead, err := l.neighborStation("odpt.Railway:Test.Loop", "odpt.RailDirection:Test.Inner", "A")
	if err != nil {
		t.Fatal(err)
	}
	if !ahead || c != [2]float64{3, 3} {
		t.Errorf("expected C ahead, got %v %v", c, ahead)
	}
}

func TestReadNetworkSkipsDanglingLinks(t *testing.T) {

	p := filepath.Join(t.TempDir(), "nwd.zip")

	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)

	write := func(name string, v any) {
		w, err := zw.Create("dir/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.NewEncoder(w).Encode(v); err != nil {
			t.Fatal(err)
		}
	}

	feature := func(props map[string]any) map[string]any {
		return map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{0, 0}}, "properties": props}
	}

	write("node.geojson", map[string]any{"features": []any{
		feature(map[string]any{"node_id": 1, "lat": 35.69, "lon": 139.7, "floor": 1}),
		feature(map[string]any{"node_id": 2, "lat": 35.6901, "lon": 139.7, "floor": 1}),
	}})
	write("link.geojson", map[string]any{"features": []any{
		feature(map[string]any{"start_id": 1, "end_id": 2, "distance": 11.1, "route_type": 1}),
		feature(map[string]any{"start_id": 2, "end_id": 99, "distance": 5, "route_type": 1}),
	}})

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	nw, err := readNetwork(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(nw.nodes) != 2 || len(nw.links) != 1 {
		t.Fatalf("expected 2 nodes and 1 link, got %d and %d", len(nw.nodes), len(nw.links))
	}
}

// コミットしている対応表が読めて、乗り換え先の改札が改札の一覧にあること（データが無くても確かめられる範囲）。
func TestGuideLabelFiles(t *testing.T) {

	files, err := filepath.Glob(filepath.Join("..", "..", "assets", "guide", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no label files: %v", err)
	}

	for _, f := range files {
		l, err := readLabels(f)
		if err != nil {
			t.Fatal(err)
		}

		gates := map[string]bool{}
		for _, g := range l.Gates {
			gates[g.Name] = true
		}
		for station, gate := range l.Transfers {
			if !gates[gate] {
				t.Errorf("%s: transfer %s: unknown gate %s", f, station, gate)
			}
		}

		for _, p := range l.Platforms {
			for _, s := range p.Sides {
				if l.Cars[s.Railway] == 0 {
					t.Logf("%s: %s: number of cars is not set", f, s.Railway)
				}
			}
		}
	}
}
