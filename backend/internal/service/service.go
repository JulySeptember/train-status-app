package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/client"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/route"
	"train-status-app/backend/internal/station"
)

var (
	ErrExternalAPI     = errors.New("external api error")
	ErrStationNotFound = errors.New("station not found")
	ErrFareNotFound    = errors.New("fare not found")
	ErrTrainNotFound   = errors.New("train not found")

	// ErrInvalidJourneyQuery は経路検索の条件が不正なこと。メッセージに理由を含めて wrap する
	ErrInvalidJourneyQuery = errors.New("invalid journey query")
)

type TrainClient interface {
	GetTrainStatus(ctx context.Context) (client.Result[model.TrainStatus], error)
	GetTrainLocations(ctx context.Context) (client.Result[model.TrainLocation], error)

	// GetOperatorTrainLocations は1つの事業者の列車位置を取る。
	// その事業者を設定していないか、列車位置を配信していなければ client.ErrNoSource を返す
	GetOperatorTrainLocations(ctx context.Context, operator string) ([]model.TrainLocation, error)
}

type Service struct {
	client TrainClient
	assets *assets.Loader

	// 列車ID（odpt.Train:...）から路線・列車番号を引くための索引
	trains map[string]trainRef

	// 駅ID → 駅名（対象の駅と、都外・直通運転先の行先駅）
	stationNames map[string]string

	// 列車種別ID → 種別名
	trainTypeNames map[string]string

	// 路線ID → 路線名
	railwayNames map[string]string

	// 事業者（odpt.Operator:<Name> の Name）→ 路線ID（assets の順）
	operatorRailways map[string][]string

	// 駅ID → 経路検索の出発駅・到着駅としてまとめる駅の ID（stationGroups）
	stationGroups map[string][]string

	// 全駅の一覧（GET /api/stations）。起動後に変わらないので、起動時に作る
	allStations []StationSummary

	routes *route.Engine

	// 駅名から駅を引く索引（AI の道具で使う）
	stationIndex *station.Index

	// 経路検索に反映する運行状況のキャッシュ
	realtime realtimeCache

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
	routes := route.New(
		a.TrainTimetables(),
		transfers(a.Stations()),
		route.DefaultConfig(),
	)

	s := &Service{
		client:           c,
		assets:           a,
		trains:           indexTrains(a.StationTimetables()),
		stationNames:     indexStationNames(a.Stations(), a.DestinationStations()),
		trainTypeNames:   indexTrainTypeNames(a.TrainTypes()),
		railwayNames:     indexRailwayNames(a.Railways()),
		operatorRailways: indexOperatorRailways(a.Railways()),
		stationGroups:    stationGroups(a.Stations(), routes.HasStation),
		routes:           routes,
		stationIndex:     station.New(a.Stations()),
		now:              time.Now,
	}

	s.allStations = s.indexAllStations()

	s.warnUnknownNames()

	return s
}

func indexStationNames(
	stations []model.Station,
	destinations []model.Station,
) map[string]string {

	result := make(map[string]string, len(stations)+len(destinations)+len(throughServiceStations))

	maps.Copy(result, throughServiceStations)

	for _, st := range slices.Concat(destinations, stations) {
		result[st.SameAs] = st.StationTitle.Ja
	}

	return result
}

func indexTrainTypeNames(
	types []model.TrainType,
) map[string]string {

	result := make(map[string]string, len(types))

	for _, t := range types {
		result[t.SameAs] = t.TrainTypeTitle.Ja
	}

	return result
}

func indexRailwayNames(
	railways []model.Railway,
) map[string]string {

	result := make(map[string]string, len(railways))

	for _, r := range railways {
		result[r.SameAs] = r.RailwayTitle.Ja
	}

	return result
}

func indexOperatorRailways(
	railways []model.Railway,
) map[string][]string {

	result := make(map[string][]string)

	for _, r := range railways {
		op := operatorOf(r.Operator)
		result[op] = append(result[op], r.SameAs)
	}

	return result
}

// warnUnknownNames は、駅名・種別名が分からない行先や種別が時刻表にあればログに出す。
// assets を更新して直通運転先の駅が増えたときに気付けるようにするため。
func (s *Service) warnUnknownNames() {

	unknownStations := make(map[string]bool)
	unknownTypes := make(map[string]bool)

	for _, tt := range s.assets.StationTimetables() {
		for _, obj := range tt.StationTimetableObject {

			for _, id := range obj.DestinationStation {
				if _, ok := s.stationNames[id]; !ok {
					unknownStations[id] = true
				}
			}

			if obj.TrainType != "" {
				if _, ok := s.trainTypeNames[obj.TrainType]; !ok {
					unknownTypes[obj.TrainType] = true
				}
			}
		}
	}

	for _, id := range slices.Sorted(maps.Keys(unknownStations)) {
		log.Printf("unknown destination station: %s", id)
	}

	for _, id := range slices.Sorted(maps.Keys(unknownTypes)) {
		log.Printf("unknown train type: %s", id)
	}
}

// stationName は駅名を返す。分からない場合は ID の末尾（例: Sasazuka）を返す
func (s *Service) stationName(id string) string {
	if name, ok := s.stationNames[id]; ok {
		return name
	}
	return lastSegment(id)
}

// trainTypeName は列車種別名を返す。分からない場合は ID の末尾を返す
func (s *Service) trainTypeName(id string) string {
	if id == "" {
		return ""
	}
	if name, ok := s.trainTypeNames[id]; ok {
		return name
	}
	return lastSegment(id)
}

func lastSegment(id string) string {
	return id[strings.LastIndex(id, ".")+1:]
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
	// 路線ID（例: odpt.Railway:Toei.Asakusa）。フロントで路線一覧と並び順を揃えるのに使う
	RailwayID string `json:"railwayId"`
	Railway   string `json:"railway"`
	Status    string `json:"status"`

	// 事業者の運行情報を取得できなかったときは true（status は「運行情報を取得できませんでした」）
	Unavailable bool `json:"unavailable,omitempty"`
}

// statusUnavailableText は、運行情報を取得できなかった路線の status
const statusUnavailableText = "運行情報を取得できませんでした"

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

	texts := s.statusTexts(statuses.Items)

	failed := make(map[string]bool, len(statuses.Failed))
	for _, name := range statuses.Failed {
		failed[name] = true
	}

	items := make([]TrainStatus, 0, len(texts))

	// 路線の順に並べる。取得に失敗した事業者の路線は、平常と見分けられるように unavailable で返す
	for _, r := range s.assets.Railways() {

		item := TrainStatus{
			RailwayID: r.SameAs,
			Railway:   r.RailwayTitle.Ja,
		}

		if text, ok := texts[r.SameAs]; ok {
			item.Status = text
		} else if failed[operatorOf(r.Operator)] {
			item.Status = statusUnavailableText
			item.Unavailable = true
		} else {
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

// =========================
// Railway DTO
// =========================

type Railway struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// 路線記号（例: A）と路線の色（例: #FF535F）。GET /api/routes だけが返す。
	// ODPT が配信していない路線では空になる
	LineCode string `json:"lineCode,omitempty"`
	Color    string `json:"color,omitempty"`

	// 事業者（例: odpt.Operator:Toei）と、その表示名（例: 都営交通）。GET /api/routes だけが返す
	Operator     string `json:"operator,omitempty"`
	OperatorName string `json:"operatorName,omitempty"`
}

// =========================
// Asset
// =========================

func (s *Service) GetRailways(
	ctx context.Context,
) ([]Railway, error) {

	items := make([]Railway, 0, len(s.assets.Railways()))

	for _, r := range s.railwaysByOperator() {
		items = append(items, Railway{
			ID:           r.SameAs,
			Name:         r.RailwayTitle.Ja,
			LineCode:     r.LineCode,
			Color:        r.Color,
			Operator:     r.Operator,
			OperatorName: operatorName(r.Operator),
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

// StationSummary は、全駅の一覧（GET /api/stations）の1駅。
type StationSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	RailwayID string `json:"railwayId"`

	// 経路検索の選択肢として、この駅とまとめる駅の代表（journeyStations）。
	// 同じ名前で近くにある駅をつないだまとまりの中で、ID 順に最初の経路検索に使える駅。
	// 経路検索に使えない駅（列車時刻表の無い事業者の駅で、近くに同じ名前の駅も無いもの）では空になる
	JourneyStation string `json:"journeyStation"`
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

	return s.railwayStations(s.assets.Railways()[idx]), nil
}

// GetAllStations は、全路線の駅を路線の順・路線上の駅順に返す。
// 同じ駅が複数の路線に属することはない（駅ID は路線ごと）。
func (s *Service) GetAllStations(
	ctx context.Context,
) ([]StationSummary, error) {
	return slices.Clone(s.allStations), nil
}

func (s *Service) indexAllStations() []StationSummary {

	journey := journeyStations(s.stationGroups)
	items := make([]StationSummary, 0, len(s.assets.Stations()))

	for _, r := range s.railwaysByOperator() {
		for _, st := range s.railwayStations(r) {

			items = append(items, StationSummary{
				ID:             st.ID,
				Name:           st.Name,
				RailwayID:      r.SameAs,
				JourneyStation: journey[st.ID],
			})
		}
	}

	return items
}

// railwaysByOperator は、路線を事業者の順（operatorOrder）に並べる。事業者の中は元の順のまま。
func (s *Service) railwaysByOperator() []model.Railway {
	railways := slices.Clone(s.assets.Railways())
	slices.SortStableFunc(railways, func(a, b model.Railway) int {
		return operatorRank(a.Operator) - operatorRank(b.Operator)
	})
	return railways
}

// railwayStations は、路線の駅を路線上の駅順に返す。
func (s *Service) railwayStations(railway model.Railway) []Station {

	stationMap := make(map[string]model.Station)

	for _, station := range s.assets.Stations() {
		if station.Railway == railway.SameAs {
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

	return items
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

	// 列車種別（例: 普通、急行、エアポート快特）
	TrainTypeID string `json:"trainTypeId"`
	TrainType   string `json:"trainType"`
}

type Passenger struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// =========================
// Station Detail
// =========================

// ダイヤ種別の表示順
var calendarOrder = []string{
	calendar.Weekday,
	calendar.Saturday,
	calendar.Holiday,
	calendar.SaturdayHoliday,
}

// railwayDirections は路線の上り・下り方面を、表示する順に返す
func (s *Service) railwayDirections(railwayID string) []string {

	for _, r := range s.assets.Railways() {
		if r.SameAs == railwayID {
			return []string{
				r.AscendingRailDirection,
				r.DescendingRailDirection,
			}
		}
	}

	return nil
}

// sortDirectionTimetables は、方面（directions の順 → それ以外は ID 順）、
// ダイヤ種別（calendarOrder の順 → それ以外は ID 順）の順に並べる
func sortDirectionTimetables(
	items []DirectionTimetable,
	directions []string,
) {

	rank := func(order []string, v string) int {
		if i := slices.Index(order, v); i >= 0 {
			return i
		}
		return len(order)
	}

	slices.SortFunc(items, func(a, b DirectionTimetable) int {
		return cmp.Or(
			cmp.Compare(
				rank(directions, a.RailDirection),
				rank(directions, b.RailDirection),
			),
			cmp.Compare(a.RailDirection, b.RailDirection),
			cmp.Compare(
				rank(calendarOrder, a.Calendar),
				rank(calendarOrder, b.Calendar),
			),
			cmp.Compare(a.Calendar, b.Calendar),
		)
	})
}

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
		TrainLocationAvailable: locationAvailable(station.Railway),
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
				destination = s.stationName(obj.DestinationStation[0])
			}

			group.Timetables = append(group.Timetables, Timetable{
				Time:        time,
				TrainID:     obj.Train,
				TrainNumber: obj.TrainNumber,
				Destination: destination,
				TrainTypeID: obj.TrainType,
				TrainType:   s.trainTypeName(obj.TrainType),
			})
		}
	}

	for _, g := range groups {
		detail.Timetables = append(detail.Timetables, *g)
	}

	// map の反復順は不定なので、方面・ダイヤ種別の順に並べ直す
	sortDirectionTimetables(
		detail.Timetables,
		s.railwayDirections(station.Railway),
	)

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

	RailwayID string `json:"railwayId"`
	Railway   string `json:"railway"`

	// 列車種別（例: 普通、エアポート快特）
	TrainTypeID string `json:"trainTypeId"`
	TrainType   string `json:"trainType"`

	RailDirection string `json:"railDirection"`

	// 行先の駅名。直通運転先（他社）の駅も含む
	Destination string `json:"destination"`

	FromStationID string `json:"fromStationId"`
	FromStation   string `json:"fromStation"`
	ToStationID   string `json:"toStationId"`
	ToStation     string `json:"toStation"`

	// true の場合は fromStation に停車中（toStation は空）
	Stopped bool `json:"stopped"`

	// 遅れ（秒）。delayAvailable が false の路線（荒川線）では配信されず 0 になる
	Delay          int  `json:"delay"`
	DelayAvailable bool `json:"delayAvailable"`

	// 位置情報の配信時刻（ODPT の dc:date）
	UpdatedAt string `json:"updatedAt"`

	Available bool   `json:"available"`
	Message   string `json:"message"`

	// available が false のとき、本日のダイヤから見た状態（beforeDeparture・finished・noData）。
	// 本日のダイヤに無い列車では空
	NotRunning string `json:"notRunning,omitempty"`

	// notRunning が beforeDeparture・noData のときは出発する駅と時刻、finished のときは着いた駅と時刻
	ScheduledStationID string `json:"scheduledStationId,omitempty"`
	ScheduledStation   string `json:"scheduledStation,omitempty"`
	ScheduledTime      string `json:"scheduledTime,omitempty"`
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

	unsupported := &TrainLocation{
		TrainID:     trainID,
		TrainNumber: ref.trainNumber,
		Available:   false,
		Message:     "この路線は列車位置情報が提供されていません",
	}

	if !locationAvailable(ref.railway) {
		return unsupported, nil
	}

	// 列車の事業者にだけ問い合わせる
	trains, err := s.client.GetOperatorTrainLocations(ctx, operatorOf(trainID))
	if err != nil {
		switch {
		case errors.Is(err, client.ErrNoSource):
			// 配信はあるが、設定（ODPT_OPERATORS）で取得していない事業者
			return unsupported, nil
		case errors.Is(err, client.ErrExternalAPI):
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
			TrainID:        trainID,
			TrainNumber:    train.TrainNumber,
			RailwayID:      train.Railway,
			TrainTypeID:    train.TrainType,
			TrainType:      s.trainTypeName(train.TrainType),
			RailDirection:  train.RailDirection,
			Stopped:        train.ToStation == nil,
			Delay:          train.Delay,
			DelayAvailable: delayAvailable(train.Railway),
			UpdatedAt:      train.Date,
			Available:      true,
		}

		if railway, ok := railwayMap[train.Railway]; ok {
			item.Railway = railway.RailwayTitle.Ja
		}

		if len(train.DestinationStation) > 0 {
			item.Destination = s.stationName(train.DestinationStation[0])
		}

		if train.FromStation != nil {
			item.FromStationID = *train.FromStation
			if station, ok := stationMap[*train.FromStation]; ok {
				item.FromStation = station.StationTitle.Ja
			} else if name, ok := s.stationNames[*train.FromStation]; ok {
				// 都外の駅（行先の駅のデータにあれば）
				item.FromStation = name
			}
		}

		if train.ToStation != nil {
			item.ToStationID = *train.ToStation
			if station, ok := stationMap[*train.ToStation]; ok {
				item.ToStation = station.StationTitle.Ja
			} else if name, ok := s.stationNames[*train.ToStation]; ok {
				item.ToStation = name
			}
		}

		return item, nil
	}

	item := &TrainLocation{
		TrainID:     trainID,
		TrainNumber: ref.trainNumber,
		Available:   false,
		Message:     "現在この列車の運行情報は取得できません",
	}

	// 位置が配信されるのは走っている列車だけなので、本日のダイヤで出発前か運行を終えたかを伝える
	now := s.now()
	if sc, ok := s.todaySchedule(trainID, now); ok {
		s.describeNotRunning(item, sc, now)
	} else {
		item.Message = "本日のダイヤでは走らない列車です"
	}

	return item, nil
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

// =========================
// Journey DTO
// =========================

type JourneySearch struct {
	Journeys []Journey `json:"journeys"`

	// 遅れと運転見合わせを反映したか。運行状況を取得できなかったときは false で、時刻表どおりに探す
	DelayApplied bool `json:"delayApplied"`

	// 運転を見合わせているため使わなかった路線
	SuspendedRailways []Railway `json:"suspendedRailways"`
}

// Journey は1つの経路。出発・到着の時刻は、出発駅から最初に乗る駅まで・最後に降りる駅から到着駅まで
// 歩く時間を含む（例: 東京を指定して大手町から乗るとき）。歩かなければ徒歩の分は 0
type Journey struct {
	DepartureTime string       `json:"departureTime"`
	ArrivalTime   string       `json:"arrivalTime"`
	Transfers     int          `json:"transfers"`
	Legs          []JourneyLeg `json:"legs"`

	// 出発駅から最初に乗る駅まで歩く時間（分）
	WalkBeforeMinutes int `json:"walkBeforeMinutes"`
	// 最後に降りる駅から到着駅まで歩く時間（分）
	WalkAfterMinutes int `json:"walkAfterMinutes"`
}

// JourneyLeg は1本の列車に乗る区間。ID と日本語の名前を両方返す
type JourneyLeg struct {
	Railway     string `json:"railway"`
	RailwayName string `json:"railwayName"`

	Train       string `json:"train"`
	TrainNumber string `json:"trainNumber"`

	TrainType     string `json:"trainType"`
	TrainTypeName string `json:"trainTypeName"`

	// 大江戸線の環状部などでは行先が無く、空になる
	Destination     string `json:"destination"`
	DestinationName string `json:"destinationName"`

	From     string `json:"from"`
	FromName string `json:"fromName"`
	To       string `json:"to"`
	ToName   string `json:"toName"`

	// 遅れを足した時刻。遅れは路線・方向ごとの見込みで、現在から1時間以内の時刻にだけ足す
	DepartureTime string `json:"departureTime"`
	ArrivalTime   string `json:"arrivalTime"`

	// 乗る駅での発車の遅れ（分）。時刻表の発車時刻は departureTime からこの分を引いた時刻
	DelayMinutes int `json:"delayMinutes"`
}

// JourneyQuery は経路検索の条件。
// 同じ名前の駅（新宿・春日など）は、路線が違ってもまとめて1つの駅として扱う
// （三田線の春日を指定しても、大江戸線の春日から乗る経路を返す）。
// DepartAt と ArriveBy（"HH:MM"）はどちらか一方だけ指定でき、どちらも無ければ現在時刻に出発する。
// 時刻は現在の運行日のものとして扱う（3時前は前日の運行日。例: 23時台に "00:30" を指定すると、その夜の 0:30）。
type JourneyQuery struct {
	From string
	To   string

	DepartAt string
	ArriveBy string

	MaxTransfers int
	Avoid        []string

	// TimetableOnly なら、遅れと運転見合わせを反映せず、時刻表どおりに探す
	TimetableOnly bool
}

// =========================
// Journey
// =========================

func (s *Service) SearchJourneys(
	ctx context.Context,
	q JourneyQuery,
) (*JourneySearch, error) {

	for _, id := range []string{q.From, q.To} {
		group, ok := s.stationGroups[id]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrStationNotFound, id)
		}
		// 列車時刻表の無い事業者の駅で、近くに同じ名前の駅も無いもの
		if len(group) == 0 {
			return nil, fmt.Errorf("%w: route search is not available for %s", ErrInvalidJourneyQuery, id)
		}
	}

	from, to := s.stationGroups[q.From], s.stationGroups[q.To]

	if slices.ContainsFunc(from, func(id string) bool { return slices.Contains(to, id) }) {
		return nil, fmt.Errorf("%w: from and to must be different stations", ErrInvalidJourneyQuery)
	}

	if q.DepartAt != "" && q.ArriveBy != "" {
		return nil, fmt.Errorf("%w: specify either departAt or arriveBy", ErrInvalidJourneyQuery)
	}

	if q.MaxTransfers < 0 || q.MaxTransfers > route.DefaultMaxTransfers {
		return nil, fmt.Errorf(
			"%w: maxTransfers must be between 0 and %d",
			ErrInvalidJourneyQuery,
			route.DefaultMaxTransfers,
		)
	}

	for _, id := range q.Avoid {
		if _, ok := s.railwayNames[id]; !ok {
			return nil, fmt.Errorf("%w: unknown railway %s", ErrInvalidJourneyQuery, id)
		}
	}

	now := s.now()

	rq := route.Query{
		From:         from,
		To:           to,
		Calendars:    calendar.Calendars(now),
		MaxTransfers: q.MaxTransfers,
		Avoid:        slices.Clone(q.Avoid),
	}

	switch {
	case q.DepartAt != "":
		m, err := parseClock(q.DepartAt)
		if err != nil {
			return nil, fmt.Errorf("%w: departAt: %v", ErrInvalidJourneyQuery, err)
		}
		rq.Time = m

	case q.ArriveBy != "":
		m, err := parseClock(q.ArriveBy)
		if err != nil {
			return nil, fmt.Errorf("%w: arriveBy: %v", ErrInvalidJourneyQuery, err)
		}
		rq.Time = m
		rq.ArriveBy = true

	default:
		rq.Time = serviceDayMinutes(now)
	}

	result := &JourneySearch{
		SuspendedRailways: []Railway{},
	}

	if !q.TimetableOnly {
		result.DelayApplied = s.applyRealtime(ctx, &rq, result, now)
	}

	journeys, err := s.routes.Search(rq)
	if err != nil {
		return nil, err
	}

	result.Journeys = make([]Journey, 0, len(journeys))

	for _, j := range journeys {

		item := Journey{
			DepartureTime: formatClock(j.Departure),
			ArrivalTime:   formatClock(j.Arrival),
			Transfers:     j.Transfers(),
			Legs:          make([]JourneyLeg, 0, len(j.Legs)),

			WalkBeforeMinutes: j.Legs[0].Departure - j.Departure,
			WalkAfterMinutes:  j.Arrival - j.Legs[len(j.Legs)-1].Arrival,
		}

		for _, l := range j.Legs {

			destinationName := ""
			if l.Destination != "" {
				destinationName = s.stationName(l.Destination)
			}

			item.Legs = append(item.Legs, JourneyLeg{
				Railway:         l.Railway,
				RailwayName:     s.railwayNames[l.Railway],
				Train:           l.Train,
				TrainNumber:     l.TrainNumber,
				TrainType:       l.TrainType,
				TrainTypeName:   s.trainTypeName(l.TrainType),
				Destination:     l.Destination,
				DestinationName: destinationName,
				From:            l.From,
				FromName:        s.stationName(l.From),
				To:              l.To,
				ToName:          s.stationName(l.To),
				DepartureTime:   formatClock(l.Departure),
				ArrivalTime:     formatClock(l.Arrival),
				DelayMinutes:    l.Delay,
			})
		}

		result.Journeys = append(result.Journeys, item)
	}

	return result, nil
}

// applyRealtime は、遅れと運転見合わせを探索の条件に足し、見合わせている路線を result に入れる。
// 運行状況を取得できなかったときは何も足さずに false を返す（時刻表どおりに探す）。
func (s *Service) applyRealtime(
	ctx context.Context,
	rq *route.Query,
	result *JourneySearch,
	now time.Time,
) bool {

	conds, err := s.railwayConditions(ctx)
	if err != nil {
		log.Printf("journey search without realtime conditions: %v", err)
		return false
	}

	rq.Delays = conds.delays
	rq.DelayUntil = serviceDayMinutes(now) + delayWindowMinutes

	for _, id := range conds.suspended {
		result.SuspendedRailways = append(result.SuspendedRailways, Railway{
			ID:   id,
			Name: s.railwayNames[id],
		})
		if !slices.Contains(rq.Avoid, id) {
			rq.Avoid = append(rq.Avoid, id)
		}
	}

	return true
}

// parseClock は "HH:MM" を運行日の0時からの分にする（3時前は +24時間）。
func parseClock(v string) (int, error) {

	t, err := time.Parse("15:04", v)
	if err != nil || len(v) != 5 {
		return 0, fmt.Errorf("invalid time %q (expected HH:MM)", v)
	}

	h, m := t.Hour(), t.Minute()
	if h < calendar.ServiceDayStartHour {
		h += 24
	}

	return h*60 + m, nil
}

// serviceDayMinutes は時刻 t を、その運行日の0時からの分にする。
func serviceDayMinutes(t time.Time) int {
	return int(t.Sub(calendar.ServiceDate(t)) / time.Minute)
}

// formatClock は運行日の0時からの分を "HH:MM" にする（24時以降は 00:15 のように戻す）。
func formatClock(m int) string {
	m %= 24 * 60
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}
