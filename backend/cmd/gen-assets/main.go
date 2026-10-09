// gen-assets は、assets の ODPT の JSON から軽量な gob を生成する。
//
//   - -kind station: station_timetable.json → station_timetable.gob
//   - -kind train:   train_timetable.json → train_timetable.gob
//   - -kind area:    区市町村の境界と駅の座標 → 都内の駅の一覧（tokyo_stations.txt）
//   - -kind extra:   都営以外の事業者の ODPT の JSON → 都内に絞った assets/extra
//
// station・train は、通常は assets ディレクトリで go generate から実行する:
//
//	cd backend && go generate ./assets
//
// area・extra は scripts/update_assets.sh から実行する（ODPT のキーが要る）。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

func main() {
	kind := flag.String("kind", "station", "station (station timetable), train (train timetable), area (Tokyo station list) or extra (other operators)")
	in := flag.String("in", "station_timetable.json", "input ODPT timetable JSON (station, train)")
	out := flag.String("out", "station_timetable.gob", "output file (station, train, area) or directory (extra)")
	geojson := flag.String("geojson", "", "municipal boundaries GeoJSON of Tokyo, N03 of the National Land Numerical Information (area)")
	stationFiles := flag.String("stations", "", "comma-separated ODPT station JSON files (area)")
	raw := flag.String("raw", "", "directory of ODPT JSON fetched by scripts/update_assets.sh (extra)")
	areaPath := flag.String("area", "tokyo_stations.txt", "Tokyo station list (extra)")
	flag.Parse()

	switch *kind {
	case "area":
		if err := runArea(*geojson, *stationFiles, *out); err != nil {
			log.Fatal(err)
		}
		return

	case "extra":
		if err := genExtra(*raw, *areaPath, *out); err != nil {
			log.Fatal(err)
		}
		return
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	var count int

	switch *kind {
	case "station":
		var timetables []model.StationTimetable

		if err := json.Unmarshal(data, &timetables); err != nil {
			log.Fatalf("%s: %v", *in, err)
		}

		if err := slim.Encode(&buf, timetables); err != nil {
			log.Fatal(err)
		}

		count = len(timetables)

	case "train":
		var timetables []model.TrainTimetable

		if err := json.Unmarshal(data, &timetables); err != nil {
			log.Fatalf("%s: %v", *in, err)
		}

		if err := slim.EncodeTrainTimetables(&buf, timetables); err != nil {
			log.Fatal(err)
		}

		count = len(timetables)

	default:
		log.Fatalf("unknown kind %q", *kind)
	}

	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"%s: %d timetables, %d bytes",
		*out,
		count,
		buf.Len(),
	)
}

func runArea(geojson, stationFiles, out string) error {

	polygons, err := loadPolygons(geojson)
	if err != nil {
		return err
	}

	stations, err := tokyoStations(polygons, strings.Split(stationFiles, ","))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := writeArea(&buf, stations); err != nil {
		return err
	}

	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}

	log.Printf("%s: %d stations", out, len(stations))

	return nil
}
