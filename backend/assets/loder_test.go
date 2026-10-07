package assets

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

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
