package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

// 都営以外の事業者のデータを、都内に絞って assets/extra に書き出す（-kind extra）。
//
// 入力は scripts/update_assets.sh が取得した ODPT の JSON:
//
//	<raw>/operators/<事業者>/{Railway,Station,TrainType}.json   事業者ごとの API
//	<raw>/dumps/<ホスト>/{StationTimetable,TrainTimetable}.json 全件ダウンロード用の URL
//
// 都営のデータは assets 直下にあるので、全件版に含まれていても捨てる。
// 設計は docs/design/multi-operator.md 5章。

const toeiOperator = "odpt.Operator:Toei"

// operatorSummary は、事業者ごとに残した件数（ログに出す）。
type operatorSummary struct {
	railways          int
	stations          int
	stationTimetables int
	trains            int
	droppedStops      int // 時刻の無い停車
	multiDestinations int // 行先が2駅以上ある列車
	estimatedTrains   int // 駅時刻表から推定した列車（列車時刻表の無い路線）
}

type extraSummary map[string]*operatorSummary

func (s extraSummary) of(operator string) *operatorSummary {
	if s[operator] == nil {
		s[operator] = &operatorSummary{}
	}
	return s[operator]
}

func genExtra(rawDir, areaPath, outDir string) error {

	area, err := readArea(areaPath)
	if err != nil {
		return err
	}

	summary := extraSummary{}

	operatorFiles := func(name string) ([]string, error) {
		files, err := filepath.Glob(filepath.Join(rawDir, "operators", "*", name))
		if err == nil && len(files) == 0 {
			err = fmt.Errorf("%s: no operators/*/%s", rawDir, name)
		}
		return files, err
	}

	dumpFiles := func(name string) ([]string, error) {
		files, err := filepath.Glob(filepath.Join(rawDir, "dumps", "*", name))
		if err == nil && len(files) == 0 {
			err = fmt.Errorf("%s: no dumps/*/%s", rawDir, name)
		}
		return files, err
	}

	// 駅
	files, err := operatorFiles("Station.json")
	if err != nil {
		return err
	}
	stations, err := filterRaw(files, func(v struct {
		SameAs   string `json:"owl:sameAs"`
		Operator string `json:"odpt:operator"`
	}) bool {
		ok := v.Operator != toeiOperator && area[v.SameAs]
		if ok {
			summary.of(v.Operator).stations++
		}
		return ok
	})
	if err != nil {
		return err
	}

	// 路線（都内の駅を1つでも通るもの）
	files, err = operatorFiles("Railway.json")
	if err != nil {
		return err
	}
	operators := make(map[string]bool)
	keptRailways := make(map[string]bool)
	railwayFiles := files
	railways, err := filterRaw(files, func(v struct {
		SameAs       string               `json:"owl:sameAs"`
		Operator     string               `json:"odpt:operator"`
		StationOrder []model.StationOrder `json:"odpt:stationOrder"`
	}) bool {
		ok := v.Operator != toeiOperator && slices.ContainsFunc(v.StationOrder, func(o model.StationOrder) bool {
			return area[o.Station]
		})
		if ok {
			operators[v.Operator] = true
			keptRailways[v.SameAs] = true
			summary.of(v.Operator).railways++
		}
		return ok
	})
	if err != nil {
		return err
	}

	// 列車種別（路線を残した事業者のもの）
	files, err = operatorFiles("TrainType.json")
	if err != nil {
		return err
	}
	trainTypes, err := filterRaw(files, func(v struct {
		Operator string `json:"odpt:operator"`
	}) bool {
		return operators[v.Operator]
	})
	if err != nil {
		return err
	}

	// 駅時刻表（都内の駅のもの）
	files, err = dumpFiles("StationTimetable.json")
	if err != nil {
		return err
	}
	var stationTimetables []model.StationTimetable
	for _, path := range files {
		var all []model.StationTimetable
		if err := readJSON(path, &all); err != nil {
			return err
		}
		for _, tt := range all {
			if tt.Operator != toeiOperator && area[tt.Station] {
				stationTimetables = append(stationTimetables, tt)
				summary.of(tt.Operator).stationTimetables++
			}
		}
	}

	// 列車時刻表（都内の最初の停車から最後の停車まで）
	files, err = dumpFiles("TrainTimetable.json")
	if err != nil {
		return err
	}
	var trainTimetables []model.TrainTimetable
	hasTrains := make(map[string]bool) // 列車時刻表のある路線
	for _, path := range files {
		var all []model.TrainTimetable
		if err := readJSON(path, &all); err != nil {
			return err
		}
		for _, tt := range all {
			if tt.Operator == toeiOperator {
				continue
			}
			hasTrains[tt.Railway] = true
			s := summary.of(tt.Operator)
			clipped, dropped, ok := clipTrain(tt, area)
			s.droppedStops += dropped
			if !ok {
				continue
			}
			if len(tt.DestinationStation) > 1 {
				s.multiDestinations++
			}
			trainTimetables = append(trainTimetables, clipped)
			s.trains++
		}
	}

	// 列車時刻表の無い路線（東急・西武・小田急・京急・ゆりかもめ）は、駅時刻表から列車を推定する。
	// 推定は都内に絞る前の全駅の駅時刻表で行い、列車時刻表と同じく都内の停車に切る
	stationFiles, err := operatorFiles("Station.json")
	if err != nil {
		return err
	}
	estimated, err := estimateFromRaw(railwayFiles, stationFiles, files, func(railway string) bool {
		return keptRailways[railway] && !hasTrains[railway]
	})
	if err != nil {
		return err
	}
	for _, tt := range estimated {
		s := summary.of(tt.Operator)
		clipped, _, ok := clipTrain(tt, area)
		if !ok {
			continue
		}
		if len(tt.DestinationStation) > 1 {
			s.multiDestinations++
		}
		trainTimetables = append(trainTimetables, clipped)
		s.trains++
		s.estimatedTrains++
	}

	// 都外の行先駅の駅名（直通運転・都外まで走る列車の行先を表示するため）
	files, err = operatorFiles("Station.json")
	if err != nil {
		return err
	}
	destinations, err := destinationStations(files, area, stationTimetables, trainTimetables)
	if err != nil {
		return err
	}

	// 2つの全件版に同じ事業者が入ると、同じ列車・時刻表が2つずつになる
	if err := checkDuplicates(stationTimetables, trainTimetables); err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	for name, data := range map[string][]json.RawMessage{
		"station.json":             stations,
		"railway.json":             railways,
		"train_type.json":          trainTypes,
		"destination_station.json": destinations,
	} {
		if err := writeJSON(filepath.Join(outDir, name), data); err != nil {
			return err
		}
	}

	var buf bytes.Buffer
	if err := slim.Encode(&buf, stationTimetables); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "station_timetable.gob"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	buf.Reset()
	if err := slim.EncodeTrainTimetables(&buf, trainTimetables); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "train_timetable.gob"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	for _, op := range slices.Sorted(maps.Keys(summary)) {
		s := summary[op]
		if *s == (operatorSummary{droppedStops: s.droppedStops}) {
			continue // 都内に駅の無い事業者（全件版に含まれる相鉄など）
		}
		log.Printf(
			"%s: %d railways, %d stations, %d station timetables, %d trains (%d estimated from station timetables; dropped %d stops without time, %d trains with multiple destinations keep the first)",
			op, s.railways, s.stations, s.stationTimetables, s.trains, s.estimatedTrains, s.droppedStops, s.multiDestinations,
		)
	}

	return nil
}

// destinationStations は、時刻表の行先のうち都外の駅を、駅ID と駅名だけにして返す（ID 順）。
// 駅のデータに無い駅（京成など ODPT に無い事業者の駅）は含めない。
func destinationStations(
	stationFiles []string,
	area map[string]bool,
	stationTimetables []model.StationTimetable,
	trainTimetables []model.TrainTimetable,
) ([]json.RawMessage, error) {

	wanted := make(map[string]bool)
	for _, tt := range stationTimetables {
		for _, obj := range tt.StationTimetableObject {
			for _, id := range obj.DestinationStation {
				wanted[id] = !area[id]
			}
		}
	}
	for _, tt := range trainTimetables {
		for _, id := range tt.DestinationStation {
			wanted[id] = !area[id]
		}
	}

	type destination struct {
		SameAs       string                `json:"owl:sameAs"`
		StationTitle model.LocalizedString `json:"odpt:stationTitle"`
	}

	found := make(map[string]destination)

	for _, path := range stationFiles {
		var stations []destination
		if err := readJSON(path, &stations); err != nil {
			return nil, err
		}
		for _, st := range stations {
			if wanted[st.SameAs] {
				found[st.SameAs] = st
			}
		}
	}

	result := make([]json.RawMessage, 0, len(found))

	for _, id := range slices.Sorted(maps.Keys(found)) {
		data, err := json.Marshal(found[id])
		if err != nil {
			return nil, err
		}
		result = append(result, data)
	}

	return result, nil
}

// clipTrain は、列車時刻表を都内の最初の停車から最後の停車までに切る。
// 時刻の無い停車（乗り降りできない）は捨て、捨てた数を返す。
// 都内の停車が2つ未満なら ok は false。
// 行先が2駅以上ある列車（分割・併合）は、経路検索の結果に1つしか出せないので最初の行先だけを残す。
func clipTrain(tt model.TrainTimetable, area map[string]bool) (model.TrainTimetable, int, bool) {

	stops := make([]model.TrainTimetableEntry, 0, len(tt.TrainTimetableObject))
	dropped := 0

	for _, obj := range tt.TrainTimetableObject {
		if obj.ArrivalTime == "" && obj.DepartureTime == "" {
			dropped++
			continue
		}
		stops = append(stops, obj)
	}

	station := func(obj model.TrainTimetableEntry) string {
		if obj.DepartureStation != "" {
			return obj.DepartureStation
		}
		return obj.ArrivalStation
	}

	first, last := -1, -1
	for i, obj := range stops {
		if area[station(obj)] {
			if first < 0 {
				first = i
			}
			last = i
		}
	}

	if first < 0 || first == last {
		return tt, dropped, false
	}

	tt.TrainTimetableObject = stops[first : last+1]

	if len(tt.DestinationStation) > 1 {
		tt.DestinationStation = tt.DestinationStation[:1]
	}

	return tt, dropped, true
}

// filterRaw は、JSON の配列の要素のうち keep が true を返すものを、元の形のまま返す
// （odpt:connectingStation など model に無い項目も残す）。
func filterRaw[T any](paths []string, keep func(T) bool) ([]json.RawMessage, error) {

	var result []json.RawMessage

	for _, path := range paths {

		var items []json.RawMessage
		if err := readJSON(path, &items); err != nil {
			return nil, err
		}

		for _, item := range items {
			var v T
			if err := json.Unmarshal(item, &v); err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			if keep(v) {
				result = append(result, item)
			}
		}
	}

	return result, nil
}

func readJSON(path string, v any) error {

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

func writeJSON(path string, items []json.RawMessage) error {

	if items == nil {
		items = []json.RawMessage{}
	}

	data, err := json.Marshal(items)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// checkDuplicates は、同じ時刻表（owl:sameAs）が2つ以上あればエラーにする。
func checkDuplicates(stationTimetables []model.StationTimetable, trainTimetables []model.TrainTimetable) error {

	seen := make(map[string]bool, len(stationTimetables)+len(trainTimetables))

	check := func(id string) error {
		if id == "" {
			return nil
		}
		if seen[id] {
			return fmt.Errorf("duplicate timetable %s: the same operator is in more than one dump", id)
		}
		seen[id] = true
		return nil
	}

	for _, tt := range stationTimetables {
		if err := check(tt.SameAs); err != nil {
			return err
		}
	}

	for _, tt := range trainTimetables {
		if err := check(tt.SameAs); err != nil {
			return err
		}
	}

	return nil
}
