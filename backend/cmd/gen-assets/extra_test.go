package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

func TestClipTrain(t *testing.T) {

	area := map[string]bool{"in1": true, "in2": true, "in3": true}

	stop := func(station, dep string) model.TrainTimetableEntry {
		return model.TrainTimetableEntry{DepartureStation: station, DepartureTime: dep}
	}

	tt := model.TrainTimetable{
		DestinationStation: []string{"out2", "other"},
		TrainTimetableObject: []model.TrainTimetableEntry{
			stop("out1", "10:00"),
			stop("in1", "10:05"),
			stop("in2", ""), // 時刻が無い（通過）
			stop("out-between", "10:10"),
			stop("in3", "10:15"),
			{ArrivalStation: "out2", ArrivalTime: "10:20"},
		},
	}

	got, dropped, ok := clipTrain(tt, area)

	if !ok || dropped != 1 {
		t.Fatalf("expected ok and 1 dropped, got %v %d", ok, dropped)
	}

	// 都内の最初（in1）から最後（in3）まで。途中の都外の停車は残す
	want := []model.TrainTimetableEntry{
		stop("in1", "10:05"),
		stop("out-between", "10:10"),
		stop("in3", "10:15"),
	}
	if !reflect.DeepEqual(got.TrainTimetableObject, want) {
		t.Fatalf("expected %+v, got %+v", want, got.TrainTimetableObject)
	}

	if !reflect.DeepEqual(got.DestinationStation, []string{"out2"}) {
		t.Fatalf("expected the first destination, got %v", got.DestinationStation)
	}

	// 入力は書き換えない
	if len(tt.TrainTimetableObject) != 6 || len(tt.DestinationStation) != 2 {
		t.Fatal("input was modified")
	}

	// 都内の停車が1つだけの列車は残さない
	if _, _, ok := clipTrain(model.TrainTimetable{
		TrainTimetableObject: []model.TrainTimetableEntry{stop("out1", "10:00"), stop("in1", "10:05")},
	}, area); ok {
		t.Fatal("expected a train with one stop in Tokyo to be dropped")
	}
}

func TestGenExtra(t *testing.T) {

	raw := t.TempDir()
	out := t.TempDir()

	area := filepath.Join(raw, "tokyo_stations.txt")
	writeFile(t, area, "# test\nodpt.Station:Test.Line.A\nodpt.Station:Test.Line.B\nodpt.Station:Toei.Line.A\n")

	writeFile(t, filepath.Join(raw, "operators", "Test", "Station.json"), `[
		{"owl:sameAs": "odpt.Station:Test.Line.A", "odpt:operator": "odpt.Operator:Test", "odpt:connectingStation": ["odpt.Station:Toei.Line.A"]},
		{"owl:sameAs": "odpt.Station:Test.Line.B", "odpt:operator": "odpt.Operator:Test"},
		{"owl:sameAs": "odpt.Station:Test.Line.Outside", "odpt:operator": "odpt.Operator:Test", "odpt:stationTitle": {"ja": "都外"}}
	]`)
	writeFile(t, filepath.Join(raw, "operators", "Test", "Railway.json"), `[
		{"owl:sameAs": "odpt.Railway:Test.Line", "odpt:operator": "odpt.Operator:Test", "odpt:stationOrder": [{"odpt:station": "odpt.Station:Test.Line.A"}]},
		{"owl:sameAs": "odpt.Railway:Test.Outside", "odpt:operator": "odpt.Operator:Test", "odpt:stationOrder": [{"odpt:station": "odpt.Station:Test.Line.Outside"}]}
	]`)
	writeFile(t, filepath.Join(raw, "operators", "Test", "TrainType.json"), `[
		{"owl:sameAs": "odpt.TrainType:Test.Local", "odpt:operator": "odpt.Operator:Test"}
	]`)
	writeFile(t, filepath.Join(raw, "operators", "Other", "TrainType.json"), `[
		{"owl:sameAs": "odpt.TrainType:Other.Local", "odpt:operator": "odpt.Operator:Other"}
	]`)

	writeFile(t, filepath.Join(raw, "dumps", "basic", "StationTimetable.json"), `[
		{"odpt:operator": "odpt.Operator:Test", "odpt:station": "odpt.Station:Test.Line.A", "odpt:stationTimetableObject": [
			{"odpt:departureTime": "10:00", "odpt:destinationStation": ["odpt.Station:Test.Line.Outside"]},
			{"odpt:departureTime": "10:10", "odpt:destinationStation": ["odpt.Station:Test.Line.B", "odpt.Station:Keisei.Main.Unknown"]}
		]},
		{"odpt:operator": "odpt.Operator:Test", "odpt:station": "odpt.Station:Test.Line.Outside", "odpt:stationTimetableObject": []},
		{"odpt:operator": "odpt.Operator:Toei", "odpt:station": "odpt.Station:Toei.Line.A", "odpt:stationTimetableObject": []}
	]`)
	writeFile(t, filepath.Join(raw, "dumps", "basic", "TrainTimetable.json"), `[
		{"odpt:operator": "odpt.Operator:Test", "odpt:train": "odpt.Train:Test.Line.1", "odpt:trainTimetableObject": [
			{"odpt:departureStation": "odpt.Station:Test.Line.A", "odpt:departureTime": "10:00"},
			{"odpt:arrivalStation": "odpt.Station:Test.Line.B", "odpt:arrivalTime": "10:03"}
		]},
		{"odpt:operator": "odpt.Operator:Toei", "odpt:train": "odpt.Train:Toei.Line.1", "odpt:trainTimetableObject": [
			{"odpt:departureStation": "odpt.Station:Toei.Line.A", "odpt:departureTime": "10:00"},
			{"odpt:arrivalStation": "odpt.Station:Test.Line.B", "odpt:arrivalTime": "10:03"}
		]}
	]`)

	if err := genExtra(raw, area, out); err != nil {
		t.Fatal(err)
	}

	ids := func(name string) []string {
		var items []map[string]any
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &items); err != nil {
			t.Fatal(err)
		}
		var result []string
		for _, item := range items {
			result = append(result, item["owl:sameAs"].(string))
		}
		return result
	}

	for name, want := range map[string][]string{
		"station.json":    {"odpt.Station:Test.Line.A", "odpt.Station:Test.Line.B"},
		"railway.json":    {"odpt.Railway:Test.Line"},
		"train_type.json": {"odpt.TrainType:Test.Local"},

		// 都外の行先だけ。都内の駅（B）と、駅のデータに無い駅は含めない
		"destination_station.json": {"odpt.Station:Test.Line.Outside"},
	} {
		if got := ids(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: expected %v, got %v", name, want, got)
		}
	}

	// model に無い項目（乗り換え先の駅）も元のまま残す
	data, err := os.ReadFile(filepath.Join(out, "station.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("odpt:connectingStation")) {
		t.Error("expected odpt:connectingStation to be kept")
	}

	// 都外の行先駅は、駅ID と駅名だけを残す
	data, err = os.ReadFile(filepath.Join(out, "destination_station.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `[{"owl:sameAs":"odpt.Station:Test.Line.Outside","odpt:stationTitle":{"ja":"都外"}}]`; string(data) != want {
		t.Errorf("destination_station.json: expected %s, got %s", want, data)
	}

	f, err := os.Open(filepath.Join(out, "station_timetable.gob"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	stationTimetables, err := slim.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(stationTimetables) != 1 || stationTimetables[0].Station != "odpt.Station:Test.Line.A" {
		t.Errorf("unexpected station timetables %+v", stationTimetables)
	}

	g, err := os.Open(filepath.Join(out, "train_timetable.gob"))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	trains, err := slim.DecodeTrainTimetables(g)
	if err != nil {
		t.Fatal(err)
	}
	if len(trains.Trains) != 1 || trains.String(trains.Trains[0].Train) != "odpt.Train:Test.Line.1" {
		t.Errorf("expected only the Test train (Toei is excluded), got %d trains", len(trains.Trains))
	}
}

func TestCheckDuplicates(t *testing.T) {

	trains := []model.TrainTimetable{
		{SameAs: "odpt.TrainTimetable:Test.Line.1.Weekday"},
		{SameAs: "odpt.TrainTimetable:Test.Line.2.Weekday"},
	}

	if err := checkDuplicates(nil, trains); err != nil {
		t.Fatal(err)
	}

	trains = append(trains, model.TrainTimetable{SameAs: "odpt.TrainTimetable:Test.Line.1.Weekday"})

	if err := checkDuplicates(nil, trains); err == nil {
		t.Fatal("expected duplicate error")
	}
}
