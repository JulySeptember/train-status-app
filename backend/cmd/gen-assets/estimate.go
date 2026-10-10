package main

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"train-status-app/backend/internal/infer"
	"train-status-app/backend/internal/model"
)

// estimateTrains は、列車時刻表の無い路線（estimate が true の路線。東急・西武・小田急・京急・ゆりかもめ）の列車を、
// 駅時刻表の発車をつないで推定し、列車時刻表の形にして返す（docs/design/multi-operator.md 8章）。
// 推定は都内に絞る前の全駅の駅時刻表で行い、都内に絞るのは呼び出し側（clipTrain）で行う。
//
// 行先が路線の外の列車は、直通先の列車へ渡す境目の駅まで延ばす（infer.ExtendToPartners）。直通先の列車は、
// starts（ダイヤ種別 → 駅 → その駅から出る列車の始発。列車時刻表のある列車）と、推定した列車から探す。
// connect は駅 → 乗り換えでつながる駅。
func estimateTrains(
	railways []infer.RawRailway,
	timetables []infer.RawStationTimetable,
	estimate func(railway string) bool,
	starts map[string]map[string][]infer.Partner,
	connect map[string][]string,
) []model.TrainTimetable {

	lines := infer.Lines(railways, timetables, estimate)

	keys := make([]infer.LineKey, 0, len(lines))
	for k := range lines {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b infer.LineKey) int {
		return cmp.Or(cmp.Compare(a.Railway, b.Railway), cmp.Compare(a.RailDirection, b.RailDirection), cmp.Compare(a.Calendar, b.Calendar))
	})

	operators := make(map[string]string)
	for _, r := range railways {
		operators[r.SameAs] = r.Operator
	}

	inferred := make(map[infer.LineKey][]infer.Train, len(lines))
	for _, k := range keys {
		inferred[k] = infer.Infer(lines[k])
	}

	// 推定した列車の始発も、直通先として探す（西武池袋線 → 西武有楽町線のように、どちらも推定する路線がある）
	all := make(map[string]map[string][]infer.Partner)
	add := func(calendar string, p infer.Partner) {
		if all[calendar] == nil {
			all[calendar] = make(map[string][]infer.Partner)
		}
		all[calendar][p.Station] = append(all[calendar][p.Station], p)
	}
	for calendar, byStation := range starts {
		for _, ps := range byStation {
			for _, p := range ps {
				add(calendar, p)
			}
		}
	}
	for k, trains := range inferred {
		for _, tr := range trains {
			first := tr.Stops[0]
			add(k.Calendar, infer.Partner{Station: first.Station, Minutes: first.Minutes, Destination: first.Destination})
		}
	}

	var result []model.TrainTimetable

	for _, k := range keys {

		partners := func(station string) []infer.Partner {
			ps := slices.Clone(all[k.Calendar][station])
			for _, s := range connect[station] {
				ps = append(ps, all[k.Calendar][s]...)
			}
			return ps
		}

		for n, tr := range infer.ExtendToPartners(lines[k], inferred[k], partners) {

			first := tr.Stops[0]
			id := infer.TrainID(k, n+1)

			tt := model.TrainTimetable{
				SameAs:        "odpt.TrainTimetable:" + strings.TrimPrefix(id, "odpt.Train:"),
				Train:         id,
				Railway:       k.Railway,
				Operator:      operators[k.Railway],
				Calendar:      k.Calendar,
				RailDirection: k.RailDirection,
				TrainType:     first.TrainType,
			}
			if first.Destination != "" {
				tt.DestinationStation = strings.Split(first.Destination, ",")
			}

			for _, s := range tr.Stops {
				tt.TrainTimetableObject = append(tt.TrainTimetableObject, model.TrainTimetableEntry{
					DepartureStation: s.Station,
					DepartureTime:    clock(s.Minutes),
				})
			}
			if tr.Terminal != "" {
				tt.TrainTimetableObject = append(tt.TrainTimetableObject, model.TrainTimetableEntry{
					ArrivalStation: tr.Terminal,
					ArrivalTime:    clock(tr.TerminalMinutes),
				})
			}

			result = append(result, tt)
		}
	}

	return result
}

// clock は運行日の0時からの分を "HH:MM"（24時以降は翌日の時刻）にする。
func clock(minutes int) string {
	return fmt.Sprintf("%02d:%02d", minutes/60%24, minutes%60)
}

// estimateFromRaw は、事業者ごとの路線（railwayFiles）・駅（stationFiles）と、全件版の列車時刻表（trainFiles）・
// 同じディレクトリの駅時刻表を読んで、estimateTrains を呼ぶ。
func estimateFromRaw(railwayFiles, stationFiles, trainFiles []string, estimate func(railway string) bool) ([]model.TrainTimetable, error) {

	// 乗り換えでつながる駅（他社の駅データの odpt:connectingStation。都営の駅への乗り換えは逆向きにも足す）
	connect := make(map[string][]string)
	for _, path := range stationFiles {
		var v []struct {
			SameAs     string   `json:"owl:sameAs"`
			Connecting []string `json:"odpt:connectingStation"`
		}
		if err := readJSON(path, &v); err != nil {
			return nil, err
		}
		for _, s := range v {
			for _, c := range s.Connecting {
				connect[s.SameAs] = append(connect[s.SameAs], c)
				connect[c] = append(connect[c], s.SameAs)
			}
		}
	}

	// 列車時刻表のある列車の始発（都営を含む）
	starts := make(map[string]map[string][]infer.Partner)
	for _, path := range trainFiles {
		var v []model.TrainTimetable
		if err := readJSON(path, &v); err != nil {
			return nil, err
		}
		for _, tt := range v {
			if len(tt.TrainTimetableObject) == 0 || len(tt.DestinationStation) == 0 {
				continue
			}
			first := tt.TrainTimetableObject[0]
			m, ok := infer.Minutes(first.DepartureTime)
			if !ok || first.DepartureStation == "" {
				continue
			}
			if starts[tt.Calendar] == nil {
				starts[tt.Calendar] = make(map[string][]infer.Partner)
			}
			starts[tt.Calendar][first.DepartureStation] = append(starts[tt.Calendar][first.DepartureStation],
				infer.Partner{Station: first.DepartureStation, Minutes: m, Destination: strings.Join(tt.DestinationStation, ",")})
		}
	}

	var railways []infer.RawRailway
	for _, path := range railwayFiles {
		var v []infer.RawRailway
		if err := readJSON(path, &v); err != nil {
			return nil, err
		}
		railways = append(railways, v...)
	}

	var timetables []infer.RawStationTimetable
	for _, path := range trainFiles {
		var v []infer.RawStationTimetable
		if err := readJSON(filepath.Join(filepath.Dir(path), "StationTimetable.json"), &v); err != nil {
			return nil, err
		}
		for _, tt := range v {
			if estimate(tt.Railway) {
				timetables = append(timetables, tt)
			}
		}
	}

	return estimateTrains(railways, timetables, estimate, starts, connect), nil
}
