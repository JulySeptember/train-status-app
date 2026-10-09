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

	// 運行情報を取得できた事業者と、失敗した事業者。statusSucceeded が nil なら都営だけが成功
	statusSucceeded []string
	statusFailed    []string

	statusErr error
	locErr    error

	// true なら GetOperatorTrainLocations が client.ErrNoSource を返す（事業者を設定していない）
	locNoSource bool

	// GetOperatorTrainLocations で問い合わせた事業者
	locationOperators []string
}

func (m *mockClient) GetTrainStatus(
	ctx context.Context,
) (client.Result[model.TrainStatus], error) {
	if m.statusErr != nil {
		return client.Result[model.TrainStatus]{}, m.statusErr
	}
	succeeded := m.statusSucceeded
	if succeeded == nil {
		succeeded = []string{"Toei"}
	}
	return client.Result[model.TrainStatus]{
		Items:     m.trainStatus,
		Succeeded: succeeded,
		Failed:    m.statusFailed,
	}, nil
}

func (m *mockClient) GetTrainLocations(
	ctx context.Context,
) (client.Result[model.TrainLocation], error) {
	if m.locErr != nil {
		return client.Result[model.TrainLocation]{}, m.locErr
	}
	return client.Result[model.TrainLocation]{Items: m.trainLocations, Succeeded: []string{"Toei"}}, nil
}

func (m *mockClient) GetOperatorTrainLocations(
	ctx context.Context,
	operator string,
) ([]model.TrainLocation, error) {
	m.locationOperators = append(m.locationOperators, operator)
	if m.locNoSource {
		return nil, client.ErrNoSource
	}
	if m.locErr != nil {
		return nil, m.locErr
	}
	var result []model.TrainLocation
	for _, t := range m.trainLocations {
		if operatorOf(t.SameAs) == operator {
			result = append(result, t)
		}
	}
	return result, nil
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
	if asakusa.Operator != "odpt.Operator:Toei" || asakusa.OperatorName != "都営交通" {
		t.Fatalf("unexpected operator: %+v", asakusa)
	}

	// 荒川線は路線の色が配信されないので、色は空になる（JSON では省かれる）
	arakawa, ok := byID["odpt.Railway:Toei.Arakawa"]
	if !ok || arakawa.LineCode != "SA" || arakawa.Color != "" {
		t.Fatalf("unexpected line code or color: %+v", arakawa)
	}

	wantDirections := []RailDirection{
		{ID: "odpt.RailDirection:Northbound", Name: "北行"},
		{ID: "odpt.RailDirection:Southbound", Name: "南行"},
	}
	if !slices.Equal(asakusa.Directions, wantDirections) {
		t.Fatalf("directions = %+v, want %+v", asakusa.Directions, wantDirections)
	}
}

// 東京メトロの方向は終点の駅で表すので、路線の駅の名前から「〇〇方面」にする
func TestRailwayDirectionName(t *testing.T) {
	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(&mockClient{}, loader)
	svc.stationNames["odpt.Station:TokyoMetro.Marunouchi.Ogikubo"] = "荻窪"

	railway := model.Railway{
		SameAs: "odpt.Railway:TokyoMetro.Marunouchi",
		StationOrder: []model.StationOrder{
			{Index: 1, Station: "odpt.Station:TokyoMetro.Marunouchi.Ogikubo"},
		},
	}

	tests := map[string]string{
		"odpt.RailDirection:TokyoMetro.Ogikubo":   "荻窪方面",
		"odpt.RailDirection:Inbound":              "上り",
		"odpt.RailDirection:TokyoMetro.Ikebukuro": "Ikebukuro",
	}
	for id, want := range tests {
		if got := svc.railwayDirectionName(railway, id); got != want {
			t.Errorf("railwayDirectionName(%s) = %s, want %s", id, got, want)
		}
	}
}

func TestGetAllStations(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(&mockClient{}, loader)

	result, err := svc.GetAllStations(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != len(loader.Stations()) {
		t.Fatalf("expected %d stations, got %d", len(loader.Stations()), len(result))
	}

	byID := map[string]StationSummary{}
	for _, st := range result {
		if _, dup := byID[st.ID]; dup {
			t.Fatalf("duplicate station %s", st.ID)
		}
		byID[st.ID] = st
	}

	// 路線ごとの一覧（GET /api/routes/{id}/stations）と同じ順に並ぶ
	first := loader.Railways()[0]
	stations, err := svc.GetStations(context.Background(), first.SameAs)
	if err != nil {
		t.Fatal(err)
	}
	for i, st := range stations {
		if result[i].ID != st.ID || result[i].RailwayID != first.SameAs {
			t.Fatalf("result[%d] = %+v, want %s on %s", i, result[i], st.ID, first.SameAs)
		}
	}

	// 同じ名前の駅（三田線と大江戸線の春日）は、経路検索の代表の駅が同じになる
	mita := byID["odpt.Station:Toei.Mita.Kasuga"]
	oedo := byID["odpt.Station:Toei.Oedo.Kasuga"]
	if mita.JourneyStation == "" || mita.JourneyStation != oedo.JourneyStation {
		t.Fatalf("Kasuga journey stations: %q, %q", mita.JourneyStation, oedo.JourneyStation)
	}

	// 名前の違う駅はまとめない
	if byID["odpt.Station:Toei.Mita.Suidobashi"].JourneyStation == mita.JourneyStation {
		t.Fatal("Suidobashi must not be grouped with Kasuga")
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

				TrainType:     "odpt.TrainType:Toei.Express",
				RailDirection: "odpt.RailDirection:Southbound",
				// 直通運転先（東急）の駅は through_service.go の辞書で引く
				DestinationStation: []string{"odpt.Station:Tokyu.Meguro.Hiyoshi"},
				Date:               "2026-10-08T20:12:26+09:00",
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

	if result.Delay != 60 || !result.DelayAvailable {
		t.Fatalf("unexpected delay %d (available %v)", result.Delay, result.DelayAvailable)
	}

	if result.RailwayID != railway || result.Railway != "三田線" {
		t.Fatalf("unexpected railway %s %s", result.RailwayID, result.Railway)
	}

	if result.FromStationID != from || result.FromStation != "春日" {
		t.Fatalf("unexpected from station %s %s", result.FromStationID, result.FromStation)
	}

	if result.ToStationID != to || result.ToStation != "白山" {
		t.Fatalf("unexpected to station %s %s", result.ToStationID, result.ToStation)
	}

	if result.TrainTypeID != "odpt.TrainType:Toei.Express" || result.TrainType != "急行" {
		t.Fatalf("unexpected train type %s %s", result.TrainTypeID, result.TrainType)
	}

	if result.RailDirection != "odpt.RailDirection:Southbound" {
		t.Fatalf("unexpected direction %s", result.RailDirection)
	}

	if result.Destination != "日吉" {
		t.Fatalf("unexpected destination %s", result.Destination)
	}

	if result.UpdatedAt != "2026-10-08T20:12:26+09:00" {
		t.Fatalf("unexpected updatedAt %s", result.UpdatedAt)
	}

	if result.Stopped {
		t.Fatal("expected running train")
	}
}

// 荒川線は odpt:delay が配信されないので、遅れを出せないことを返す
func TestGetTrainLocationDelayUnavailable(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	railway := "odpt.Railway:Toei.Arakawa"
	trainID, trainNumber := findTrain(t, loader, railway)

	from := "odpt.Station:Toei.Arakawa.Minowabashi"

	mock := &mockClient{
		trainLocations: []model.TrainLocation{
			{
				SameAs:      trainID,
				TrainNumber: trainNumber,
				Railway:     railway,
				FromStation: &from,
			},
		},
	}

	result, err := New(mock, loader).GetTrainLocation(context.Background(), trainID)
	if err != nil {
		t.Fatal(err)
	}

	if result.DelayAvailable {
		t.Fatal("expected delay to be unavailable on Arakawa line")
	}

	if !result.Stopped || result.ToStationID != "" {
		t.Fatalf("expected stopped train, got toStationId %q", result.ToStationID)
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

// 位置が配信されていない列車は、本日のダイヤから出発前か運行を終えたかを返す。
// 浅草線 2022N は平日ダイヤで泉岳寺 20:31 発、西馬込 20:44 着（泉岳寺始発）
func TestGetTrainLocationNotRunningSchedule(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	const trainID = "odpt.Train:Toei.Asakusa.2022N"
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)

	tests := []struct {
		name    string
		now     time.Time
		want    string
		station string
		time    string
	}{
		{"出発前", time.Date(2026, 10, 8, 20, 26, 0, 0, jst), NotRunningBeforeDeparture, "泉岳寺", "20:31"},
		// 出発時刻を過ぎても2分までは、配信の遅れとみなして出発前のままにする
		{"出発時刻と同じ分", time.Date(2026, 10, 8, 20, 31, 30, 0, jst), NotRunningBeforeDeparture, "泉岳寺", "20:31"},
		{"出発の2分後まで", time.Date(2026, 10, 8, 20, 32, 59, 0, jst), NotRunningBeforeDeparture, "泉岳寺", "20:31"},
		{"走行中のはずが配信なし", time.Date(2026, 10, 8, 20, 33, 0, 0, jst), NotRunningNoData, "泉岳寺", "20:31"},
		{"到着の3分前は配信なし", time.Date(2026, 10, 8, 20, 41, 59, 0, jst), NotRunningNoData, "泉岳寺", "20:31"},
		// 到着の2分前からは、配信が消えていれば運行を終えたとみなす
		{"到着の2分前から", time.Date(2026, 10, 8, 20, 42, 0, 0, jst), NotRunningFinished, "西馬込", "20:44"},
		{"運行終了", time.Date(2026, 10, 8, 21, 0, 0, 0, jst), NotRunningFinished, "西馬込", "20:44"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			svc := New(&mockClient{}, loader)
			svc.now = func() time.Time { return tc.now }

			result, err := svc.GetTrainLocation(context.Background(), trainID)
			if err != nil {
				t.Fatal(err)
			}

			if result.Available {
				t.Fatal("expected unavailable train")
			}

			if result.NotRunning != tc.want ||
				result.ScheduledStation != tc.station ||
				result.ScheduledTime != tc.time {
				t.Fatalf("got %q %q %q, want %q %q %q",
					result.NotRunning, result.ScheduledStation, result.ScheduledTime,
					tc.want, tc.station, tc.time)
			}

			if result.Railway != "浅草線" || result.Destination != "西馬込" {
				t.Fatalf("unexpected railway %q / destination %q", result.Railway, result.Destination)
			}

			if !strings.Contains(result.Message, tc.station) {
				t.Fatalf("message should mention %s: %s", tc.station, result.Message)
			}
		})
	}
}

// 0時〜3時は前日の運行日として扱い、24時以降の時刻と比べる。
// 浅草線 2402T は平日ダイヤで泉岳寺 00:19 発、西馬込 00:32 着。2026-10-09 は金曜日で、0時台は 10-08（木曜日）の運行日
func TestGetTrainLocationNotRunningAfterMidnight(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	const trainID = "odpt.Train:Toei.Asakusa.2402T"
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)

	tests := []struct {
		name    string
		now     time.Time
		want    string
		station string
		time    string
	}{
		{"23時台は出発前", time.Date(2026, 10, 8, 23, 50, 0, 0, jst), NotRunningBeforeDeparture, "泉岳寺", "00:19"},
		{"0時台の出発前", time.Date(2026, 10, 9, 0, 10, 0, 0, jst), NotRunningBeforeDeparture, "泉岳寺", "00:19"},
		{"0時台の配信なし", time.Date(2026, 10, 9, 0, 25, 0, 0, jst), NotRunningNoData, "泉岳寺", "00:19"},
		{"1時台は運行終了", time.Date(2026, 10, 9, 1, 0, 0, 0, jst), NotRunningFinished, "西馬込", "00:32"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			svc := New(&mockClient{}, loader)
			svc.now = func() time.Time { return tc.now }

			result, err := svc.GetTrainLocation(context.Background(), trainID)
			if err != nil {
				t.Fatal(err)
			}

			if result.NotRunning != tc.want ||
				result.ScheduledStation != tc.station ||
				result.ScheduledTime != tc.time {
				t.Fatalf("got %q %q %q, want %q %q %q",
					result.NotRunning, result.ScheduledStation, result.ScheduledTime,
					tc.want, tc.station, tc.time)
			}
		})
	}
}

// 本日のダイヤに無い列車（平日だけ走る列車を土休日に開いたなど）は、状態を返さない
func TestGetTrainLocationNotRunningOtherCalendar(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	svc := New(&mockClient{}, loader)
	// 2026-10-11 は日曜日
	svc.now = func() time.Time {
		return time.Date(2026, 10, 11, 20, 26, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60))
	}

	result, err := svc.GetTrainLocation(context.Background(), "odpt.Train:Toei.Asakusa.2022N")
	if err != nil {
		t.Fatal(err)
	}

	if result.NotRunning != "" || result.Message != "本日のダイヤでは走らない列車です" {
		t.Fatalf("unexpected result %+v", result)
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
