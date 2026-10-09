package service

// AI エージェントの道具（internal/ai）が使う照会。
// 時刻の計算（何分後か、時間帯の絞り込み）はここで行い、AI には計算済みの値を渡す。

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/client"
)

// ErrInvalidDepartureQuery は発車の照会の条件が不正なこと。メッセージに理由を含めて wrap する
var ErrInvalidDepartureQuery = errors.New("invalid departure query")

// =========================
// Railway Condition
// =========================

// 路線の運行状況の分類
const (
	RailwayNormal    = "normal"
	RailwayDelayed   = "delayed"
	RailwaySuspended = "suspended"

	// 運行情報を取得できなかった（事業者の取得に失敗した、または配信していない）
	RailwayUnknown = "unknown"
)

// 運行情報の文章に含まれていたら平常とみなす言葉
var normalWords = []string{"遅延はありません", "平常"}

type RailwayCondition struct {
	Railway     string `json:"railway"`
	RailwayName string `json:"railwayName"`

	// normal / delayed / suspended
	State string `json:"state"`

	// 方向ごとの遅れ（走っている列車の遅れの中央値）のうち大きいほう（分）
	DelayMinutes int `json:"delayMinutes"`

	// 運行情報の文章
	Text string `json:"text"`
}

// GetRailwayConditions は、路線ごとの運行状況を分類して返す。経路検索と同じキャッシュを使う。
func (s *Service) GetRailwayConditions(ctx context.Context) ([]RailwayCondition, error) {

	conds, err := s.railwayConditions(ctx)
	if err != nil {
		if errors.Is(err, client.ErrExternalAPI) {
			return nil, ErrExternalAPI
		}
		return nil, err
	}

	delays := make(map[string]int)
	for _, d := range conds.delays {
		delays[d.Railway] = max(delays[d.Railway], d.Minutes)
	}

	result := make([]RailwayCondition, 0, len(s.assets.Railways()))

	for _, r := range s.assets.Railways() {

		text := conds.texts[r.SameAs]

		item := RailwayCondition{
			Railway:      r.SameAs,
			RailwayName:  r.RailwayTitle.Ja,
			State:        RailwayNormal,
			DelayMinutes: delays[r.SameAs],
			Text:         text,
		}

		isNormalText := text == "" || slices.ContainsFunc(normalWords, func(w string) bool {
			return strings.Contains(text, w)
		})

		switch {
		case slices.Contains(conds.suspended, r.SameAs):
			item.State = RailwaySuspended
		case item.DelayMinutes > 0 || !isNormalText:
			item.State = RailwayDelayed
		case text == "" && !conds.statusOperators[operatorOf(r.Operator)]:
			item.State = RailwayUnknown
		}

		result = append(result, item)
	}

	return result, nil
}

// =========================
// Station Candidates
// =========================

type StationCandidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Railway     string `json:"railway"`
	RailwayName string `json:"railwayName"`
}

// OperatorCoverage は、アプリが扱う事業者と、その駅で経路検索ができるか。
type OperatorCoverage struct {
	Name string

	// 列車時刻表があり、経路検索に使えるか（東急・西武などは駅時刻表と運行情報だけ）
	RouteSearch bool
}

// Operators は、アプリが扱う事業者を路線一覧の順に返す。AI への指示に使う。
func (s *Service) Operators() []OperatorCoverage {

	routeSearch := make(map[string]bool)
	for _, st := range s.assets.Stations() {
		if s.routes.HasStation(st.SameAs) {
			routeSearch[st.Operator] = true
		}
	}

	var result []OperatorCoverage
	seen := make(map[string]bool)
	for _, r := range s.railwaysByOperator() {
		if seen[r.Operator] {
			continue
		}
		seen[r.Operator] = true
		result = append(result, OperatorCoverage{
			Name:        operatorName(r.Operator),
			RouteSearch: routeSearch[r.Operator],
		})
	}

	return result
}

// FindStations は、駅名（表記の揺れを含む）から駅の候補を返す。同じ名前の駅は路線ごとに返す。
func (s *Service) FindStations(name string) []StationCandidate {

	found := s.stationIndex.Find(name)

	result := make([]StationCandidate, 0, len(found))
	for _, st := range found {
		result = append(result, StationCandidate{
			ID:          st.ID,
			Name:        st.Name,
			Railway:     st.Railway,
			RailwayName: s.railwayNames[st.Railway],
		})
	}

	return result
}

// =========================
// Departures
// =========================

// DepartureQuery は駅の発車の照会。
// Calendar が空なら本日のダイヤ。From（"HH:MM"）が空なら、本日のダイヤでは現在時刻、それ以外は始発から。
type DepartureQuery struct {
	Station       string
	RailDirection string
	Calendar      string
	From          string
	To            string
	Limit         int
}

type Departure struct {
	Time string `json:"time"`

	// 現在から何分後か。本日のダイヤのときだけ
	MinutesFromNow *int `json:"minutesFromNow,omitempty"`

	RailDirection     string `json:"railDirection"`
	RailDirectionName string `json:"railDirectionName"`

	Destination string `json:"destination"`
	TrainType   string `json:"trainType"`
	TrainID     string `json:"trainId"`
}

type Departures struct {
	Station     string `json:"station"`
	StationName string `json:"stationName"`
	RailwayName string `json:"railwayName"`

	Calendar     string `json:"calendar"`
	CalendarName string `json:"calendarName"`

	Departures []Departure `json:"departures"`

	// Limit で切り詰める前の件数
	Total int `json:"total"`
}

// GetDepartures は、駅の発車を時刻順に返す。0時台・1時台の列車は運行日の最後に並ぶ。
func (s *Service) GetDepartures(ctx context.Context, q DepartureQuery) (*Departures, error) {

	detail, err := s.GetStationDetail(ctx, q.Station)
	if err != nil {
		return nil, err
	}

	now := s.now()
	nowMinutes := serviceDayMinutes(now)

	// 求めたダイヤ種別が無い路線では、まとめた方（土休日）や分けた方（土曜・休日）を使う
	wanted := []string{q.Calendar}
	switch q.Calendar {
	case "":
		wanted = calendar.Calendars(now)
	case calendar.Saturday, calendar.Holiday:
		wanted = append(wanted, calendar.SaturdayHoliday)
	case calendar.SaturdayHoliday:
		wanted = append(wanted, calendar.Saturday, calendar.Holiday)
	}

	from, to := -1, 1<<30
	if q.From != "" {
		if from, err = parseClock(q.From); err != nil {
			return nil, fmt.Errorf("%w: from: %v", ErrInvalidDepartureQuery, err)
		}
	}
	if q.To != "" {
		if to, err = parseClock(q.To); err != nil {
			return nil, fmt.Errorf("%w: to: %v", ErrInvalidDepartureQuery, err)
		}
	}

	result := &Departures{
		Station:     detail.ID,
		StationName: detail.Name,
		Departures:  []Departure{},
	}

	type item struct {
		minutes int
		Departure
	}

	var items []item

	for _, tt := range detail.Timetables {

		if !slices.Contains(wanted, tt.Calendar) {
			continue
		}
		if q.RailDirection != "" && tt.RailDirection != q.RailDirection {
			continue
		}

		// 方向ごとに同じダイヤ種別になるので、最初に見つかったものを返す
		if result.Calendar == "" {
			result.Calendar = tt.Calendar
			result.CalendarName = calendarNames[tt.Calendar]
		}
		if tt.Calendar != result.Calendar {
			continue
		}

		start := from
		if start < 0 && tt.IsToday && q.Calendar == "" {
			start = nowMinutes
		}

		for _, e := range tt.Timetables {

			m, err := parseClock(e.Time)
			if err != nil || m < start || m > to {
				continue
			}

			d := Departure{
				Time:              e.Time,
				RailDirection:     tt.RailDirection,
				RailDirectionName: railDirectionName(tt.RailDirection),
				Destination:       e.Destination,
				TrainType:         e.TrainType,
				TrainID:           e.TrainID,
			}
			if tt.IsToday {
				diff := m - nowMinutes
				d.MinutesFromNow = &diff
			}

			items = append(items, item{m, d})
		}
	}

	slices.SortStableFunc(items, func(a, b item) int { return cmp.Compare(a.minutes, b.minutes) })

	result.Total = len(items)
	if q.Limit > 0 && len(items) > q.Limit {
		items = items[:q.Limit]
	}

	for _, it := range items {
		result.Departures = append(result.Departures, it.Departure)
	}

	for _, st := range s.assets.Stations() {
		if st.SameAs == detail.ID {
			result.RailwayName = s.railwayNames[st.Railway]
			break
		}
	}

	return result, nil
}

// StationName は駅名を返す。分からない場合は ID の末尾を返す
func (s *Service) StationName(id string) string {
	return s.stationName(id)
}

// RailwayName は路線名を返す。分からない場合は ID の末尾を返す
func (s *Service) RailwayName(id string) string {
	if name, ok := s.railwayNames[id]; ok {
		return name
	}
	return lastSegment(id)
}
