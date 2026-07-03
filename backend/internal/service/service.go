package service

import (
	"context"
	"fmt"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/model"
)

type Service struct {
	client *client.Client
	assets *assets.Loader
}

func New(
	c *client.Client,
	a *assets.Loader,
) *Service {
	return &Service{
		client: c,
		assets: a,
	}
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

	items := make([]Station, 0)

	for _, station := range s.assets.Stations() {

		if station.Railway != routeID {
			continue
		}

		items = append(items, Station{
			ID:   station.SameAs,
			Name: station.StationTitle.Ja,
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

	Timetable []Timetable `json:"timetable"`

	Passengers []Passenger `json:"passengers"`
}

type Timetable struct {
	Time        string `json:"time"`
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
		return nil, fmt.Errorf(
			"station not found: %s",
			stationID,
		)
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
		ID:   station.SameAs,
		Name: station.StationTitle.Ja,
	}

	for _, tt := range stationTables {

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

			detail.Timetable = append(
				detail.Timetable,
				Timetable{
					Time:        time,
					TrainNumber: obj.TrainNumber,
					Destination: destination,
				},
			)
		}
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
	TrainNumber string `json:"trainNumber"`

	Railway string `json:"railway"`

	FromStation string `json:"fromStation"`
	ToStation   string `json:"toStation"`

	Delay int `json:"delay"`
}

// =========================
// Train Location
// =========================

func (s *Service) GetTrainLocation(
	ctx context.Context,
	trainNumber string,
) (*TrainLocation, error) {

	trains, err := s.client.GetTrainLocations(ctx)
	if err != nil {
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

		if train.TrainNumber != trainNumber {
			continue
		}

		item := &TrainLocation{
			TrainNumber: train.TrainNumber,
			Delay:       train.Delay,
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

	return nil, fmt.Errorf(
		"train not found: %s",
		trainNumber,
	)
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

	return nil, fmt.Errorf(
		"fare not found: %s -> %s",
		from,
		to,
	)
}
