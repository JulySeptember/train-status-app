package main

import (
	"archive/zip"
	"container/heap"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"train-status-app/backend/assets/guide"
	"train-status-app/backend/internal/model"
)

// JR の駅の中の案内の表を作る（-kind guide）。
//
// 入力:
//
//	<labels>/<駅>.yaml                                手で作る対応表（assets/guide）
//	<raw>/station-guide/jrc_nwd_<駅>_geojson.zip         歩行空間ネットワーク
//	<raw>/station-guide/jrc_stationmap_<駅>_geojson.zip  構内地図
//	<raw>/operators/JR-East/{Railway,Station}.json      路線の駅の並びと駅の座標（ホームの前の端を決める）
//
// 設計は docs/design/station-guide.md 6章。

// 経路の種類（歩行空間ネットワークデータ整備仕様の route_type）
const (
	routeFlat      = 1
	routeElevator  = 4
	routeEscalator = 5
	routeStairs    = 6
	routeSlope     = 7
)

const (
	// 階段・エスカレーターで1階上り下りするごとに足す距離（m）。15秒相当
	floorPenaltyMeters = 20
	// エレベーター1回に足す距離（m）。待ち時間を含めて60秒相当
	elevatorPenaltyMeters = 80
	// ホームの軸と、隣の駅への向きのなす角の cos がこれより小さいと、前の端を決められないとする（約60°）
	minFrontCos = 0.5
)

// guideLabels は手で作る対応表（assets/guide/<駅>.yaml）。
type guideLabels struct {
	Name      string            `yaml:"name"`
	Stations  []string          `yaml:"stations"`
	Platforms []labelPlatform   `yaml:"platforms"`
	Cars      map[string]int    `yaml:"cars"`
	Gates     []labelGate       `yaml:"gates"`
	Transfers map[string]string `yaml:"transfers"`
}

type labelPlatform struct {
	Space string      `yaml:"space"`
	Floor float64     `yaml:"floor"`
	Sides []labelSide `yaml:"sides"`
}

type labelSide struct {
	Track     string `yaml:"track"`
	Railway   string `yaml:"railway"`
	Direction string `yaml:"direction"`

	// 前の端の向き（north・south・east・west）。隣の駅への向きがホームの軸と直角に近く、
	// 前の端を決められないときに書く（線路が駅を出てすぐ曲がる路線）
	Front string `yaml:"front"`

	// 列車の先頭が止まる位置（区画の前の端から何 m）。区画は編成より長く、端にそろって止まるとは限らない。
	// 書いた番線だけ、前から何両目かを出す（分からない番線で、間違った車両を案内しないため）
	Stop *float64 `yaml:"stop"`
}

// labelGate は改札。位置は歩行空間ネットワークの点の ID（node_id）で指す
// （コミットするファイルに、データの座標を書かないため）。
type labelGate struct {
	Name string `yaml:"name"`
	Node int    `yaml:"node"`
}

// compassXY は、front の向きを東・北の単位ベクトルで表す。
var compassXY = map[string][2]float64{
	"north": {0, 1},
	"south": {0, -1},
	"east":  {1, 0},
	"west":  {-1, 0},
}

// genGuide は、labelsDir の対応表ごとに案内を作り、out に書く。previewDir が空でなければ、確認用の SVG も書く。
func genGuide(rawDir, labelsDir, out, previewDir string) error {

	labelFiles, err := filepath.Glob(filepath.Join(labelsDir, "*.yaml"))
	if err != nil {
		return err
	}
	if len(labelFiles) == 0 {
		return fmt.Errorf("%s: no *.yaml", labelsDir)
	}

	lines, err := loadJRLines(rawDir)
	if err != nil {
		return err
	}

	var table guide.Table

	for _, f := range labelFiles {

		labels, err := readLabels(f)
		if err != nil {
			return err
		}

		nw, err := readNetwork(filepath.Join(rawDir, "station-guide", "jrc_nwd_"+labels.Name+"_geojson.zip"))
		if err != nil {
			return err
		}

		spaces, err := readPlatformSpaces(filepath.Join(rawDir, "station-guide", "jrc_stationmap_"+labels.Name+"_geojson.zip"))
		if err != nil {
			return err
		}

		g, err := buildStationGuide(labels, nw, spaces, lines)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}

		table.Stations = append(table.Stations, g.station)

		log.Printf("%s: %d entries", labels.Name, len(g.station.Entries))

		if previewDir != "" {
			if err := os.MkdirAll(previewDir, 0o755); err != nil {
				return err
			}
			p := filepath.Join(previewDir, labels.Name+".svg")
			if err := os.WriteFile(p, g.preview(), 0o644); err != nil {
				return err
			}
			log.Printf("%s: preview %s", labels.Name, p)
		}
	}

	data, err := json.Marshal(table)
	if err != nil {
		return err
	}

	// 読み込みと同じ確かめ方をしておく
	if _, err := guide.Decode(data); err != nil {
		return err
	}

	return os.WriteFile(out, data, 0o644)
}

func readLabels(path string) (*guideLabels, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var l guideLabels

	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)

	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if l.Name == "" || len(l.Stations) == 0 || len(l.Platforms) == 0 {
		return nil, fmt.Errorf("%s: name, stations and platforms are required", path)
	}

	if err := l.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &l, nil
}

// validate は、対応表の中の重なりと書き方の誤りを確かめる。
func (l *guideLabels) validate() error {

	sides := map[[2]string]bool{}
	for _, p := range l.Platforms {
		for _, s := range p.Sides {
			key := [2]string{s.Railway, s.Direction}
			if sides[key] {
				return fmt.Errorf("%s %s: listed on two platforms", s.Railway, s.Direction)
			}
			sides[key] = true

			if _, ok := compassXY[s.Front]; s.Front != "" && !ok {
				return fmt.Errorf("track %s: unknown front %q", s.Track, s.Front)
			}

			if s.Stop != nil && (*s.Stop < 0 || l.Cars[s.Railway] == 0) {
				return fmt.Errorf("track %s: stop needs a non-negative value and the number of cars", s.Track)
			}
		}
	}

	gates := map[string]bool{}
	for _, g := range l.Gates {
		if gates[g.Name] {
			return fmt.Errorf("gate %s: listed twice", g.Name)
		}
		gates[g.Name] = true
	}

	for station, gate := range l.Transfers {
		if !gates[gate] {
			return fmt.Errorf("transfer %s: unknown gate %s", station, gate)
		}
	}

	return nil
}

// =========================
// 入力の読み込み
// =========================

type gNode struct {
	id       int
	lon, lat float64
	floor    float64
}

type gLink struct {
	from, to  int // nodes の添字
	meters    float64
	routeType int
}

type network struct {
	nodes []gNode
	links []gLink
}

type geoFeatures struct {
	Features []struct {
		Geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
		Properties json.RawMessage `json:"properties"`
	} `json:"features"`
}

// readZipFiles は ZIP の中のファイルを、名前（ディレクトリを除く）→ 中身で返す。
func readZipFiles(p string) (map[string][]byte, error) {

	r, err := zip.OpenReader(p)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	files := map[string][]byte{}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		files[path.Base(f.Name)] = data
	}

	return files, nil
}

func readNetwork(zipPath string) (*network, error) {

	files, err := readZipFiles(zipPath)
	if err != nil {
		return nil, err
	}

	var nodeFC, linkFC geoFeatures

	for name, v := range map[string]*geoFeatures{"node.geojson": &nodeFC, "link.geojson": &linkFC} {
		data, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("%s: no %s", zipPath, name)
		}
		if err := json.Unmarshal(data, v); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", zipPath, name, err)
		}
	}

	nw := &network{}
	index := map[int]int{}

	for _, f := range nodeFC.Features {
		var p struct {
			NodeID int     `json:"node_id"`
			Lat    float64 `json:"lat"`
			Lon    float64 `json:"lon"`
			Floor  float64 `json:"floor"`
		}
		if err := json.Unmarshal(f.Properties, &p); err != nil {
			return nil, fmt.Errorf("%s: node: %w", zipPath, err)
		}
		index[p.NodeID] = len(nw.nodes)
		nw.nodes = append(nw.nodes, gNode{id: p.NodeID, lon: p.Lon, lat: p.Lat, floor: p.Floor})
	}

	var dangling int

	for _, f := range linkFC.Features {
		var p struct {
			StartID   int     `json:"start_id"`
			EndID     int     `json:"end_id"`
			Distance  float64 `json:"distance"`
			RouteType int     `json:"route_type"`
		}
		if err := json.Unmarshal(f.Properties, &p); err != nil {
			return nil, fmt.Errorf("%s: link: %w", zipPath, err)
		}
		from, ok1 := index[p.StartID]
		to, ok2 := index[p.EndID]
		if !ok1 || !ok2 {
			// 点の無い線がある（新宿で 1,062本中2本、2026-10-28）。つなげないので捨てる
			dangling++
			continue
		}
		nw.links = append(nw.links, gLink{from: from, to: to, meters: p.Distance, routeType: p.RouteType})
	}

	if dangling > 0 {
		log.Printf("%s: skipped %d links to unknown nodes", filepath.Base(zipPath), dangling)
	}

	return nw, nil
}

// readPlatformSpaces は、構内地図の区画のうちホーム（B028）の輪郭を、区画の ID → 外側の輪郭で返す。
func readPlatformSpaces(zipPath string) (map[string]ring, error) {

	files, err := readZipFiles(zipPath)
	if err != nil {
		return nil, err
	}

	spaces := map[string]ring{}

	for name, data := range files {

		if !strings.HasSuffix(name, "_Space.geojson") {
			continue
		}

		var fc geoFeatures
		if err := json.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("%s/%s: %w", zipPath, name, err)
		}

		for _, f := range fc.Features {

			var p struct {
				ID       string `json:"id"`
				Category string `json:"category"`
			}
			if err := json.Unmarshal(f.Properties, &p); err != nil {
				return nil, fmt.Errorf("%s/%s: %w", zipPath, name, err)
			}
			if p.Category != "B028" {
				continue
			}

			var pgs []polygon

			switch f.Geometry.Type {
			case "Polygon":
				var pg polygon
				if err := json.Unmarshal(f.Geometry.Coordinates, &pg); err != nil {
					return nil, fmt.Errorf("%s/%s: %w", zipPath, name, err)
				}
				pgs = []polygon{pg}
			case "MultiPolygon":
				if err := json.Unmarshal(f.Geometry.Coordinates, &pgs); err != nil {
					return nil, fmt.Errorf("%s/%s: %w", zipPath, name, err)
				}
			default:
				return nil, fmt.Errorf("%s/%s: unsupported geometry %s", zipPath, name, f.Geometry.Type)
			}

			// ホームは1つの多角形のはず。分かれていると軸も内外の判定も変わるので、使わない
			if len(pgs) != 1 || len(pgs[0]) == 0 || len(pgs[0][0]) < 4 {
				return nil, fmt.Errorf("%s/%s: platform space %s: expected a single polygon", zipPath, name, p.ID)
			}

			spaces[p.ID] = pgs[0][0]
		}
	}

	if len(spaces) == 0 {
		return nil, fmt.Errorf("%s: no platform spaces (B028)", zipPath)
	}

	return spaces, nil
}

// jrLines は JR の路線の駅の並びと駅の座標。
type jrLines struct {
	railways map[string]model.Railway
	stations map[string][2]float64 // 駅ID → [経度, 緯度]
}

func loadJRLines(rawDir string) (*jrLines, error) {

	var railways []model.Railway
	if err := readJSON(filepath.Join(rawDir, "operators", "JR-East", "Railway.json"), &railways); err != nil {
		return nil, err
	}

	var stations []model.Station
	if err := readJSON(filepath.Join(rawDir, "operators", "JR-East", "Station.json"), &stations); err != nil {
		return nil, err
	}

	l := &jrLines{railways: map[string]model.Railway{}, stations: map[string][2]float64{}}

	for _, r := range railways {
		l.railways[r.SameAs] = r
	}
	for _, s := range stations {
		if s.Latitude != 0 || s.Longitude != 0 {
			l.stations[s.SameAs] = [2]float64{s.Longitude, s.Latitude}
		}
	}

	return l, nil
}

// neighborStation は、路線 railway を direction に進む列車が、station の次に停まる駅の座標を返す（ahead は true）。
// 終点で次の駅が無ければ、前の駅の座標を返す（ahead は false）。
// 環状線（山手線）は駅の並びの最初と最後が同じ駅なので、端で反対の端につなぐ。
func (l *jrLines) neighborStation(railway, direction, stationName string) (c [2]float64, ahead bool, err error) {

	r, ok := l.railways[railway]
	if !ok {
		return c, false, fmt.Errorf("%s: unknown railway", railway)
	}

	order := r.StationOrder
	i := slices.IndexFunc(order, func(o model.StationOrder) bool {
		return lastSegment(o.Station) == stationName
	})
	if i < 0 || len(order) < 2 {
		return c, false, fmt.Errorf("%s: no station %s", railway, stationName)
	}

	var step int
	switch direction {
	case r.AscendingRailDirection:
		step = 1
	case r.DescendingRailDirection:
		step = -1
	default:
		return c, false, fmt.Errorf("%s: unknown direction %s", railway, direction)
	}

	loop := order[0].Station == order[len(order)-1].Station

	j, ahead := i+step, true
	switch {
	case j >= 0 && j < len(order):
	case loop && j < 0:
		j = len(order) - 2
	case loop:
		j = 1
	default:
		j, ahead = i-step, false
	}

	c, ok = l.stations[order[j].Station]
	if !ok {
		return c, false, fmt.Errorf("%s: no coordinates", order[j].Station)
	}

	return c, ahead, nil
}

// =========================
// 計算
// =========================

// localXY は、緯度・経度を基準点からの東西・南北の距離（m）に直す。駅の中の範囲なので平面で近似する。
type localXY struct {
	lon0, lat0 float64
	kx, ky     float64
}

func newLocalXY(lon0, lat0 float64) localXY {
	return localXY{
		lon0: lon0,
		lat0: lat0,
		kx:   111320 * math.Cos(lat0*math.Pi/180),
		ky:   110540,
	}
}

func (p localXY) xy(lon, lat float64) (float64, float64) {
	return (lon - p.lon0) * p.kx, (lat - p.lat0) * p.ky
}

// platformAxis はホームの区画の長い向きの軸。位置は「前の端から何 m」で表す。
type platformAxis struct {
	cx, cy float64 // 中心（m）
	ux, uy float64 // 軸の向きの単位ベクトル
	lo, hi float64 // 区画の頂点を軸に射影した範囲
}

func newPlatformAxis(r ring, p localXY) platformAxis {

	var xs, ys []float64
	for _, v := range r {
		x, y := p.xy(v[0], v[1])
		xs = append(xs, x)
		ys = append(ys, y)
	}

	var a platformAxis
	for i := range xs {
		a.cx += xs[i]
		a.cy += ys[i]
	}
	a.cx /= float64(len(xs))
	a.cy /= float64(len(ys))

	// 主成分の向き
	var sxx, syy, sxy float64
	for i := range xs {
		dx, dy := xs[i]-a.cx, ys[i]-a.cy
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
	}
	th := 0.5 * math.Atan2(2*sxy, sxx-syy)
	a.ux, a.uy = math.Cos(th), math.Sin(th)

	a.lo, a.hi = math.Inf(1), math.Inf(-1)
	for i := range xs {
		u := a.project(xs[i], ys[i])
		a.lo = min(a.lo, u)
		a.hi = max(a.hi, u)
	}

	return a
}

func (a platformAxis) project(x, y float64) float64 {
	return (x-a.cx)*a.ux + (y-a.cy)*a.uy
}

// fromFront は、軸の上の位置 u を、前の端（frontHigh なら hi の端）から何 m かに直す。
func (a platformAxis) fromFront(u float64, frontHigh bool) float64 {
	if frontHigh {
		return a.hi - u
	}
	return u - a.lo
}

// route は、行き先までの最短の経路（Dijkstra）の結果。
type route struct {
	cost   []float64 // 段差の分を足した距離（m）
	meters []float64 // 実際に歩く距離（m）
	next   []int     // 行き先へ向かう次の点（行き先なら -1）
	via    []int     // 次の点へ進む線の route_type
}

type adjEdge struct {
	to        int
	meters    float64
	cost      float64
	routeType int
}

// adjacency は、点ごとの隣の点への線を返す。excluded の点を通る線は入れない。
func (nw *network) adjacency(excluded map[int]bool) [][]adjEdge {

	adj := make([][]adjEdge, len(nw.nodes))

	for _, l := range nw.links {

		if excluded[l.from] || excluded[l.to] {
			continue
		}

		cost := l.meters
		floors := math.Abs(nw.nodes[l.from].floor - nw.nodes[l.to].floor)

		switch l.routeType {
		case routeStairs, routeEscalator:
			cost += floors * floorPenaltyMeters
		case routeElevator:
			cost += elevatorPenaltyMeters
		}

		// エスカレーターの向きはデータに無い（不明）ので、どの線も両方向に歩けるとする
		adj[l.from] = append(adj[l.from], adjEdge{to: l.to, meters: l.meters, cost: cost, routeType: l.routeType})
		adj[l.to] = append(adj[l.to], adjEdge{to: l.from, meters: l.meters, cost: cost, routeType: l.routeType})
	}

	return adj
}

type pqItem struct {
	node int
	cost float64
}

type pq []pqItem

func (q pq) Len() int           { return len(q) }
func (q pq) Less(i, j int) bool { return q[i].cost < q[j].cost }
func (q pq) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)        { *q = append(*q, x.(pqItem)) }
func (q *pq) Pop() any {
	old := *q
	it := old[len(old)-1]
	*q = old[:len(old)-1]
	return it
}

// shortestTo は、行き先の点（targets）から逆にたどって、各点から行き先までの最短の経路を求める。
func shortestTo(adj [][]adjEdge, targets []int) route {

	n := len(adj)
	r := route{
		cost:   make([]float64, n),
		meters: make([]float64, n),
		next:   make([]int, n),
		via:    make([]int, n),
	}
	for i := range n {
		r.cost[i] = math.Inf(1)
		r.next[i] = -1
	}

	q := &pq{}
	for _, t := range targets {
		r.cost[t] = 0
		heap.Push(q, pqItem{node: t})
	}

	for q.Len() > 0 {
		it := heap.Pop(q).(pqItem)
		if it.cost > r.cost[it.node] {
			continue
		}
		for _, e := range adj[it.node] {
			c := it.cost + e.cost
			if c < r.cost[e.to] {
				r.cost[e.to] = c
				r.meters[e.to] = r.meters[it.node] + e.meters
				r.next[e.to] = it.node
				r.via[e.to] = e.routeType
				heap.Push(q, pqItem{node: e.to, cost: c})
			}
		}
	}

	return r
}

// viaFrom は、点 u から行き先へ向かう経路で、最初に使う平らでない設備を返す。
func (r route) viaFrom(u int) string {
	for v := u; r.next[v] >= 0; v = r.next[v] {
		switch r.via[v] {
		case routeStairs:
			return guide.ViaStairs
		case routeEscalator:
			return guide.ViaEscalator
		case routeElevator:
			return guide.ViaElevator
		case routeSlope:
			return guide.ViaSlope
		}
	}
	return ""
}

// guidePlatform は、対応表のホームを計算に使える形にしたもの。
type guidePlatform struct {
	label labelPlatform
	ring  ring
	axis  platformAxis
	nodes []int // 区画の中にある、ホームの階の点
}

type guideSide struct {
	labelSide
	platform  *guidePlatform
	frontHigh bool // 進む方向の前の端が軸の hi の側か
}

type stationGuide struct {
	labels    *guideLabels
	nw        *network
	proj      localXY
	platforms []*guidePlatform
	sides     []guideSide
	gateNodes map[string]int
	station   guide.Station
}

func buildStationGuide(labels *guideLabels, nw *network, spaces map[string]ring, lines *jrLines) (*stationGuide, error) {

	if len(nw.nodes) == 0 {
		return nil, fmt.Errorf("no network nodes")
	}

	// 基準点は点の平均
	var lon0, lat0 float64
	for _, n := range nw.nodes {
		lon0 += n.lon
		lat0 += n.lat
	}
	g := &stationGuide{
		labels:    labels,
		nw:        nw,
		proj:      newLocalXY(lon0/float64(len(nw.nodes)), lat0/float64(len(nw.nodes))),
		gateNodes: map[string]int{},
	}

	stationName := lastSegment(labels.Stations[0])

	for _, lp := range labels.Platforms {

		r, ok := spaces[lp.Space]
		if !ok {
			return nil, fmt.Errorf("platform space %s: not found in the station map", lp.Space)
		}

		p := &guidePlatform{label: lp, ring: r, axis: newPlatformAxis(r, g.proj)}

		pg := polygon{r}
		for i, n := range nw.nodes {
			if n.floor == lp.Floor && pg.contains(n.lon, n.lat) {
				p.nodes = append(p.nodes, i)
			}
		}
		if len(p.nodes) == 0 {
			return nil, fmt.Errorf("platform space %s: no network nodes on floor %v", lp.Space, lp.Floor)
		}

		g.platforms = append(g.platforms, p)

		for _, s := range lp.Sides {

			frontHigh, err := g.frontHigh(p, s, lines, stationName)
			if err != nil {
				return nil, fmt.Errorf("track %s: %w", s.Track, err)
			}

			g.sides = append(g.sides, guideSide{
				labelSide: s,
				platform:  p,
				frontHigh: frontHigh,
			})
		}
	}

	index := map[int]int{}
	for i, n := range nw.nodes {
		index[n.id] = i
	}

	gateSet := map[int]bool{}
	for _, gate := range labels.Gates {
		n, ok := index[gate.Node]
		if !ok {
			return nil, fmt.Errorf("gate %s: no network node %d", gate.Name, gate.Node)
		}
		g.gateNodes[gate.Name] = n
		gateSet[n] = true
	}

	g.station = guide.Station{
		StationIDs: labels.Stations,
		Transfers:  labels.Transfers,
	}
	if g.station.Transfers == nil {
		g.station.Transfers = map[string]string{}
	}

	// ホームからホームへは、改札の点を通らない（改札を出て別の改札から入り直す経路にしない）。
	// ネットワークには改札の内外の区別が無いので、改札の点を通る線を除いて探す
	inside := nw.adjacency(gateSet)
	adj := nw.adjacency(nil)

	// 行き先: 別のホームの番線（ホームごとに1回計算する）
	for _, to := range g.platforms {

		r := shortestTo(inside, to.nodes)

		for _, from := range g.sides {
			if from.platform == to {
				// 同じホームの乗り継ぎは案内しない
				continue
			}
			for _, ts := range to.label.Sides {
				e, ok := g.entry(from, r)
				if !ok {
					return nil, fmt.Errorf("track %s → %s: unreachable", from.Track, ts.Track)
				}
				e.Target = guide.Target{Railway: ts.Railway, Direction: ts.Direction, Track: ts.Track}
				g.station.Entries = append(g.station.Entries, e)
			}
		}
	}

	// 行き先: 改札
	for _, gate := range labels.Gates {

		r := shortestTo(adj, []int{g.gateNodes[gate.Name]})

		for _, from := range g.sides {
			e, ok := g.entry(from, r)
			if !ok {
				return nil, fmt.Errorf("track %s → %s: unreachable", from.Track, gate.Name)
			}
			e.Target = guide.Target{Gate: gate.Name}
			g.station.Entries = append(g.station.Entries, e)
		}
	}

	return g, nil
}

// entry は、from のホームで降りて、r の行き先へ歩くときに乗るとよい車両と、歩く距離・時間を求める。
func (g *stationGuide) entry(from guideSide, r route) (guide.Entry, bool) {

	cars := g.labels.Cars[from.Railway]
	if from.Stop == nil {
		cars = 0
	}
	p := from.platform

	// 前の端から何 m かを、ホームの点ごとに
	pos := make([]float64, len(p.nodes))
	for i, n := range p.nodes {
		x, y := g.proj.xy(g.nw.nodes[n].lon, g.nw.nodes[n].lat)
		pos[i] = p.axis.fromFront(p.axis.project(x, y), from.frontHigh)
	}

	best := struct {
		car   int
		node  int
		along float64 // ホームの上を車両から点まで歩く距離
		cost  float64
	}{node: -1, cost: math.Inf(1)}

	consider := func(car int, center float64) {
		for i, n := range p.nodes {
			if math.IsInf(r.cost[n], 1) {
				continue
			}
			along := 0.0
			if car > 0 {
				along = math.Abs(center - pos[i])
			}
			if c := along + r.cost[n]; c < best.cost {
				best.car, best.node, best.along, best.cost = car, n, along, c
			}
		}
	}

	if cars > 0 {
		for k := 1; k <= cars; k++ {
			consider(k, *from.Stop+(float64(k)-0.5)*guide.CarMeters)
		}
	} else {
		// 止まる位置か両数が分からなければ、ホームの上の最も近い点から歩くとする
		consider(0, 0)
	}

	if best.node < 0 {
		return guide.Entry{}, false
	}

	e := guide.Entry{
		Railway:     from.Railway,
		Direction:   from.Direction,
		Via:         r.viaFrom(best.node),
		WalkMeters:  int(math.Round(best.along + r.meters[best.node])),
		WalkMinutes: int(math.Ceil(best.cost / guide.WalkMetersPerMinute)),
	}
	if cars > 0 {
		e.CarFromFront = best.car
		e.Cars = cars
	}

	return e, true
}

// frontHigh は、s の列車が進む方向の前の端が、ホームの軸の hi の側かを返す。
// 対応表に front があればその向き、無ければ隣の駅への向きで決める。隣の駅への向きがホームの軸と
// 直角に近いと、軸の少しの傾きで前後が逆になるので、エラーにして front を書いてもらう。
func (g *stationGuide) frontHigh(p *guidePlatform, s labelSide, lines *jrLines, stationName string) (bool, error) {

	if s.Front != "" {
		v := compassXY[s.Front]
		cos := v[0]*p.axis.ux + v[1]*p.axis.uy
		if math.Abs(cos) < minFrontCos {
			return false, fmt.Errorf("front %s is nearly perpendicular to the platform", s.Front)
		}
		return cos > 0, nil
	}

	c, ahead, err := lines.neighborStation(s.Railway, s.Direction, stationName)
	if err != nil {
		return false, err
	}

	nx, ny := g.proj.xy(c[0], c[1])
	u := p.axis.project(nx, ny)
	dist := math.Hypot(nx-p.axis.cx, ny-p.axis.cy)

	if dist == 0 || math.Abs(u)/dist < minFrontCos {
		return false, fmt.Errorf("the neighbor station is nearly perpendicular to the platform (cos %.2f): set front", math.Abs(u)/max(dist, 1))
	}

	return (u > 0) == ahead, nil
}

// =========================
// 確認用の SVG
// =========================

// preview は、ネットワーク・ホーム（番線と前の端）・改札を描いた SVG を返す。対応表が正しいかを目で確かめるためのもの。
func (g *stationGuide) preview() []byte {

	const scale = 2.0 // 1m = 2px
	const pad = 40.0

	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, n := range g.nw.nodes {
		x, y := g.proj.xy(n.lon, n.lat)
		minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
	}

	sx := func(x float64) float64 { return (x-minX)*scale + pad }
	sy := func(y float64) float64 { return (maxY-y)*scale + pad } // 北を上に

	var b strings.Builder

	w, h := (maxX-minX)*scale+2*pad, (maxY-minY)*scale+2*pad
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" font-family="'Yu Gothic', Meiryo, 'Noto Sans CJK JP', sans-serif">`+"\n", w, h)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="#fff"/>`+"\n")

	floorColor := map[float64]string{-2: "#9b59b6", -1: "#2980b9", 1: "#7f8c8d", 2: "#e67e22"}

	for _, l := range g.nw.links {
		a, c := g.nw.nodes[l.from], g.nw.nodes[l.to]
		x1, y1 := g.proj.xy(a.lon, a.lat)
		x2, y2 := g.proj.xy(c.lon, c.lat)
		color := floorColor[a.floor]
		if color == "" {
			color = "#bbb"
		}
		if l.routeType != routeFlat {
			color = "#c0392b"
		}
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1" opacity="0.6"/>`+"\n", sx(x1), sy(y1), sx(x2), sy(y2), color)
	}

	for _, p := range g.platforms {
		var pts []string
		for _, v := range p.ring {
			x, y := g.proj.xy(v[0], v[1])
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", sx(x), sy(y)))
		}
		fmt.Fprintf(&b, `<polygon points="%s" fill="#2ecc71" fill-opacity="0.25" stroke="#27ae60"/>`+"\n", strings.Join(pts, " "))
	}

	for i, s := range g.sides {
		a := s.platform.axis
		end := a.lo
		if s.frontHigh {
			end = a.hi
		}
		// 前の端に、番線と路線の向きの印を付ける（同じホームの2つ目の番線は少しずらす）
		x, y := a.cx+a.ux*end, a.cy+a.uy*end
		off := float64(i%2) * 14
		name := idName(s.Railway) + " " + idName(s.Direction)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="#27ae60"/>`+"\n", sx(x), sy(y))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="12" fill="#145a32">%s番線 %s ▶前</text>`+"\n", sx(x)+6, sy(y)+off, s.Track, name)
	}

	for _, gate := range g.labels.Gates {
		n := g.nw.nodes[g.gateNodes[gate.Name]]
		x, y := g.proj.xy(n.lon, n.lat)
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="5" fill="#e74c3c"/>`+"\n", sx(x), sy(y))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="13" font-weight="bold" fill="#922b21">%s（%v階）</text>`+"\n", sx(x)+7, sy(y)-6, gate.Name, n.floor)
	}

	b.WriteString("</svg>\n")

	return []byte(b.String())
}

// idName は ODPT の ID の最後の名前を返す（odpt.RailDirection:OuterLoop → OuterLoop）。
func idName(id string) string {
	_, after, _ := strings.Cut(id, ":")
	return lastSegment(after)
}
