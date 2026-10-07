// Package slim は、駅時刻表（odpt:StationTimetable）をアプリが使う項目だけに絞った
// 軽量な形式に変換する。
//
// station_timetable.json（約25MB、約12万件）をそのまま json.Unmarshal すると、
// Lambda のコールドスタートで数百ms と数十MB を使う。そこで、繰り返し現れる文字列
// （路線・駅・列車ID・時刻など）を文字列表にまとめ、各項目をその番号で持つ形にして
// gob で保存する。
//
// 保持する項目は、service が使うものだけ（路線・駅・ダイヤ種別・方面・発着時刻・
// 列車ID・列車番号・列車種別・行先）。それ以外（@id、dc:date、番線など）は捨てる。
package slim

import (
	"encoding/gob"
	"fmt"
	"io"

	"train-status-app/backend/internal/model"
)

// StationTimetables は軽量化した駅時刻表。
// Strings の 0 番は常に空文字で、項目が無いことを表す。
type StationTimetables struct {
	Strings    []string
	Timetables []Timetable
}

type Timetable struct {
	Railway       int32
	Station       int32
	Calendar      int32
	RailDirection int32

	Entries []Entry
}

type Entry struct {
	DepartureTime int32
	ArrivalTime   int32
	Train         int32
	TrainNumber   int32
	TrainType     int32
	Destination   int32
}

// Encode は駅時刻表を軽量な形式で w に書き出す。
// 行先が2駅以上ある項目は表現できないため、エラーにする。
func Encode(w io.Writer, timetables []model.StationTimetable) error {

	b := newStringTable()

	out := StationTimetables{
		Timetables: make([]Timetable, 0, len(timetables)),
	}

	for _, tt := range timetables {

		item := Timetable{
			Railway:       b.id(tt.Railway),
			Station:       b.id(tt.Station),
			Calendar:      b.id(tt.Calendar),
			RailDirection: b.id(tt.RailDirection),
			Entries:       make([]Entry, 0, len(tt.StationTimetableObject)),
		}

		for _, obj := range tt.StationTimetableObject {

			if len(obj.DestinationStation) > 1 {
				return fmt.Errorf(
					"%s %s: multiple destinations are not supported",
					tt.SameAs,
					obj.Train,
				)
			}

			destination := ""
			if len(obj.DestinationStation) == 1 {
				destination = obj.DestinationStation[0]
			}

			item.Entries = append(item.Entries, Entry{
				DepartureTime: b.id(obj.DepartureTime),
				ArrivalTime:   b.id(obj.ArrivalTime),
				Train:         b.id(obj.Train),
				TrainNumber:   b.id(obj.TrainNumber),
				TrainType:     b.id(obj.TrainType),
				Destination:   b.id(destination),
			})
		}

		out.Timetables = append(out.Timetables, item)
	}

	out.Strings = b.strings

	return gob.NewEncoder(w).Encode(out)
}

// Decode は軽量な形式を読み込み、model の駅時刻表に戻す。
// 同じ文字列は同じ領域を共有する。行先のスライスも行先ごとに共有するので、
// 呼び出し側で書き換えてはいけない。
func Decode(r io.Reader) ([]model.StationTimetable, error) {

	var in StationTimetables

	if err := gob.NewDecoder(r).Decode(&in); err != nil {
		return nil, err
	}

	str := func(i int32) (string, error) {
		if i < 0 || int(i) >= len(in.Strings) {
			return "", fmt.Errorf("string index %d out of range", i)
		}
		return in.Strings[i], nil
	}

	destinations := make(map[int32][]string)

	result := make([]model.StationTimetable, 0, len(in.Timetables))

	for _, tt := range in.Timetables {

		var item model.StationTimetable
		var err error

		if item.Railway, err = str(tt.Railway); err != nil {
			return nil, err
		}
		if item.Station, err = str(tt.Station); err != nil {
			return nil, err
		}
		if item.Calendar, err = str(tt.Calendar); err != nil {
			return nil, err
		}
		if item.RailDirection, err = str(tt.RailDirection); err != nil {
			return nil, err
		}

		item.StationTimetableObject = make([]model.StationTimetableEntry, len(tt.Entries))

		for i, e := range tt.Entries {

			obj := &item.StationTimetableObject[i]

			if obj.DepartureTime, err = str(e.DepartureTime); err != nil {
				return nil, err
			}
			if obj.ArrivalTime, err = str(e.ArrivalTime); err != nil {
				return nil, err
			}
			if obj.Train, err = str(e.Train); err != nil {
				return nil, err
			}
			if obj.TrainNumber, err = str(e.TrainNumber); err != nil {
				return nil, err
			}
			if obj.TrainType, err = str(e.TrainType); err != nil {
				return nil, err
			}

			if e.Destination != 0 {
				dest, ok := destinations[e.Destination]
				if !ok {
					s, err := str(e.Destination)
					if err != nil {
						return nil, err
					}
					dest = []string{s}
					destinations[e.Destination] = dest
				}
				obj.DestinationStation = dest
			}
		}

		result = append(result, item)
	}

	return result, nil
}

type stringTable struct {
	strings []string
	index   map[string]int32
}

func newStringTable() *stringTable {
	return &stringTable{
		strings: []string{""},
		index:   map[string]int32{"": 0},
	}
}

func (t *stringTable) id(s string) int32 {
	if i, ok := t.index[s]; ok {
		return i
	}

	i := int32(len(t.strings))
	t.strings = append(t.strings, s)
	t.index[s] = i

	return i
}
