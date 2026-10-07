// gen-assets は、assets/station_timetable.json から軽量な station_timetable.gob を生成する。
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
	in := flag.String("in", "station_timetable.json", "input ODPT station timetable JSON")
	out := flag.String("out", "station_timetable.gob", "output slim timetable")
	flag.Parse()

	data, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}

	var timetables []model.StationTimetable

	if err := json.Unmarshal(data, &timetables); err != nil {
		log.Fatalf("%s: %v", *in, err)
	}

	var buf bytes.Buffer

	if err := slim.Encode(&buf, timetables); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"%s: %d timetables, %d bytes",
		*out,
		len(timetables),
		buf.Len(),
	)
}
