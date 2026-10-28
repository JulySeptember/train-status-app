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
	stop := 0.0
	return &guideLabels{
		Name:     "test",
		Stations: []string{"odpt.Station:Test.Line.Here", "odpt.Station:Test.Branch.Here"},
		Platforms: []labelPlatform{
			{
				Space: "A",
				Floor: 1,
				Sides: []labelSide{
					{Track: "1", Railway: "odpt.Railway:Test.Line", Direction: "odpt.RailDirection:Test.Up", Stop: &stop},
					{Track: "2", Railway: "odpt.Railway:Test.Line", Direction: "odpt.RailDirection:Test.Down", Stop: &stop},
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
		Cars: map[string]int{"odpt.Railway:Test.Line": 10},
		// testNetwork の G（点の ID は作った順の番号）
		Gates:     []labelGate{{Name: "北改札", Node: 22}},
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
		"unknown gate node":     func(l *guideLabels) { l.Gates[0].Node = 999 },
		"unknown direction":     func(l *guideLabels) { l.Platforms[0].Sides[0].Direction = "odpt.RailDirection:Test.Left" },
		// ホーム A と B は、2階の S を通ってしかつながっていない。S を改札にすると、ホームの間を行けない
		"platforms only through a gate": func(l *guideLabels) { l.Gates = append(l.Gates, labelGate{Name: "中改札", Node: 20}) },
		"front perpendicular":           func(l *guideLabels) { l.Platforms[0].Sides[0].Front = "east" },
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

// コミットしている対応表が読めること（データが無くても確かめられる範囲）。
func TestGuideLabelFiles(t *testing.T) {

	files, err := filepath.Glob(filepath.Join("..", "..", "assets", "guide", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no label files: %v", err)
	}

	for _, f := range files {
		// readLabels は重なりと乗り換え先の改札も確かめる
		l, err := readLabels(f)
		if err != nil {
			t.Fatal(err)
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

func TestBuildStationGuideFront(t *testing.T) {

	// 隣の駅が真東にある路線（ホームは南北）。前の端を決められない
	lines := testLines()
	lon, lat := testLonLat(1000, 100)
	lines.stations["odpt.Station:Test.Line.North"] = [2]float64{lon, lat}

	if _, err := buildStationGuide(testLabels(), testNetwork(), testSpaces(), lines); err == nil {
		t.Fatal("expected an error when the neighbor station is perpendicular to the platform")
	}

	// 対応表で前の端の向きを書けば決まる
	labels := testLabels()
	labels.Platforms[0].Sides[0].Front = "north"

	g, err := buildStationGuide(labels, testNetwork(), testSpaces(), lines)
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", guide.Target{Gate: "北改札"}); e.CarFromFront != 1 {
		t.Errorf("expected car 1 with front north, got %d", e.CarFromFront)
	}
}

func TestGuideLabelsValidate(t *testing.T) {

	for name, edit := range map[string]func(*guideLabels){
		"duplicate gate": func(l *guideLabels) { l.Gates = append(l.Gates, labelGate{Name: "北改札", Node: 20}) },
		"duplicate side": func(l *guideLabels) {
			l.Platforms[1].Sides = append(l.Platforms[1].Sides, l.Platforms[0].Sides[0])
		},
		"unknown front":         func(l *guideLabels) { l.Platforms[0].Sides[0].Front = "up" },
		"unknown side":          func(l *guideLabels) { l.Platforms[0].Sides[0].Side = "up" },
		"negative stop":         func(l *guideLabels) { v := -1.0; l.Platforms[0].Sides[0].Stop = &v },
		"stop without cars":     func(l *guideLabels) { v := 0.0; l.Platforms[1].Sides[0].Stop = &v },
		"unknown transfer gate": func(l *guideLabels) { l.Transfers["odpt.Station:Other.Here"] = "南改札" },
	} {
		t.Run(name, func(t *testing.T) {
			labels := testLabels()
			edit(labels)
			if err := labels.validate(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	if err := testLabels().validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildStationGuideStop(t *testing.T) {

	gate := guide.Target{Gate: "北改札"}

	// 先頭が前の端から 100m に止まると、北の端（前）の階段は先頭より 90m 先にある。最も近いのは1両目で、
	// 1両目の中ほど（前の端から 110m）から階段（10m）まで 100m と、階段・2階の 10m
	labels := testLabels()
	stop := 100.0
	labels.Platforms[0].Sides[0].Stop = &stop
	labels.Platforms[0].Sides[1].Stop = &stop

	g, err := buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", gate); e.CarFromFront != 1 || e.WalkMeters != 110 {
		t.Errorf("expected car 1 and 110m, got %+v", e)
	}

	// 南へ進む列車は、南の端から 100m〜300m（区画は 200m なので外まで）。北の端の階段に近いのは5両目
	if e := findEntry(t, g.station, "2", gate); e.CarFromFront != 5 {
		t.Errorf("expected car 5, got %d", e.CarFromFront)
	}

	// 止まる位置が無い番線は、何両目を出さない
	labels = testLabels()
	labels.Platforms[0].Sides[0].Stop = nil

	g, err = buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", gate); e.CarFromFront != 0 || e.Cars != 0 {
		t.Errorf("expected no car without stop, got %d of %d", e.CarFromFront, e.Cars)
	}
}

func TestBuildStationGuideToward(t *testing.T) {

	gate := guide.Target{Gate: "北改札"}

	// 1番線（北へ進む）はホーム A の西の側。先頭が前の端から 100m に止まると、1両目の中ほど（前の端から 110m）から、
	// 北の端の階段（前の端から 10m）へ進行方向に 100m 歩く。列車を背にすると東を向くので、北は左
	labels := testLabels()
	stop := 100.0
	labels.Platforms[0].Sides[0].Stop = &stop
	labels.Platforms[0].Sides[0].Side = "west"

	g, err := buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	e := findEntry(t, g.station, "1", gate)
	if e.Toward != guide.TowardFront || e.TowardMeters != 100 || e.Turn != guide.TurnLeft {
		t.Errorf("expected front 100m to the left, got %+v", e)
	}

	// 東の側なら右
	labels.Platforms[0].Sides[0].Side = "east"
	g, err = buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", gate); e.Turn != guide.TurnRight {
		t.Errorf("expected right, got %+v", e)
	}

	// 側が無ければ右・左を出さない。先頭が前の端に止まると、1両目の中ほど（10m）は階段のすぐそば
	g, err = buildStationGuide(testLabels(), testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", gate); e.Toward != "" || e.Turn != "" {
		t.Errorf("expected no walking on the platform, got %+v", e)
	}

	// 側がホームの軸と平行ならエラー
	labels.Platforms[0].Sides[0].Side = "north"
	if _, err := buildStationGuide(labels, testNetwork(), testSpaces(), testLines()); err == nil {
		t.Error("expected an error for a side parallel to the platform")
	}

	// 2番線（南へ進む）を3両にすると、先頭が南の端に止まって3両目の中ほど（前の端から 50m）から、
	// 北の端の階段（前の端から 190m）へ進行方向と逆（北）に 140m 歩く。西の側なら東を向くので北は左、東の側なら右
	for side, want := range map[string]string{"west": guide.TurnLeft, "east": guide.TurnRight} {
		labels := testLabels()
		labels.Cars["odpt.Railway:Test.Line"] = 3
		labels.Platforms[0].Sides[1].Side = side

		g, err := buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
		if err != nil {
			t.Fatal(err)
		}
		e := findEntry(t, g.station, "2", gate)
		if e.CarFromFront != 3 || e.Toward != guide.TowardBack || e.TowardMeters != 140 || e.Turn != want {
			t.Errorf("side %s: expected car 3, back 140m to the %s, got %+v", side, want, e)
		}
	}

	// 止まる位置が無ければ、ホームを出る場所を出す。北へ進む1番線では北の端の階段は前寄り、南へ進む2番線では後ろ寄り
	labels = testLabels()
	labels.Platforms[0].Sides[0].Stop = nil
	labels.Platforms[0].Sides[1].Stop = nil
	g, err = buildStationGuide(labels, testNetwork(), testSpaces(), testLines())
	if err != nil {
		t.Fatal(err)
	}
	if e := findEntry(t, g.station, "1", gate); e.ExitAt != guide.ExitFront || e.Toward != "" {
		t.Errorf("expected exit at the front, got %+v", e)
	}
	if e := findEntry(t, g.station, "2", gate); e.ExitAt != guide.ExitBack {
		t.Errorf("expected exit at the back, got %+v", e)
	}
}
