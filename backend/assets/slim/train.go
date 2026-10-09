package slim

import (
	"encoding/gob"
	"fmt"
	"io"
	"strconv"
	"strings"

	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/model"
)

// NoTime は、停車駅に到着時刻または発車時刻が無いことを表す
// （始発駅の到着時刻・終点の発車時刻）。
const NoTime int16 = -1

// TrainTimetables は軽量化した列車時刻表。
// 経路探索で使うため、駅時刻表と違って model の型には戻さず、番号と分のまま持つ。
// Strings の 0 番は常に空文字で、項目が無いことを表す。
type TrainTimetables struct {
	Strings []string
	Trains  []Train
}

// Train は列車1本の時刻表。各項目は Strings の番号。
type Train struct {
	Train         int32
	TrainNumber   int32
	Railway       int32
	Calendar      int32
	RailDirection int32
	TrainType     int32
	Destination   int32

	Stops []Stop
}

// Stop は停車駅1つ。Station は Strings の番号。
// 時刻は運行日の0時からの分で、ServiceDayStartHour より前の時刻は翌日として
// 24時間を足す（00:15 → 1455）。無い場合は NoTime。
type Stop struct {
	Station   int32
	Arrival   int16
	Departure int16
}

// String は番号 i の文字列を返す。
// DecodeTrainTimetables で読み込んだ値は範囲を確認済み。
func (t *TrainTimetables) String(i int32) string {
	return t.Strings[i]
}

// EncodeTrainTimetables は列車時刻表を軽量な形式で w に書き出す。
// 行先が2駅以上ある列車、時刻の形式が不正な停車駅、時刻が逆戻りする列車はエラーにする。
func EncodeTrainTimetables(w io.Writer, timetables []model.TrainTimetable) error {

	b := newStringTable()

	out := TrainTimetables{
		Trains: make([]Train, 0, len(timetables)),
	}

	for _, tt := range timetables {

		if len(tt.DestinationStation) > 1 {
			return fmt.Errorf("%s: multiple destinations are not supported", tt.SameAs)
		}

		destination := ""
		if len(tt.DestinationStation) == 1 {
			destination = tt.DestinationStation[0]
		}

		train := Train{
			Train:         b.id(tt.Train),
			TrainNumber:   b.id(tt.TrainNumber),
			Railway:       b.id(tt.Railway),
			Calendar:      b.id(tt.Calendar),
			RailDirection: b.id(tt.RailDirection),
			TrainType:     b.id(tt.TrainType),
			Destination:   b.id(destination),
			Stops:         make([]Stop, 0, len(tt.TrainTimetableObject)),
		}

		last := NoTime

		for _, obj := range tt.TrainTimetableObject {

			stop, err := encodeStop(b, obj)
			if err != nil {
				return fmt.Errorf("%s: %w", tt.SameAs, err)
			}

			for _, m := range []int16{stop.Arrival, stop.Departure} {
				if m == NoTime {
					continue
				}
				if m < last {
					return fmt.Errorf("%s: time goes backwards at %s", tt.SameAs, b.strings[stop.Station])
				}
				last = m
			}

			train.Stops = append(train.Stops, stop)
		}

		out.Trains = append(out.Trains, train)
	}

	out.Strings = b.strings

	return gob.NewEncoder(w).Encode(out)
}

func encodeStop(b *stringTable, obj model.TrainTimetableEntry) (Stop, error) {

	station := obj.DepartureStation
	if station == "" {
		station = obj.ArrivalStation
	}

	if station == "" {
		return Stop{}, fmt.Errorf("stop without station")
	}

	if obj.ArrivalStation != "" && obj.DepartureStation != "" &&
		obj.ArrivalStation != obj.DepartureStation {
		return Stop{}, fmt.Errorf(
			"arrival station %s differs from departure station %s",
			obj.ArrivalStation,
			obj.DepartureStation,
		)
	}

	arrival, err := parseMinutes(obj.ArrivalTime)
	if err != nil {
		return Stop{}, fmt.Errorf("%s: %w", station, err)
	}

	departure, err := parseMinutes(obj.DepartureTime)
	if err != nil {
		return Stop{}, fmt.Errorf("%s: %w", station, err)
	}

	if arrival == NoTime && departure == NoTime {
		return Stop{}, fmt.Errorf("%s: stop without time", station)
	}

	return Stop{
		Station:   b.id(station),
		Arrival:   arrival,
		Departure: departure,
	}, nil
}

// parseMinutes は "HH:MM" を運行日の0時からの分に変換する。空文字は NoTime。
func parseMinutes(s string) (int16, error) {

	if s == "" {
		return NoTime, nil
	}

	hh, mm, ok := strings.Cut(s, ":")
	if !ok || len(hh) != 2 || len(mm) != 2 {
		return 0, fmt.Errorf("invalid time %q", s)
	}

	h, err := strconv.Atoi(hh)
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid time %q", s)
	}

	m, err := strconv.Atoi(mm)
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q", s)
	}

	if h < calendar.ServiceDayStartHour {
		h += 24
	}

	return int16(h*60 + m), nil
}

// DecodeTrainTimetables は軽量な形式の列車時刻表を読み込む。
// 文字列の番号がすべて範囲内であることを確認する。
func DecodeTrainTimetables(r io.Reader) (*TrainTimetables, error) {

	var t TrainTimetables

	if err := gob.NewDecoder(r).Decode(&t); err != nil {
		return nil, err
	}

	if len(t.Strings) == 0 || t.Strings[0] != "" {
		return nil, fmt.Errorf("string table must start with an empty string")
	}

	valid := func(i int32) bool {
		return i >= 0 && int(i) < len(t.Strings)
	}

	for i, train := range t.Trains {

		for _, s := range []int32{
			train.Train,
			train.TrainNumber,
			train.Railway,
			train.Calendar,
			train.RailDirection,
			train.TrainType,
			train.Destination,
		} {
			if !valid(s) {
				return nil, fmt.Errorf("train %d: string index %d out of range", i, s)
			}
		}

		for _, stop := range train.Stops {
			if !valid(stop.Station) {
				return nil, fmt.Errorf("train %d: string index %d out of range", i, stop.Station)
			}
		}
	}

	return &t, nil
}

// MergeTrainTimetables は、複数の列車時刻表を1つにまとめる（都営と他社など）。
// 文字列表を作り直し、各列車の番号を新しい表の番号に置き換える。入力は書き換えない。
func MergeTrainTimetables(ts ...*TrainTimetables) *TrainTimetables {

	b := newStringTable()

	total := 0
	for _, t := range ts {
		total += len(t.Trains)
	}

	out := &TrainTimetables{
		Trains: make([]Train, 0, total),
	}

	for _, t := range ts {

		ids := make([]int32, len(t.Strings))
		for i, s := range t.Strings {
			ids[i] = b.id(s)
		}

		for _, train := range t.Trains {

			stops := make([]Stop, len(train.Stops))
			for i, stop := range train.Stops {
				stops[i] = Stop{
					Station:   ids[stop.Station],
					Arrival:   stop.Arrival,
					Departure: stop.Departure,
				}
			}

			out.Trains = append(out.Trains, Train{
				Train:         ids[train.Train],
				TrainNumber:   ids[train.TrainNumber],
				Railway:       ids[train.Railway],
				Calendar:      ids[train.Calendar],
				RailDirection: ids[train.RailDirection],
				TrainType:     ids[train.TrainType],
				Destination:   ids[train.Destination],
				Stops:         stops,
			})
		}
	}

	out.Strings = b.strings

	return out
}
