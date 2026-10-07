package slim

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"testing"

	"train-status-app/backend/internal/model"
)

func TestEncodeDecode(t *testing.T) {

	input := []model.StationTimetable{
		{
			SameAs:        "odpt.StationTimetable:Toei.Mita.Kasuga.Southbound.Weekday",
			Railway:       "odpt.Railway:Toei.Mita",
			Station:       "odpt.Station:Toei.Mita.Kasuga",
			Calendar:      "odpt.Calendar:Weekday",
			RailDirection: "odpt.RailDirection:Southbound",
			StationTimetableObject: []model.StationTimetableEntry{
				{
					DepartureTime:      "05:15",
					Train:              "odpt.Train:Toei.Mita.501T",
					TrainType:          "odpt.TrainType:Toei.Local",
					TrainNumber:        "501T",
					DestinationStation: []string{"odpt.Station:Tokyu.Meguro.Hiyoshi"},
				},
				{
					// 行先が無い（大江戸線の環状部など）
					ArrivalTime: "05:20",
					Train:       "odpt.Train:Toei.Mita.503T",
					TrainType:   "odpt.TrainType:Toei.Local",
					TrainNumber: "503T",
				},
			},
		},
		{
			Railway:                "odpt.Railway:Toei.Mita",
			Station:                "odpt.Station:Toei.Mita.Hakusan",
			Calendar:               "odpt.Calendar:SaturdayHoliday",
			RailDirection:          "odpt.RailDirection:Northbound",
			StationTimetableObject: []model.StationTimetableEntry{},
		},
	}

	var buf bytes.Buffer

	if err := Encode(&buf, input); err != nil {
		t.Fatal(err)
	}

	got, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	// 時刻表単位の識別子（owl:sameAs など）は保持しない
	want := []model.StationTimetable{input[0], input[1]}
	want[0].SameAs = ""

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestEncodeMultipleDestinations(t *testing.T) {

	input := []model.StationTimetable{
		{
			StationTimetableObject: []model.StationTimetableEntry{
				{
					DestinationStation: []string{"a", "b"},
				},
			},
		},
	}

	err := Encode(&bytes.Buffer{}, input)

	if err == nil || !strings.Contains(err.Error(), "multiple destinations") {
		t.Fatalf("expected multiple destinations error, got %v", err)
	}
}

func TestDecodeInvalidIndex(t *testing.T) {

	var buf bytes.Buffer

	err := gob.NewEncoder(&buf).Encode(StationTimetables{
		Strings: []string{""},
		Timetables: []Timetable{
			{Railway: 5},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Decode(&buf); err == nil {
		t.Fatal("expected out of range error")
	}
}
