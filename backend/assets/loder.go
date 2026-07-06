package assets

import (
	"embed"
	"encoding/json"
	"fmt"

	"train-status-app/backend/internal/model"
)

//go:embed *.json
var fs embed.FS

type Loader struct {
	railways          []model.Railway
	stations          []model.Station
	fares             []model.RailwayFare
	stationTimetables []model.StationTimetable
	trainTimetables   []model.TrainTimetable
	passengerSurveys  []model.PassengerSurvey
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

	if err := load("station_timetable.json", &l.stationTimetables); err != nil {
		return nil, err
	}

	if err := load("train_timetable.json", &l.trainTimetables); err != nil {
		return nil, err
	}

	if err := load("passenger_survey.json", &l.passengerSurveys); err != nil {
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

func (l *Loader) TrainTimetables() []model.TrainTimetable {
	return l.trainTimetables
}

func (l *Loader) PassengerSurveys() []model.PassengerSurvey {
	return l.passengerSurveys
}
