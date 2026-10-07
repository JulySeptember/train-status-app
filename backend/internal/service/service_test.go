package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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

	if result.Timetables == nil {
		t.Fatal("timetables is nil")
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

	if len(result.Timetables) == 0 {
		t.Fatal("expected timetables")
	}
}

// 方面は上り → 下り → それ以外、ダイヤ種別は平日 → 土曜 → 休日 → 土休日の順に返す
func TestGetStationDetailTimetableOrder(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	cases := map[string][]string{
		"odpt.Station:Toei.Mita.Kasuga": {
			"Northbound|Weekday",
			"Northbound|SaturdayHoliday",
			"Southbound|Weekday",
			"Southbound|SaturdayHoliday",
		},
		// 荒川線は土曜・休日が別ダイヤ
		"odpt.Station:Toei.Arakawa.Minowabashi": {
			"Toei.Waseda|Weekday",
			"Toei.Waseda|Saturday",
			"Toei.Waseda|Holiday",
		},
		// 都庁前は上り・下り以外の方面（光が丘方面）を持つ
		"odpt.Station:Toei.Oedo.Tochomae": {
			"OuterLoop|Weekday",
			"OuterLoop|SaturdayHoliday",
			"InnerLoop|Weekday",
			"InnerLoop|SaturdayHoliday",
			"Toei.Hikarigaoka|Weekday",
			"Toei.Hikarigaoka|SaturdayHoliday",
		},
	}

	short := func(id string) string {
		return id[strings.Index(id, ":")+1:]
	}

	for stationID, want := range cases {

		// map の反復順に依存していないことを確かめるため、複数回取得する
		for range 20 {

			result, err := svc.GetStationDetail(
				context.Background(),
				stationID,
			)
			if err != nil {
				t.Fatal(err)
			}

			got := make([]string, 0, len(result.Timetables))
			for _, tt := range result.Timetables {
				got = append(got, short(tt.RailDirection)+"|"+short(tt.Calendar))
			}

			if !slices.Equal(got, want) {
				t.Fatalf("%s: expected %v, got %v", stationID, want, got)
			}
		}
	}
}

func TestGetStationDetailToday(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	// 2026-10-12（月）はスポーツの日
	svc.now = func() time.Time {
		return time.Date(2026, time.October, 12, 12, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	}

	result, err := svc.GetStationDetail(
		context.Background(),
		"odpt.Station:Toei.Mita.Kasuga",
	)

	if err != nil {
		t.Fatal(err)
	}

	if !result.TrainLocationAvailable {
		t.Fatal("expected train location available")
	}

	for _, tt := range result.Timetables {

		want := tt.Calendar == "odpt.Calendar:SaturdayHoliday"

		if tt.IsToday != want {
			t.Fatalf(
				"%s: expected isToday %v",
				tt.Calendar,
				want,
			)
		}

		for _, row := range tt.Timetables {
			if row.TrainID == "" {
				t.Fatal("expected train id")
			}
		}
	}
}

func TestGetStationDetailTrainLocationUnsupported(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	result, err := svc.GetStationDetail(
		context.Background(),
		"odpt.Station:Toei.NipporiToneri.Nippori",
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.TrainLocationAvailable {
		t.Fatal("expected train location unavailable")
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

// findTrain は指定路線の時刻表から列車ID・列車番号を1件取り出す
func findTrain(
	t *testing.T,
	loader *assets.Loader,
	railway string,
) (string, string) {

	t.Helper()

	for _, tt := range loader.StationTimetables() {
		if tt.Railway != railway {
			continue
		}

		for _, obj := range tt.StationTimetableObject {
			return obj.Train, obj.TrainNumber
		}
	}

	t.Fatalf("no train found on %s", railway)
	return "", ""
}

func TestGetTrainLocation(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	railway := "odpt.Railway:Toei.Mita"
	trainID, trainNumber := findTrain(t, loader, railway)

	from := "odpt.Station:Toei.Mita.Kasuga"
	to := "odpt.Station:Toei.Mita.Hakusan"

	mock := &mockClient{
		trainLocations: []model.TrainLocation{
			{
				SameAs:      trainID,
				TrainNumber: trainNumber,
				Railway:     railway,
				FromStation: &from,
				ToStation:   &to,
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
		trainID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.TrainID != trainID {
		t.Fatal("unexpected train id")
	}

	if result.TrainNumber != trainNumber {
		t.Fatal("unexpected train number")
	}

	if result.Delay != 60 {
		t.Fatal("unexpected delay")
	}

	if result.Railway != "三田線" {
		t.Fatalf("unexpected railway %s", result.Railway)
	}

	if result.FromStation != "春日" {
		t.Fatalf("unexpected from station %s", result.FromStation)
	}

	if result.ToStation != "白山" {
		t.Fatalf("unexpected to station %s", result.ToStation)
	}

	if result.Stopped {
		t.Fatal("expected running train")
	}
}

// 同じ列車番号が別路線で走っていても、指定した列車IDの列車を返す
func TestGetTrainLocationSameNumberOnOtherRailway(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	trainID, trainNumber := findTrain(t, loader, "odpt.Railway:Toei.Mita")

	asakusaFrom := "odpt.Station:Toei.Asakusa.Ningyocho"
	mitaFrom := "odpt.Station:Toei.Mita.Kasuga"

	mock := &mockClient{
		trainLocations: []model.TrainLocation{
			{
				SameAs:      "odpt.Train:Toei.Asakusa." + trainNumber,
				TrainNumber: trainNumber,
				Railway:     "odpt.Railway:Toei.Asakusa",
				FromStation: &asakusaFrom,
			},
			{
				SameAs:      trainID,
				TrainNumber: trainNumber,
				Railway:     "odpt.Railway:Toei.Mita",
				FromStation: &mitaFrom,
			},
		},
	}

	svc := New(
		mock,
		loader,
	)

	result, err := svc.GetTrainLocation(
		context.Background(),
		trainID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.Railway != "三田線" {
		t.Fatalf("unexpected railway %s", result.Railway)
	}

	if result.FromStation != "春日" {
		t.Fatalf("unexpected from station %s", result.FromStation)
	}
}

func TestGetTrainLocationStopped(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	trainID, trainNumber := findTrain(t, loader, "odpt.Railway:Toei.Mita")

	from := "odpt.Station:Toei.Mita.Kasuga"

	svc := New(
		&mockClient{
			trainLocations: []model.TrainLocation{
				{
					SameAs:      trainID,
					TrainNumber: trainNumber,
					Railway:     "odpt.Railway:Toei.Mita",
					FromStation: &from,
					ToStation:   nil,
				},
			},
		},
		loader,
	)

	result, err := svc.GetTrainLocation(
		context.Background(),
		trainID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if !result.Stopped {
		t.Fatal("expected stopped train")
	}

	if result.FromStation != "春日" {
		t.Fatalf("unexpected from station %s", result.FromStation)
	}

	if result.ToStation != "" {
		t.Fatalf("unexpected to station %s", result.ToStation)
	}
}

func TestGetTrainLocationNotRunning(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	trainID, _ := findTrain(t, loader, "odpt.Railway:Toei.Mita")

	svc := New(
		&mockClient{},
		loader,
	)

	result, err := svc.GetTrainLocation(
		context.Background(),
		trainID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.Available {
		t.Fatal("expected unavailable train")
	}

	if result.Message == "" {
		t.Fatal("expected message")
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
		"odpt.Train:Toei.Mita.dummy",
	)

	if !errors.Is(err, ErrTrainNotFound) {
		t.Fatalf(
			"expected ErrTrainNotFound, got %v",
			err,
		)
	}
}

// 列車位置が配信されない路線では外部APIを呼ばずに案内を返す
func TestGetTrainLocationUnsupportedRailway(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	trainID, _ := findTrain(t, loader, "odpt.Railway:Toei.NipporiToneri")

	svc := New(
		&mockClient{
			locErr: errors.New("should not be called"),
		},
		loader,
	)

	result, err := svc.GetTrainLocation(
		context.Background(),
		trainID,
	)

	if err != nil {
		t.Fatal(err)
	}

	if result.Available {
		t.Fatal("expected unavailable train")
	}

	if result.Message == "" {
		t.Fatal("expected message")
	}
}

func TestGetTrainLocationExternalAPI(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	trainID, _ := findTrain(t, loader, "odpt.Railway:Toei.Mita")

	svc := New(
		&mockClient{
			locErr: client.ErrExternalAPI,
		},
		loader,
	)
	_, err = svc.GetTrainLocation(
		context.Background(),
		trainID,
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
