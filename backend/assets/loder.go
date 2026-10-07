package assets

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

// 駅時刻表と列車時刻表は JSON（それぞれ約25MB）を埋め込まず、
// 必要な項目だけに絞った gob を埋め込む（起動を速くするため）。
// station_timetable.json / train_timetable.json を更新したら go generate ./assets で再生成する。
//
//go:generate go run ../cmd/gen-assets -kind station -in station_timetable.json -out station_timetable.gob
//go:generate go run ../cmd/gen-assets -kind train -in train_timetable.json -out train_timetable.gob

//go:embed railway.json station.json railway_fare.json passenger_survey.json train_type.json station_timetable.gob train_timetable.gob
var fs embed.FS

type Loader struct {
	railways          []model.Railway
	stations          []model.Station
	fares             []model.RailwayFare
	stationTimetables []model.StationTimetable
	trainTimetables   *slim.TrainTimetables
	passengerSurveys  []model.PassengerSurvey
	trainTypes        []model.TrainType
}

func New() (*Loader, error) {
	l := &Loader{}

	if err := load("railway.json", &l.railways); err != nil {
		return nil, err
	}

	if err := load("station.json", &l.stations); err != nil {
		return nil, err
	}

	if err := load("railway_fare.json", &l.fares); err != nil {
		return nil, err
	}

	if err := loadStationTimetables(&l.stationTimetables); err != nil {
		return nil, err
	}

	if err := loadTrainTimetables(&l.trainTimetables); err != nil {
		return nil, err
	}

	if err := load("passenger_survey.json", &l.passengerSurveys); err != nil {
		return nil, err
	}

	if err := load("train_type.json", &l.trainTypes); err != nil {
		return nil, err
	}

	return l, nil
}

func load(name string, v any) error {
	data, err := fs.ReadFile(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}

func loadStationTimetables(v *[]model.StationTimetable) error {
	const name = "station_timetable.gob"

	data, err := fs.ReadFile(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	timetables, err := slim.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	*v = timetables

	return nil
}

func loadTrainTimetables(v **slim.TrainTimetables) error {
	const name = "train_timetable.gob"

	data, err := fs.ReadFile(name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	timetables, err := slim.DecodeTrainTimetables(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	*v = timetables

	return nil
}

func (l *Loader) Railways() []model.Railway {
	return l.railways
}

func (l *Loader) Stations() []model.Station {
	return l.stations
}

func (l *Loader) RailwayFares() []model.RailwayFare {
	return l.fares
}

func (l *Loader) StationTimetables() []model.StationTimetable {
	return l.stationTimetables
}

// TrainTimetables は経路探索用の列車時刻表を返す。呼び出し側で書き換えてはいけない。
func (l *Loader) TrainTimetables() *slim.TrainTimetables {
	return l.trainTimetables
}

func (l *Loader) PassengerSurveys() []model.PassengerSurvey {
	return l.passengerSurveys
}

func (l *Loader) TrainTypes() []model.TrainType {
	return l.trainTypes
}
