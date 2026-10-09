package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// areaHeader は、都内の駅の一覧（-kind area の出力）の先頭に書く説明。
const areaHeader = `# 東京都内（島しょ部を除く）の駅。go run ./cmd/gen-assets -kind area で生成する（scripts/update_assets.sh）。
# 駅の座標（ODPT の geo:lat / geo:long）が区市町村の境界の中にある駅。
# 境界は「国土数値情報（行政区域データ）」（国土交通省）を加工して作成。
`

// boundaries は、国土数値情報（行政区域 N03）の GeoJSON の必要な部分。
type boundaries struct {
	Features []struct {
		Properties struct {
			Code string `json:"N03_007"`
		} `json:"properties"`
		Geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

// ring は多角形の輪郭1つ。点は [経度, 緯度]。
type ring [][2]float64

// polygon は外側の輪郭と穴（GeoJSON の Polygon と同じ並び）。
type polygon []ring

// inTokyo は、区市町村コードが対象（23区・市・西多摩の町村）かを返す。
// 13000 は所属未定地、1336x 以降は島しょ部。
func inTokyo(code string) bool {
	return len(code) == 5 && strings.HasPrefix(code, "13") && code > "13100" && code < "13360"
}

// loadPolygons は、GeoJSON から対象の区市町村の多角形を読み込む。
func loadPolygons(path string) ([]polygon, error) {

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var b boundaries
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var result []polygon

	for _, f := range b.Features {

		if !inTokyo(f.Properties.Code) {
			continue
		}

		switch f.Geometry.Type {
		case "Polygon":
			var p polygon
			if err := json.Unmarshal(f.Geometry.Coordinates, &p); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			result = append(result, p)

		case "MultiPolygon":
			var ps []polygon
			if err := json.Unmarshal(f.Geometry.Coordinates, &ps); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			result = append(result, ps...)

		default:
			return nil, fmt.Errorf("%s: unsupported geometry %s", path, f.Geometry.Type)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%s: no polygons in Tokyo", path)
	}

	return result, nil
}

// contains は、点が多角形の中にあるかを返す（偶奇規則。穴の中は外になる）。
func (p polygon) contains(lon, lat float64) bool {

	inside := false

	for _, r := range p {
		for i, j := 0, len(r)-1; i < len(r); j, i = i, i+1 {
			xi, yi := r[i][0], r[i][1]
			xj, yj := r[j][0], r[j][1]
			if (yi > lat) != (yj > lat) && lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
				inside = !inside
			}
		}
	}

	return inside
}

// stationPoint は、駅の JSON から使う項目。
type stationPoint struct {
	SameAs    string   `json:"owl:sameAs"`
	Latitude  *float64 `json:"geo:lat"`
	Longitude *float64 `json:"geo:long"`
}

// tokyoStations は、座標が都内にある駅の ID を返す（ID 順）。座標の無い駅は含めない。
func tokyoStations(polygons []polygon, stationFiles []string) ([]string, error) {

	var result []string

	for _, path := range stationFiles {

		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var stations []stationPoint
		if err := json.Unmarshal(data, &stations); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		for _, st := range stations {
			if st.Latitude == nil || st.Longitude == nil {
				continue
			}
			if slices.ContainsFunc(polygons, func(p polygon) bool {
				return p.contains(*st.Longitude, *st.Latitude)
			}) {
				result = append(result, st.SameAs)
			}
		}
	}

	slices.Sort(result)

	return slices.Compact(result), nil
}

func writeArea(w io.Writer, stations []string) error {

	if _, err := io.WriteString(w, areaHeader); err != nil {
		return err
	}

	for _, s := range stations {
		if _, err := fmt.Fprintln(w, s); err != nil {
			return err
		}
	}

	return nil
}

// readArea は、都内の駅の一覧を読み込む。# で始まる行と空行は飛ばす。
func readArea(path string) (map[string]bool, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]bool)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result[line] = true
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("%s: no stations", path)
	}

	return result, nil
}
