package main

import (
	"reflect"
	"testing"

	"train-status-app/backend/internal/infer"
	"train-status-app/backend/internal/model"
)

func TestEstimateTrains(t *testing.T) {

	railways := []infer.RawRailway{{
		SameAs:    "odpt.Railway:Test.Line",
		Operator:  "odpt.Operator:Test",
		Ascending: "odpt.RailDirection:Outbound",
		StationOrder: []infer.StationOrder{
			{Index: 1, Station: "odpt.Station:Test.Line.A"},
			{Index: 2, Station: "odpt.Station:Test.Line.B"},
			{Index: 3, Station: "odpt.Station:Test.Line.C"},
		},
	}}

	st := func(station, time, dest string) infer.RawStationTimetable {
		tt := infer.RawStationTimetable{
			Operator: "odpt.Operator:Test", Railway: "odpt.Railway:Test.Line", Station: station,
			Calendar: "odpt.Calendar:Weekday", RailDirection: "odpt.RailDirection:Outbound",
		}
		tt.Objects = append(tt.Objects, struct {
			DepartureTime string   `json:"odpt:departureTime"`
			Train         string   `json:"odpt:train"`
			TrainType     string   `json:"odpt:trainType"`
			Destination   []string `json:"odpt:destinationStation"`
			IsOrigin      bool     `json:"odpt:isOrigin"`
		}{DepartureTime: time, TrainType: "odpt.TrainType:Test.Local", Destination: []string{dest}})
		return tt
	}

	// 1本目は C 行き（C の到着を見込む）。2本目は直通先の Other.Z 行きで、C とつながる Other.C から 0:35 に出る
	// 列車へ渡すので、C まで延ばす（24時をまたぐ）
	timetables := []infer.RawStationTimetable{
		st("odpt.Station:Test.Line.A", "10:00", "odpt.Station:Test.Line.C"),
		st("odpt.Station:Test.Line.B", "10:02", "odpt.Station:Test.Line.C"),
		st("odpt.Station:Test.Line.A", "00:30", "odpt.Station:Other.Line.Z"),
		st("odpt.Station:Test.Line.B", "00:32", "odpt.Station:Other.Line.Z"),
	}

	starts := map[string]map[string][]infer.Partner{
		"odpt.Calendar:Weekday": {
			"odpt.Station:Other.Line.C": {{Station: "odpt.Station:Other.Line.C", Minutes: 24*60 + 35, Destination: "odpt.Station:Other.Line.Z"}},
		},
	}
	connect := map[string][]string{"odpt.Station:Test.Line.C": {"odpt.Station:Other.Line.C"}}

	got := estimateTrains(railways, timetables, func(string) bool { return true }, starts, connect)

	stop := func(dep, depTime, arr, arrTime string) model.TrainTimetableEntry {
		return model.TrainTimetableEntry{DepartureStation: dep, DepartureTime: depTime, ArrivalStation: arr, ArrivalTime: arrTime}
	}
	train := func(n, dest string, stops ...model.TrainTimetableEntry) model.TrainTimetable {
		id := "Test.Line.Estimated.Weekday.Outbound." + n
		return model.TrainTimetable{
			SameAs: "odpt.TrainTimetable:" + id, Train: "odpt.Train:" + id,
			Railway: "odpt.Railway:Test.Line", Operator: "odpt.Operator:Test",
			Calendar: "odpt.Calendar:Weekday", RailDirection: "odpt.RailDirection:Outbound",
			TrainType: "odpt.TrainType:Test.Local", DestinationStation: []string{dest},
			TrainTimetableObject: stops,
		}
	}

	want := []model.TrainTimetable{
		train("0001", "odpt.Station:Test.Line.C",
			stop("odpt.Station:Test.Line.A", "10:00", "", ""),
			stop("odpt.Station:Test.Line.B", "10:02", "", ""),
			stop("", "", "odpt.Station:Test.Line.C", "10:04")),
		train("0002", "odpt.Station:Other.Line.Z",
			stop("odpt.Station:Test.Line.A", "00:30", "", ""),
			stop("odpt.Station:Test.Line.B", "00:32", "", ""),
			stop("", "", "odpt.Station:Test.Line.C", "00:35")),
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}

	for _, tt := range got {
		if !infer.IsEstimated(tt.Train) {
			t.Errorf("%s is not marked as estimated", tt.Train)
		}
	}
	if infer.IsEstimated("odpt.Train:Toei.Mita.954T") {
		t.Error("a real train must not be marked as estimated")
	}
}

// 途中の駅で種別が変わる列車は、停車駅ごとに種別を入れる。変わらない列車には入れない
func TestEstimateTrainsTypeChange(t *testing.T) {

	railways := []infer.RawRailway{{
		SameAs:    "odpt.Railway:Test.Line",
		Operator:  "odpt.Operator:Test",
		Ascending: "odpt.RailDirection:Outbound",
		StationOrder: []infer.StationOrder{
			{Index: 1, Station: "odpt.Station:Test.Line.A"},
			{Index: 2, Station: "odpt.Station:Test.Line.B"},
			{Index: 3, Station: "odpt.Station:Test.Line.C"},
			{Index: 4, Station: "odpt.Station:Test.Line.D"},
		},
	}}

	st := func(station, time, trainType string) infer.RawStationTimetable {
		tt := infer.RawStationTimetable{
			Operator: "odpt.Operator:Test", Railway: "odpt.Railway:Test.Line", Station: "odpt.Station:Test.Line." + station,
			Calendar: "odpt.Calendar:Weekday", RailDirection: "odpt.RailDirection:Outbound",
		}
		tt.Objects = append(tt.Objects, struct {
			DepartureTime string   `json:"odpt:departureTime"`
			Train         string   `json:"odpt:train"`
			TrainType     string   `json:"odpt:trainType"`
			Destination   []string `json:"odpt:destinationStation"`
			IsOrigin      bool     `json:"odpt:isOrigin"`
		}{DepartureTime: time, TrainType: "odpt.TrainType:Test." + trainType, Destination: []string{"odpt.Station:Other.Line.Z"}})
		return tt
	}

	// 10時の急行は C から各停になる。11時の各停は全区間が各停
	timetables := []infer.RawStationTimetable{
		st("A", "10:00", "Express"), st("B", "10:05", "Express"), st("C", "10:08", "Local"), st("D", "10:10", "Local"),
		st("A", "11:00", "Local"), st("B", "11:03", "Local"), st("C", "11:06", "Local"), st("D", "11:08", "Local"),
	}

	got := estimateTrains(railways, timetables, func(string) bool { return true }, nil, nil)

	types := func(tt model.TrainTimetable) []string {
		var result []string
		for _, e := range tt.TrainTimetableObject {
			result = append(result, e.TrainType)
		}
		return result
	}

	if len(got) != 2 {
		t.Fatalf("got %d trains: %+v", len(got), got)
	}
	if want := []string{"odpt.TrainType:Test.Express", "odpt.TrainType:Test.Express", "odpt.TrainType:Test.Local", "odpt.TrainType:Test.Local"}; got[0].TrainType != want[0] || !reflect.DeepEqual(types(got[0]), want) {
		t.Errorf("train 1: type %s, stops %v", got[0].TrainType, types(got[0]))
	}
	if want := []string{"", "", "", ""}; !reflect.DeepEqual(types(got[1]), want) {
		t.Errorf("train 2: stops %v", types(got[1]))
	}
}
