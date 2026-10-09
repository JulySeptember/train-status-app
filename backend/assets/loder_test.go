package assets

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

// 埋め込んだ station_timetable.gob が、station_timetable.json と同じ内容
// （アプリが使う項目について）であること。
// json を更新して go generate を忘れた場合に検出する。
func TestStationTimetablesMatchJSON(t *testing.T) {

	data, err := os.ReadFile("station_timetable.json")
	if err != nil {
		t.Fatal(err)
	}

	var want []model.StationTimetable

	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}

	// 軽量化で捨てる、時刻表単位の項目
	for i := range want {
		tt := &want[i]
		tt.ID = ""
		tt.Type = ""
		tt.Context = ""
		tt.Date = ""
		tt.Issued = ""
		tt.SameAs = ""
		tt.Operator = ""
	}

	l, err := New()
	if err != nil {
		t.Fatal(err)
	}

	got := l.StationTimetables()

	if len(got) != len(want) {
		t.Fatalf("expected %d timetables, got %d", len(want), len(got))
	}

	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf(
				"timetable %d (%s) differs: run go generate ./assets",
				i,
				want[i].Station,
			)
		}
	}
}

// 埋め込んだ train_timetable.gob が、train_timetable.json から生成した内容と同じであること。
// json を更新して go generate を忘れた場合に検出する。
func TestTrainTimetablesMatchJSON(t *testing.T) {

	data, err := os.ReadFile("train_timetable.json")
	if err != nil {
		t.Fatal(err)
	}

	var timetables []model.TrainTimetable

	if err := json.Unmarshal(data, &timetables); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	if err := slim.EncodeTrainTimetables(&buf, timetables); err != nil {
		t.Fatal(err)
	}

	want, err := slim.DecodeTrainTimetables(&buf)
	if err != nil {
		t.Fatal(err)
	}

	l, err := New()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(l.TrainTimetables(), want) {
		t.Fatal("train_timetable.gob differs from train_timetable.json: run go generate ./assets")
	}
}

// testFS は、埋め込んだ都営のデータに、extra/ のファイルを足した fs.FS を返す。
func testFS(t *testing.T, extra map[string][]byte) fstest.MapFS {
	t.Helper()

	fsys := fstest.MapFS{}

	err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(path, extraDir+"/") {
			return err
		}
		data, err := fs.ReadFile(embedded, path)
		if err != nil {
			return err
		}
		fsys[path] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	for name, data := range extra {
		fsys[extraDir+"/"+name] = &fstest.MapFile{Data: data}
	}

	return fsys
}

// extraTestFiles は、架空の他社の路線・駅・時刻表を1つずつ持つ extra/ のファイルを返す。
func extraTestFiles(t *testing.T) map[string][]byte {
	t.Helper()

	marshal := func(v any) []byte {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	var stationTimetable bytes.Buffer
	err := slim.Encode(&stationTimetable, []model.StationTimetable{
		{
			Railway:  "odpt.Railway:Test.Line",
			Station:  "odpt.Station:Test.Line.A",
			Calendar: "odpt.Calendar:Weekday",
			StationTimetableObject: []model.StationTimetableEntry{
				{DepartureTime: "10:00", DestinationStation: []string{"odpt.Station:Test.Line.B"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var trainTimetable bytes.Buffer
	err = slim.EncodeTrainTimetables(&trainTimetable, []model.TrainTimetable{
		{
			Train:    "odpt.Train:Test.Line.1",
			Calendar: "odpt.Calendar:Weekday",
			TrainTimetableObject: []model.TrainTimetableEntry{
				{DepartureTime: "10:00", DepartureStation: "odpt.Station:Test.Line.A"},
				{ArrivalTime: "10:03", ArrivalStation: "odpt.Station:Test.Line.B"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return map[string][]byte{
		"railway.json":          marshal([]model.Railway{{SameAs: "odpt.Railway:Test.Line", Operator: "odpt.Operator:Test"}}),
		"station.json":          marshal([]model.Station{{SameAs: "odpt.Station:Test.Line.A", Operator: "odpt.Operator:Test"}}),
		"train_type.json":       marshal([]model.TrainType{{SameAs: "odpt.TrainType:Test.Local", Operator: "odpt.Operator:Test"}}),
		"station_timetable.gob": stationTimetable.Bytes(),
		"train_timetable.gob":   trainTimetable.Bytes(),
	}
}

func TestLoadExtra(t *testing.T) {

	base, err := New()
	if err != nil {
		t.Fatal(err)
	}

	fsys := testFS(t, extraTestFiles(t))

	t.Run("without option", func(t *testing.T) {
		l, err := newFromFS(fsys)
		if err != nil {
			t.Fatal(err)
		}
		if l.HasExtra() || len(l.Stations()) != len(base.Stations()) {
			t.Fatal("extra data should not be loaded without WithExtra")
		}
	})

	t.Run("with option", func(t *testing.T) {
		l, err := newFromFS(fsys, WithExtra())
		if err != nil {
			t.Fatal(err)
		}

		if !l.HasExtra() {
			t.Fatal("expected extra data to be loaded")
		}

		for name, pair := range map[string][2]int{
			"railways":           {len(l.Railways()), len(base.Railways()) + 1},
			"stations":           {len(l.Stations()), len(base.Stations()) + 1},
			"train types":        {len(l.TrainTypes()), len(base.TrainTypes()) + 1},
			"station timetables": {len(l.StationTimetables()), len(base.StationTimetables()) + 1},
			"trains":             {len(l.TrainTimetables().Trains), len(base.TrainTimetables().Trains) + 1},
		} {
			if pair[0] != pair[1] {
				t.Errorf("%s: expected %d, got %d", name, pair[1], pair[0])
			}
		}

		tt := l.TrainTimetables()
		last := tt.Trains[len(tt.Trains)-1]
		if tt.String(last.Train) != "odpt.Train:Test.Line.1" {
			t.Errorf("expected the extra train last, got %s", tt.String(last.Train))
		}
		if tt.String(tt.Trains[0].Train) != base.TrainTimetables().String(base.TrainTimetables().Trains[0].Train) {
			t.Error("Toei trains should come first")
		}
	})

	t.Run("no extra files", func(t *testing.T) {
		l, err := newFromFS(testFS(t, nil), WithExtra())
		if err != nil {
			t.Fatal(err)
		}
		if l.HasExtra() {
			t.Fatal("expected no extra data")
		}
	})

	t.Run("missing some files", func(t *testing.T) {
		files := extraTestFiles(t)
		delete(files, "train_timetable.gob")

		_, err := newFromFS(testFS(t, files), WithExtra())
		if err == nil || !strings.Contains(err.Error(), "train_timetable.gob") {
			t.Fatalf("expected missing file error, got %v", err)
		}
	})
}
