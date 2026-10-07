// gen-assets は、assets の ODPT の JSON から軽量な gob を生成する。
//
//   - -kind station: station_timetable.json → station_timetable.gob
//   - -kind train:   train_timetable.json → train_timetable.gob
//
// 通常は assets ディレクトリで go generate から実行する:
//
//	cd backend && go generate ./assets
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"os"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

func main() {
	kind := flag.String("kind", "station", "station (station timetable) or train (train timetable)")
	in := flag.String("in", "station_timetable.json", "input ODPT timetable JSON")
	out := flag.String("out", "station_timetable.gob", "output slim timetable")
	flag.Parse()

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
