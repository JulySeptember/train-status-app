package slim

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"testing"

	"train-status-app/backend/internal/model"
)

func TestEncodeDecodeTrainTimetables(t *testing.T) {

	input := []model.TrainTimetable{
		{
			SameAs:             "odpt.TrainTimetable:Toei.Mita.1501T.Weekday",
			Train:              "odpt.Train:Toei.Mita.1501T",
			TrainNumber:        "1501T",
			Railway:            "odpt.Railway:Toei.Mita",
			Calendar:           "odpt.Calendar:Weekday",
			RailDirection:      "odpt.RailDirection:Southbound",
			TrainType:          "odpt.TrainType:Toei.Local",
			DestinationStation: []string{"odpt.Station:Toei.Mita.Kasuga"},
			TrainTimetableObject: []model.TrainTimetableEntry{
				{
					DepartureTime:    "23:58",
					DepartureStation: "odpt.Station:Toei.Mita.Hakusan",
					PlatformNumber:   "1",
				},
				{
					ArrivalTime:      "23:59",
					ArrivalStation:   "odpt.Station:Toei.Mita.Sengoku",
					DepartureTime:    "00:00",
					DepartureStation: "odpt.Station:Toei.Mita.Sengoku",
				},
				{
					ArrivalTime:    "00:15",
					ArrivalStation: "odpt.Station:Toei.Mita.Kasuga",
				},
			},
		},
		{
			// 行先が無い列車（大江戸線の環状部など）
			Train:    "odpt.Train:Toei.Oedo.1001A",
			Calendar: "odpt.Calendar:Weekday",
			TrainTimetableObject: []model.TrainTimetableEntry{
				{
					DepartureTime:    "05:00",
					DepartureStation: "odpt.Station:Toei.Oedo.Tochomae",
				},
			},
		},
	}

	var buf bytes.Buffer

	if err := EncodeTrainTimetables(&buf, input); err != nil {
		t.Fatal(err)
	}

	got, err := DecodeTrainTimetables(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Trains) != 2 {
		t.Fatalf("expected 2 trains, got %d", len(got.Trains))
	}

	train := got.Trains[0]

	for field, want := range map[string]struct {
		got  int32
		want string
	}{
		"Train":         {train.Train, "odpt.Train:Toei.Mita.1501T"},
		"TrainNumber":   {train.TrainNumber, "1501T"},
		"Railway":       {train.Railway, "odpt.Railway:Toei.Mita"},
		"Calendar":      {train.Calendar, "odpt.Calendar:Weekday"},
		"RailDirection": {train.RailDirection, "odpt.RailDirection:Southbound"},
		"TrainType":     {train.TrainType, "odpt.TrainType:Toei.Local"},
		"Destination":   {train.Destination, "odpt.Station:Toei.Mita.Kasuga"},
	} {
		if s := got.String(want.got); s != want.want {
			t.Errorf("%s: expected %q, got %q", field, want.want, s)
		}
	}

	type stop struct {
		station            string
		arrival, departure int16
	}

	var stops []stop
	for _, s := range train.Stops {
		stops = append(stops, stop{got.String(s.Station), s.Arrival, s.Departure})
	}

	// 0時台は翌日として 24 時間を足す
	wantStops := []stop{
		{"odpt.Station:Toei.Mita.Hakusan", NoTime, 23*60 + 58},
		{"odpt.Station:Toei.Mita.Sengoku", 23*60 + 59, 24 * 60},
		{"odpt.Station:Toei.Mita.Kasuga", 24*60 + 15, NoTime},
	}

	if !reflect.DeepEqual(stops, wantStops) {
		t.Fatalf("expected %+v, got %+v", wantStops, stops)
	}

	if s := got.String(got.Trains[1].Destination); s != "" {
		t.Fatalf("expected empty destination, got %q", s)
	}
}

func TestParseMinutes(t *testing.T) {

	tests := []struct {
		in   string
		want int16
	}{
		{"", NoTime},
		{"00:00", 24 * 60},
		{"02:59", 26*60 + 59},
		{"03:00", 3 * 60},
		{"23:59", 23*60 + 59},
	}

	for _, tt := range tests {
		got, err := parseMinutes(tt.in)
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("%q: expected %d, got %d", tt.in, tt.want, got)
		}
	}

	for _, in := range []string{"5:00", "24:00", "12:60", "1200", "ab:cd"} {
		if _, err := parseMinutes(in); err == nil {
			t.Errorf("%q: expected error", in)
		}
	}
}

func TestEncodeTrainTimetablesErrors(t *testing.T) {

	tests := []struct {
		name  string
		input model.TrainTimetable
		want  string
	}{
		{
			name: "multiple destinations",
			input: model.TrainTimetable{
				DestinationStation: []string{"a", "b"},
			},
			want: "multiple destinations",
		},
		{
			name: "time goes backwards",
			input: model.TrainTimetable{
				TrainTimetableObject: []model.TrainTimetableEntry{
					{DepartureTime: "10:05", DepartureStation: "a"},
					{ArrivalTime: "10:00", ArrivalStation: "b"},
				},
			},
			want: "time goes backwards",
		},
		{
			name: "different stations",
			input: model.TrainTimetable{
				TrainTimetableObject: []model.TrainTimetableEntry{
					{ArrivalTime: "10:00", ArrivalStation: "a", DepartureTime: "10:01", DepartureStation: "b"},
				},
			},
			want: "differs",
		},
		{
			name: "stop without station",
			input: model.TrainTimetable{
				TrainTimetableObject: []model.TrainTimetableEntry{
					{DepartureTime: "10:00"},
				},
			},
			want: "without station",
		},
		{
			name: "stop without time",
			input: model.TrainTimetable{
				TrainTimetableObject: []model.TrainTimetableEntry{
					{DepartureStation: "a"},
				},
			},
			want: "without time",
		},
		{
			name: "invalid time",
			input: model.TrainTimetable{
				TrainTimetableObject: []model.TrainTimetableEntry{
					{DepartureTime: "25:00", DepartureStation: "a"},
				},
			},
			want: "invalid time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EncodeTrainTimetables(&bytes.Buffer{}, []model.TrainTimetable{tt.input})

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestDecodeTrainTimetablesInvalidIndex(t *testing.T) {

	tests := []TrainTimetables{
		{
			Strings: []string{""},
			Trains:  []Train{{Railway: 5}},
		},
		{
			Strings: []string{""},
			Trains:  []Train{{Stops: []Stop{{Station: -1}}}},
		},
		{
			Strings: []string{"x"},
		},
	}

	for i, in := range tests {
		var buf bytes.Buffer

		if err := gob.NewEncoder(&buf).Encode(in); err != nil {
			t.Fatal(err)
		}

		if _, err := DecodeTrainTimetables(&buf); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

// まとめた列車時刻表は、それぞれを読み込んだものと同じ列車・文字列を持つ。
// 同じ文字列（ダイヤ種別など）は1つにまとまる。
func TestMergeTrainTimetables(t *testing.T) {

	encode := func(timetables []model.TrainTimetable) *TrainTimetables {
		var buf bytes.Buffer
		if err := EncodeTrainTimetables(&buf, timetables); err != nil {
			t.Fatal(err)
		}
		tt, err := DecodeTrainTimetables(&buf)
		if err != nil {
			t.Fatal(err)
		}
		return tt
	}

	stops := func(from, to string) []model.TrainTimetableEntry {
		return []model.TrainTimetableEntry{
			{DepartureTime: "10:00", DepartureStation: from},
			{ArrivalTime: "10:05", ArrivalStation: to},
		}
	}

	a := encode([]model.TrainTimetable{
		{
			Train:                "odpt.Train:Toei.Shinjuku.1001",
			Calendar:             "odpt.Calendar:Weekday",
			TrainTimetableObject: stops("odpt.Station:Toei.Shinjuku.Shinjuku", "odpt.Station:Toei.Shinjuku.Ichigaya"),
		},
	})

	b := encode([]model.TrainTimetable{
		{
			Train:                "odpt.Train:Keio.KeioNew.1001",
			Calendar:             "odpt.Calendar:Weekday",
			DestinationStation:   []string{"odpt.Station:Keio.Keio.Hashimoto"},
			TrainTimetableObject: stops("odpt.Station:Keio.KeioNew.Shinjuku", "odpt.Station:Keio.KeioNew.Hatsudai"),
		},
	})

	got := MergeTrainTimetables(a, b)

	if len(got.Trains) != 2 {
		t.Fatalf("expected 2 trains, got %d", len(got.Trains))
	}

	if got.Strings[0] != "" {
		t.Fatalf("expected empty string at index 0, got %q", got.Strings[0])
	}

	// 文字列の重複が無い
	seen := map[string]bool{}
	for _, s := range got.Strings {
		if seen[s] {
			t.Fatalf("duplicate string %q", s)
		}
		seen[s] = true
	}

	for i, src := range []*TrainTimetables{a, b} {

		want := src.Trains[0]
		train := got.Trains[i]

		for _, pair := range [][2]int32{
			{train.Train, want.Train},
			{train.Calendar, want.Calendar},
			{train.Destination, want.Destination},
		} {
			if got.String(pair[0]) != src.String(pair[1]) {
				t.Fatalf("train %d: expected %q, got %q", i, src.String(pair[1]), got.String(pair[0]))
			}
		}

		for j, stop := range train.Stops {
			w := want.Stops[j]
			if got.String(stop.Station) != src.String(w.Station) ||
				stop.Arrival != w.Arrival || stop.Departure != w.Departure {
				t.Fatalf("train %d stop %d: expected %+v, got %+v", i, j, w, stop)
			}
		}
	}

	// 入力は書き換えない
	if a.String(a.Trains[0].Train) != "odpt.Train:Toei.Shinjuku.1001" {
		t.Fatal("input was modified")
	}
}
