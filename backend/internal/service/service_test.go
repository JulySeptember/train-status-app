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

	if result[0].RailwayID != railway.SameAs {
		t.Fatalf(
			"unexpected railway id %s",
			result[0].RailwayID,
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

	byID := map[string]Railway{}
	for _, r := range result {
		byID[r.ID] = r
	}

	asakusa, ok := byID["odpt.Railway:Toei.Asakusa"]
	if !ok || asakusa.LineCode != "A" || asakusa.Color != "#FF535F" {
		t.Fatalf("unexpected line code or color: %+v", asakusa)
	}

	// 荒川線は路線の色が配信されないので、色は空になる（JSON では省かれる）
	arakawa, ok := byID["odpt.Railway:Toei.Arakawa"]
	if !ok || arakawa.LineCode != "SA" || arakawa.Color != "" {
		t.Fatalf("unexpected line code or color: %+v", arakawa)
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

// 駅一覧は路線の駅順（odpt:stationOrder）で返す
func TestGetStationsOrder(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	names := func(routeID string) []string {
		t.Helper()

		result, err := svc.GetStations(context.Background(), routeID)
		if err != nil {
			t.Fatal(err)
		}

		items := make([]string, 0, len(result))
		for _, st := range result {
			items = append(items, st.Name)
		}

		return items
	}

	mita := names("odpt.Railway:Toei.Mita")

	if want := []string{"目黒", "白金台", "白金高輪"}; !slices.Equal(mita[:3], want) {
		t.Fatalf("expected %v, got %v", want, mita[:3])
	}

	if mita[len(mita)-1] != "西高島平" {
		t.Fatalf("unexpected last station %s", mita[len(mita)-1])
	}

	// 大江戸線は都庁前が駅順に2回現れるが、一覧には1回だけ含める
	oedo := names("odpt.Railway:Toei.Oedo")

	if len(oedo) != 38 {
		t.Fatalf("expected 38 stations, got %d", len(oedo))
	}

	if oedo[0] != "都庁前" || oedo[len(oedo)-1] != "光が丘" {
		t.Fatalf("unexpected oedo order %v", oedo)
	}

	// 全路線で、路線に属する駅をすべて重複なく返す
	for _, railway := range loader.Railways() {

		result, err := svc.GetStations(context.Background(), railway.SameAs)
		if err != nil {
			t.Fatal(err)
		}

		seen := make(map[string]bool)

		for _, st := range result {
			if seen[st.ID] {
				t.Fatalf("%s: duplicated station %s", railway.SameAs, st.ID)
			}
			seen[st.ID] = true
		}

		count := 0
		for _, st := range loader.Stations() {
			if st.Railway == railway.SameAs {
				count++
			}
		}

		if len(result) != count {
			t.Fatalf(
				"%s: expected %d stations, got %d",
				railway.SameAs,
				count,
				len(result),
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

// 全駅の時刻表で、行先（直通運転先を含む）と列車種別に日本語名が付く
func TestGetStationDetailDestinationAndTrainType(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(
		&mockClient{},
		loader,
	)

	// 名前が見つからず ID の末尾にフォールバックした値は ASCII だけになる
	isASCII := func(s string) bool {
		for _, r := range s {
			if r > 127 {
				return false
			}
		}
		return true
	}

	destinations := make(map[string]bool)
	trainTypes := make(map[string]bool)

	for _, station := range loader.Stations() {

		result, err := svc.GetStationDetail(
			context.Background(),
			station.SameAs,
		)
		if err != nil {
			t.Fatal(err)
		}

		for _, tt := range result.Timetables {
			for _, row := range tt.Timetables {

				// 大江戸線の環状部（外回り・内回り）は、駅時刻表の元データに行先が無い
				noDestination := station.Railway == "odpt.Railway:Toei.Oedo" &&
					(tt.RailDirection == "odpt.RailDirection:OuterLoop" ||
						tt.RailDirection == "odpt.RailDirection:InnerLoop")

				switch {
				case row.Destination == "" && noDestination:
					// 環状部は行先が空でよい
				case row.Destination == "" || isASCII(row.Destination):
					t.Fatalf(
						"%s %s: unresolved destination %q",
						station.SameAs,
						row.Time,
						row.Destination,
					)
				}

				if row.TrainTypeID == "" || isASCII(row.TrainType) {
					t.Fatalf(
						"%s %s: unresolved train type %q (%s)",
						station.SameAs,
						row.Time,
						row.TrainType,
						row.TrainTypeID,
					)
				}

				destinations[row.Destination] = true
				trainTypes[row.TrainType] = true
			}
		}
	}

	// 直通運転先の行先と、各停以外の種別が含まれている
	for _, want := range []string{"日吉", "羽田空港第1・第2ターミナル", "成田空港", "笹塚"} {
		if !destinations[want] {
			t.Fatalf("expected destination %s", want)
		}
	}

	for _, want := range []string{"普通", "急行", "エアポート快特", "アクセス特急"} {
		if !trainTypes[want] {
			t.Fatalf("expected train type %s", want)
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
