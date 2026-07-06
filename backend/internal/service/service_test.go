package service

import (
	"context"
	"errors"
	"testing"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/model"
)

type mockClient struct {
	trainStatus    []model.TrainStatus
	trainLocations []model.TrainLocation

	statusErr error
	locErr    error
}

func (m *mockClient) GetTrainStatus(
	ctx context.Context,
) ([]model.TrainStatus, error) {
	return m.trainStatus, m.statusErr
}

func (m *mockClient) GetTrainLocations(
	ctx context.Context,
) ([]model.TrainLocation, error) {
	return m.trainLocations, m.locErr
}

func newTestService() *Service {

	loader, err := assets.New()
	if err != nil {
		panic(err)
	}

	return New(
		&mockClient{},
		loader,
	)
}

func TestAssociateBy(t *testing.T) {

	type sample struct {
		ID string
	}

	items := []sample{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}

	result := associateBy(
		items,
		func(s sample) string {
			return s.ID
		},
	)

	if len(result) != 3 {
		t.Fatalf(
			"expected 3 items, got %d",
			len(result),
		)
	}

	if _, ok := result["a"]; !ok {
		t.Fatal("missing key a")
	}

	if _, ok := result["b"]; !ok {
		t.Fatal("missing key b")
	}

	if _, ok := result["c"]; !ok {
		t.Fatal("missing key c")
	}
}

func TestGetTrainStatus(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	railway := loader.Railways()[0]

	mock := &mockClient{
		trainStatus: []model.TrainStatus{
			{
				Railway: railway.SameAs,
				TrainInformationText: model.LocalizedString{
					Ja: "平常運転",
				},
			},
		},
	}

	svc := New(
		mock,
		loader,
	)

	result, err := svc.GetTrainStatus(
		context.Background(),
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 {
		t.Fatalf(
			"expected 1 item, got %d",
			len(result),
		)
	}

	if result[0].Railway != railway.RailwayTitle.Ja {
		t.Fatalf(
			"unexpected railway %s",
			result[0].Railway,
		)
	}

	if result[0].Status != "平常運転" {
		t.Fatalf(
			"unexpected status %s",
			result[0].Status,
		)
	}
}

func TestGetTrainStatusExternalAPI(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockClient{
		statusErr: client.ErrExternalAPI,
	}

	svc := New(
		mock,
		loader,
	)

	_, err = svc.GetTrainStatus(
		context.Background(),
	)

	if !errors.Is(
		err,
		ErrExternalAPI,
	) {
		t.Fatalf(
			"expected ErrExternalAPI, got %v",
			err,
		)
	}
}

func TestGetRailways(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	result, err := svc.GetRailways(
		context.Background(),
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(result) != len(loader.Railways()) {
		t.Fatalf(
			"expected %d railways, got %d",
			len(loader.Railways()),
			len(result),
		)
	}

	first := loader.Railways()[0]

	if result[0].ID != first.SameAs {
		t.Fatalf(
			"unexpected id %s",
			result[0].ID,
		)
	}

	if result[0].Name != first.RailwayTitle.Ja {
		t.Fatalf(
			"unexpected name %s",
			result[0].Name,
		)
	}
}

func TestGetStations(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	railway := loader.Railways()[0]

	result, err := svc.GetStations(
		context.Background(),
		railway.SameAs,
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(result) == 0 {
		t.Fatal("expected stations")
	}

	for _, st := range result {

		found := false

		for _, asset := range loader.Stations() {

			if asset.SameAs == st.ID {

				if asset.Railway != railway.SameAs {
					t.Fatal("wrong railway")
				}

				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"station %s not found",
				st.ID,
			)
		}
	}
}

func TestGetStationsNotFound(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	_, err = svc.GetStations(
		context.Background(),
		"dummy",
	)

	if !errors.Is(
		err,
		ErrStationNotFound,
	) {
		t.Fatalf(
			"expected ErrStationNotFound, got %v",
			err,
		)
	}
}

func TestGetStationDetail(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	station := loader.Stations()[0]

	result, err := svc.GetStationDetail(
		context.Background(),
		station.SameAs,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result == nil {
		t.Fatal("expected station detail")
	}

	if result.ID != station.SameAs {
		t.Fatalf(
			"expected id %s, got %s",
			station.SameAs,
			result.ID,
		)
	}

	if result.Name != station.StationTitle.Ja {
		t.Fatalf(
			"expected %s, got %s",
			station.StationTitle.Ja,
			result.Name,
		)
	}

	if result.Timetable == nil {
		t.Fatal("timetable is nil")
	}

	if result.Passengers == nil {
		t.Fatal("passengers is nil")
	}
}

func TestGetStationDetailNotFound(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	_, err = svc.GetStationDetail(
		context.Background(),
		"dummy",
	)

	if !errors.Is(
		err,
		ErrStationNotFound,
	) {
		t.Fatalf(
			"expected ErrStationNotFound, got %v",
			err,
		)
	}
}

func TestGetStationDetailTimetable(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	var stationID string

	for _, tt := range loader.StationTimetables() {
		if len(tt.StationTimetableObject) > 0 {
			stationID = tt.Station
			break
		}
	}

	if stationID == "" {
		t.Skip("no timetable data")
	}

	result, err := svc.GetStationDetail(
		context.Background(),
		stationID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(result.Timetable) == 0 {
		t.Fatal("expected timetable")
	}
}

func TestGetStationDetailPassengers(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	var stationID string

OUT:
	for _, survey := range loader.PassengerSurveys() {
		for _, st := range survey.Station {
			stationID = st
			break OUT
		}
	}

	if stationID == "" {
		t.Skip("no passenger survey")
	}

	result, err := svc.GetStationDetail(
		context.Background(),
		stationID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if len(result.Passengers) == 0 {
		t.Fatal("expected passengers")
	}
}

func TestGetTrainLocation(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockClient{
		trainLocations: []model.TrainLocation{
			{
				TrainNumber: "5301",
				Railway:     loader.Railways()[0].SameAs,
				FromStation: &loader.Stations()[0].SameAs,
				ToStation:   &loader.Stations()[1].SameAs,
				Delay:       60,
			},
		},
	}
	svc := New(
		mock,
		loader,
	)

	result, err := svc.GetTrainLocation(
		context.Background(),
		"5301",
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.TrainNumber != "5301" {
		t.Fatal("unexpected train number")
	}

	if result.Delay != 60 {
		t.Fatal("unexpected delay")
	}

	if result.Railway == "" {
		t.Fatal("railway should not be empty")
	}

	if result.FromStation == "" {
		t.Fatal("from station should not be empty")
	}

	if result.ToStation == "" {
		t.Fatal("to station should not be empty")
	}
}

func TestGetTrainLocationNotFound(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	_, err = svc.GetTrainLocation(
		context.Background(),
		"99999",
	)

	if !errors.Is(err, ErrTrainNotFound) {
		t.Fatalf(
			"expected ErrTrainNotFound, got %v",
			err,
		)
	}
}

func TestGetTrainLocationExternalAPI(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{
			locErr: client.ErrExternalAPI,
		},
		loader,
	)
	_, err = svc.GetTrainLocation(
		context.Background(),
		"5301",
	)

	if !errors.Is(err, ErrExternalAPI) {
		t.Fatalf(
			"expected ErrExternalAPI, got %v",
			err,
		)
	}
}

func TestGetFare(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	fare := loader.RailwayFares()[0]

	svc := New(
		&mockClient{},
		loader,
	)

	result, err := svc.GetFare(
		context.Background(),
		fare.FromStation,
		fare.ToStation,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.IC != fare.IcCardFare {
		t.Fatal("unexpected ic fare")
	}

	if result.Ticket != fare.TicketFare {
		t.Fatal("unexpected ticket fare")
	}
}

func TestGetFareNotFound(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	_, err = svc.GetFare(
		context.Background(),
		"dummy1",
		"dummy2",
	)

	if !errors.Is(err, ErrFareNotFound) {
		t.Fatalf(
			"expected ErrFareNotFound, got %v",
			err,
		)
	}
}
