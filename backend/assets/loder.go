package assets

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/model"
)

// 駅時刻表と列車時刻表は JSON（それぞれ約25MB）を埋め込まず、
// 必要な項目だけに絞った gob を埋め込む（起動を速くするため）。
// station_timetable.json / train_timetable.json を更新したら go generate ./assets で再生成する。
//
//go:generate go run ../cmd/gen-assets -kind station -in station_timetable.json -out station_timetable.gob
//go:generate go run ../cmd/gen-assets -kind train -in train_timetable.json -out train_timetable.gob

//go:embed railway.json station.json railway_fare.json passenger_survey.json train_type.json station_timetable.gob train_timetable.gob extra
var embedded embed.FS

// 都営以外の事業者のデータは extra/ に置く（docs/design/multi-operator.md 4章）。
// ライセンスで公開が禁じられているので、リポジトリには README.md だけを置き、
// データは scripts/update_assets.sh で手元に取るか、CD が S3 から取ってきて埋め込む。
// 無ければ都営だけで動く。
const extraDir = "extra"

// extraFiles は extra/ に置くファイル。すべてそろっているか、すべて無いかのどちらかとする。
var extraFiles = []string{
	"railway.json",
	"station.json",
	"train_type.json",
	"station_timetable.gob",
	"train_timetable.gob",
}

type Loader struct {
	railways          []model.Railway
	stations          []model.Station
	fares             []model.RailwayFare
	stationTimetables []model.StationTimetable
	trainTimetables   *slim.TrainTimetables
	passengerSurveys  []model.PassengerSurvey
	trainTypes        []model.TrainType

	hasExtra bool
}

type options struct {
	extra bool
}

type Option func(*options)

// WithExtra は、extra/ に都営以外の事業者のデータがあれば、都営のデータに加えて読み込む。
func WithExtra() Option {
	return func(o *options) {
		o.extra = true
	}
}

func New(opts ...Option) (*Loader, error) {
	return newFromFS(embedded, opts...)
}

func newFromFS(fsys fs.FS, opts ...Option) (*Loader, error) {

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	l := &Loader{}

	if err := load(fsys, "railway.json", &l.railways); err != nil {
		return nil, err
	}

	if err := load(fsys, "station.json", &l.stations); err != nil {
		return nil, err
	}

	if err := load(fsys, "railway_fare.json", &l.fares); err != nil {
		return nil, err
	}

	if err := loadStationTimetables(fsys, "station_timetable.gob", &l.stationTimetables); err != nil {
		return nil, err
	}

	if err := loadTrainTimetables(fsys, "train_timetable.gob", &l.trainTimetables); err != nil {
		return nil, err
	}

	if err := load(fsys, "passenger_survey.json", &l.passengerSurveys); err != nil {
		return nil, err
	}

	if err := load(fsys, "train_type.json", &l.trainTypes); err != nil {
		return nil, err
	}

	if o.extra {
		if err := l.loadExtra(fsys); err != nil {
			return nil, err
		}
	}

	return l, nil
}

// loadExtra は extra/ のデータを都営のデータに足す。extra/ にデータが無ければ何もしない。
func (l *Loader) loadExtra(fsys fs.FS) error {

	var missing []string

	for _, name := range extraFiles {
		_, err := fs.Stat(fsys, extraDir+"/"+name)
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, name)
		} else if err != nil {
			return fmt.Errorf("%s/%s: %w", extraDir, name, err)
		}
	}

	if len(missing) == len(extraFiles) {
		return nil
	}

	if len(missing) > 0 {
		return fmt.Errorf("%s: missing %v: run scripts/update_assets.sh", extraDir, missing)
	}

	var (
		railways          []model.Railway
		stations          []model.Station
		trainTypes        []model.TrainType
		stationTimetables []model.StationTimetable
		trainTimetables   *slim.TrainTimetables
	)

	if err := load(fsys, extraDir+"/railway.json", &railways); err != nil {
		return err
	}

	if err := load(fsys, extraDir+"/station.json", &stations); err != nil {
		return err
	}

	if err := load(fsys, extraDir+"/train_type.json", &trainTypes); err != nil {
		return err
	}

	if err := loadStationTimetables(fsys, extraDir+"/station_timetable.gob", &stationTimetables); err != nil {
		return err
	}

	if err := loadTrainTimetables(fsys, extraDir+"/train_timetable.gob", &trainTimetables); err != nil {
		return err
	}

	l.railways = append(l.railways, railways...)
	l.stations = append(l.stations, stations...)
	l.trainTypes = append(l.trainTypes, trainTypes...)
	l.stationTimetables = append(l.stationTimetables, stationTimetables...)
	l.trainTimetables = slim.MergeTrainTimetables(l.trainTimetables, trainTimetables)
	l.hasExtra = true

	return nil
}

func load(fsys fs.FS, name string, v any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	return nil
}

func loadStationTimetables(fsys fs.FS, name string, v *[]model.StationTimetable) error {
	data, err := fs.ReadFile(fsys, name)
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

func loadTrainTimetables(fsys fs.FS, name string, v **slim.TrainTimetables) error {
	data, err := fs.ReadFile(fsys, name)
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

// HasExtra は、都営以外の事業者のデータを読み込んだかを返す。
func (l *Loader) HasExtra() bool {
	return l.hasExtra
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
