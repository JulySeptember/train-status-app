package service

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/model"
)

var (
	ErrExternalAPI     = errors.New("external api error")
	ErrStationNotFound = errors.New("station not found")
	ErrFareNotFound    = errors.New("fare not found")
	ErrTrainNotFound   = errors.New("train not found")
)

// 列車位置情報（odpt:Train）が配信されていない路線
var trainLocationUnsupported = map[string]bool{
	"odpt.Railway:Toei.NipporiToneri": true,
}

type TrainClient interface {
	GetTrainStatus(ctx context.Context) ([]model.TrainStatus, error)
	GetTrainLocations(ctx context.Context) ([]model.TrainLocation, error)
}

type Service struct {
	client TrainClient
	assets *assets.Loader

	// 列車ID（odpt.Train:...）から路線・列車番号を引くための索引
	trains map[string]trainRef

	now func() time.Time
}

type trainRef struct {
	railway     string
	trainNumber string
}

func New(
	c TrainClient,
	a *assets.Loader,
) *Service {
	return &Service{
		client: c,
		assets: a,
		trains: indexTrains(a.StationTimetables()),
		now:    time.Now,
	}
}

func indexTrains(
	timetables []model.StationTimetable,
) map[string]trainRef {

	result := make(map[string]trainRef)

	for _, tt := range timetables {
		for _, obj := range tt.StationTimetableObject {
			if obj.Train == "" {
				continue
			}

			result[obj.Train] = trainRef{
				railway:     tt.Railway,
				trainNumber: obj.TrainNumber,
			}
		}
	}

	return result
}

// =========================
// Utility
// =========================

func associateBy[T any, K comparable](
	items []T,
	key func(T) K,
) map[K]T {

	result := make(map[K]T, len(items))

	for _, item := range items {
		result[key(item)] = item
	}

	return result
}

// =========================
// Train Status DTO
// =========================

type TrainStatus struct {
	Railway string `json:"railway"`
	Status  string `json:"status"`
}

// =========================
// Realtime
// =========================

func (s *Service) GetTrainStatus(
	ctx context.Context,
) ([]TrainStatus, error) {

	statuses, err := s.client.GetTrainStatus(ctx)
	if err != nil {
		if errors.Is(err, client.ErrExternalAPI) {
			return nil, ErrExternalAPI
		}
		return nil, err
	}

	railwayMap := associateBy(
		s.assets.Railways(),
		func(r model.Railway) string {
			return r.SameAs
		},
	)

	items := make([]TrainStatus, 0, len(statuses))

	for _, status := range statuses {

		name := status.Railway

		if railway, ok := railwayMap[status.Railway]; ok {
			name = railway.RailwayTitle.Ja
		}

		items = append(items, TrainStatus{
			Railway: name,
			Status:  status.TrainInformationText.Ja,
		})
	}

	return items, nil
}

// =========================
// Railway DTO
// =========================

type Railway struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// =========================
// Asset
// =========================

func (s *Service) GetRailways(
	ctx context.Context,
) ([]Railway, error) {

	items := make([]Railway, 0, len(s.assets.Railways()))

	for _, r := range s.assets.Railways() {
		items = append(items, Railway{
			ID:   r.SameAs,
			Name: r.RailwayTitle.Ja,
		})
	}

	return items, nil
}

// =========================
// Station DTO
// =========================

type Station struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) GetStations(
	ctx context.Context,
	routeID string,
) ([]Station, error) {

	idx := slices.IndexFunc(
		s.assets.Railways(),
		func(r model.Railway) bool {
			return r.SameAs == routeID
		},
	)

	if idx < 0 {
		return nil, ErrStationNotFound
	}

	railway := s.assets.Railways()[idx]

	stationMap := make(map[string]model.Station)

	for _, station := range s.assets.Stations() {
		if station.Railway == routeID {
			stationMap[station.SameAs] = station
		}
	}

	order := slices.Clone(railway.StationOrder)

	slices.SortStableFunc(order, func(a, b model.StationOrder) int {
		return a.Index - b.Index
	})

	items := make([]Station, 0, len(stationMap))

	// 路線上の駅順に並べる。
	// 大江戸線は環状部と放射部の分岐点（都庁前）が2回現れるため、最初の出現だけを使う
	for _, o := range order {

		station, ok := stationMap[o.Station]
		if !ok {
			continue
		}

		items = append(items, Station{
			ID:   station.SameAs,
			Name: station.StationTitle.Ja,
		})

		delete(stationMap, o.Station)
	}

	// 駅順に含まれない駅があれば、ID 順で末尾に加える
	rest := slices.Sorted(maps.Keys(stationMap))

	for _, id := range rest {
		items = append(items, Station{
			ID:   id,
			Name: stationMap[id].StationTitle.Ja,
		})
	}

	return items, nil
}

// =========================
// Station Detail DTO
// =========================

type StationDetail struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// この駅の路線で列車位置情報が配信されているか
	TrainLocationAvailable bool `json:"trainLocationAvailable"`

	Timetables []DirectionTimetable `json:"timetables"`

	Passengers []Passenger `json:"passengers"`
}

type DirectionTimetable struct {
	Calendar      string      `json:"calendar"`
	RailDirection string      `json:"railDirection"`
	Timetables    []Timetable `json:"timetables"`

	// 現在の運行日に適用されるダイヤか
	IsToday bool `json:"isToday"`
}

type Timetable struct {
	Time        string `json:"time"`
	TrainID     string `json:"trainId"`
	TrainNumber string `json:"trainNumber"`
	Destination string `json:"destination"`
}

type Passenger struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// =========================
// Station Detail
// =========================

func (s *Service) GetStationDetail(
	ctx context.Context,
	stationID string,
) (*StationDetail, error) {

	stations := s.assets.Stations()
	timetables := s.assets.StationTimetables()
	surveys := s.assets.PassengerSurveys()

	stationMap := associateBy(
		stations,
		func(st model.Station) string {
			return st.SameAs
		},
	)

	station, ok := stationMap[stationID]
	if !ok {
		return nil, ErrStationNotFound
	}

	stationTables := make([]model.StationTimetable, 0)

	for _, tt := range timetables {
		if tt.Station == stationID {
			stationTables = append(
				stationTables,
				tt,
			)
		}
	}

	passengers := make([]model.PassengerSurvey, 0)

	for _, survey := range surveys {

		for _, st := range survey.Station {

			if st == stationID {
				passengers = append(
					passengers,
					survey,
				)
				break
			}
		}
	}

	detail := &StationDetail{
		ID:                     station.SameAs,
		Name:                   station.StationTitle.Ja,
		TrainLocationAvailable: !trainLocationUnsupported[station.Railway],
		Timetables:             make([]DirectionTimetable, 0),
		Passengers:             make([]Passenger, 0),
	}

	todayCalendars := calendar.Calendars(s.now())

	groups := make(map[string]*DirectionTimetable)

	for _, tt := range stationTables {

		key := tt.Calendar + "|" + tt.RailDirection

		group, ok := groups[key]
		if !ok {
			group = &DirectionTimetable{
				Calendar:      tt.Calendar,
				RailDirection: tt.RailDirection,
				IsToday:       slices.Contains(todayCalendars, tt.Calendar),
			}
			groups[key] = group
		}

		for _, obj := range tt.StationTimetableObject {

			time := obj.DepartureTime
			if time == "" {
				time = obj.ArrivalTime
			}

			destination := ""
			if len(obj.DestinationStation) > 0 {
				if st, ok := stationMap[obj.DestinationStation[0]]; ok {
					destination = st.StationTitle.Ja
				}
			}

			group.Timetables = append(group.Timetables, Timetable{
				Time:        time,
				TrainID:     obj.Train,
				TrainNumber: obj.TrainNumber,
				Destination: destination,
			})
		}
	}

	for _, g := range groups {
		detail.Timetables = append(detail.Timetables, *g)
	}
	for _, survey := range passengers {

		for _, p := range survey.PassengerSurveyObject {

			detail.Passengers = append(
				detail.Passengers,
				Passenger{
					Year:  p.SurveyYear,
					Count: p.PassengerJourneys,
				},
			)
		}
	}

	return detail, nil

}

// =========================
// Train Location DTO
// =========================

type TrainLocation struct {
	TrainID     string `json:"trainId"`
	TrainNumber string `json:"trainNumber"`

	Railway string `json:"railway"`

	FromStation string `json:"fromStation"`
	ToStation   string `json:"toStation"`

	// true の場合は fromStation に停車中（toStation は空）
	Stopped bool `json:"stopped"`

	Delay int `json:"delay"`

	Available bool   `json:"available"`
	Message   string `json:"message"`
}

// =========================
// Train Location
// =========================

func (s *Service) GetTrainLocation(
	ctx context.Context,
	trainID string,
) (*TrainLocation, error) {

	ref, ok := s.trains[trainID]
	if !ok {
		return nil, ErrTrainNotFound
	}

	if trainLocationUnsupported[ref.railway] {
		return &TrainLocation{
			TrainID:     trainID,
			TrainNumber: ref.trainNumber,
			Available:   false,
			Message:     "この路線は列車位置情報が提供されていません",
		}, nil
	}

	trains, err := s.client.GetTrainLocations(ctx)
	if err != nil {
		if errors.Is(err, client.ErrExternalAPI) {
			return nil, ErrExternalAPI
		}
		return nil, err
	}

	stationMap := associateBy(
		s.assets.Stations(),
		func(st model.Station) string {
			return st.SameAs
		},
	)

	railwayMap := associateBy(
		s.assets.Railways(),
		func(r model.Railway) string {
			return r.SameAs
		},
	)

	for _, train := range trains {

		// 列車番号は路線間で重複するため、路線を含む列車IDで照合する
		if train.SameAs != trainID {
			continue
		}

		item := &TrainLocation{
			TrainID:     trainID,
			TrainNumber: train.TrainNumber,
			Stopped:     train.ToStation == nil,
			Delay:       train.Delay,
			Available:   true,
		}

		if railway, ok := railwayMap[train.Railway]; ok {
			item.Railway = railway.RailwayTitle.Ja
		}

		if train.FromStation != nil {
			if station, ok := stationMap[*train.FromStation]; ok {
				item.FromStation = station.StationTitle.Ja
			}
		}

		if train.ToStation != nil {
			if station, ok := stationMap[*train.ToStation]; ok {
				item.ToStation = station.StationTitle.Ja
			}
		}

		return item, nil
	}

	return &TrainLocation{
		TrainID:     trainID,
		TrainNumber: ref.trainNumber,
		Available:   false,
		Message:     "現在この列車の運行情報は取得できません",
	}, nil

}

// =========================
// Fare DTO
// =========================

type Fare struct {
	From string `json:"from"`
	To   string `json:"to"`

	IC     int `json:"icFare"`
	Ticket int `json:"ticketFare"`
}

// =========================
// Fare
// =========================

func (s *Service) GetFare(
	ctx context.Context,
	from string,
	to string,
) (*Fare, error) {

	for _, fare := range s.assets.RailwayFares() {

		if fare.FromStation == from &&
			fare.ToStation == to {

			return &Fare{
				From:   fare.FromStation,
				To:     fare.ToStation,
				IC:     fare.IcCardFare,
				Ticket: fare.TicketFare,
			}, nil
		}
	}

	return nil, ErrFareNotFound
}
