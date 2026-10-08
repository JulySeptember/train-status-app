package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"train-status-app/backend/internal/calendar"
	"train-status-app/backend/internal/route"
	"train-status-app/backend/internal/service"
)

// 道具の結果の件数の上限。AI に渡すトークンを減らすため
const (
	maxJourneys           = 4
	defaultNextDepartures = 5
	maxNextDepartures     = 10
	maxTimetableEntries   = 30
)

// Backend は、道具が使うアプリの照会。*service.Service が満たす
type Backend interface {
	FindStations(name string) []service.StationCandidate
	GetRailwayConditions(ctx context.Context) ([]service.RailwayCondition, error)
	SearchJourneys(ctx context.Context, q service.JourneyQuery) (*service.JourneySearch, error)
	GetDepartures(ctx context.Context, q service.DepartureQuery) (*service.Departures, error)
	GetTrainLocation(ctx context.Context, trainID string) (*service.TrainLocation, error)
	StationName(id string) string
	RailwayName(id string) string
}

// ToolOutput は道具を実行した結果。
type ToolOutput struct {
	// AI に渡す結果（JSON のオブジェクトにする）
	Content any

	// 画面に出す、何をしたかの短い説明（例: 「三田線の運行状況を確認」）
	Label string

	// search_route の結果。画面で経路をアプリのデータのまま表示するため
	Journeys *service.JourneySearch
}

// ToolSet は、AI に渡す道具の定義と実行。
type ToolSet interface {
	Definitions() []Tool
	Call(ctx context.Context, call ToolCall) ToolOutput
}

type Tools struct {
	backend Backend
}

func NewTools(b Backend) *Tools {
	return &Tools{backend: b}
}

var calendarIDs = map[string]string{
	"weekday":  calendar.Weekday,
	"saturday": calendar.Saturday,
	"holiday":  calendar.Holiday,
}

func (t *Tools) Definitions() []Tool {
	return []Tool{
		{
			Name: "find_station",
			Description: "駅名から都営交通の駅を探す。同じ名前の駅は路線ごとに返す。" +
				"見つからなければ candidates は空で、都営交通の駅ではない。駅の id は必ずこの結果から使う。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "description": "駅名（例: 春日）"}
				},
				"required": ["name"]
			}`),
		},
		{
			Name: "get_train_status",
			Description: "都営交通の全路線の現在の運行状況を返す。state は normal（平常）・delayed（遅延）・suspended（運転見合わせ）。" +
				"delayMinutes は走っている列車の遅れの目安（分）。",
			Parameters: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name: "search_route",
			Description: "2つの駅の間の経路を探す。現在の遅れと運転見合わせは反映済み（見合わせ中の路線は使わない）。" +
				"結果は乗り換え回数ごとの候補。時刻は HH:MM、durationMinutes は所要時間（分）。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"from": {"type": "string", "description": "出発駅の id（find_station の結果）"},
					"to": {"type": "string", "description": "到着駅の id（find_station の結果）"},
					"departAt": {"type": "string", "description": "出発時刻 HH:MM。省略すると現在時刻"},
					"arriveBy": {"type": "string", "description": "到着したい時刻 HH:MM。departAt とは同時に指定しない"},
					"maxTransfers": {"type": "integer", "minimum": 0, "maximum": 3, "description": "乗り換え回数の上限"},
					"avoidRailways": {"type": "array", "items": {"type": "string"}, "description": "使わない路線の id（例: odpt.Railway:Toei.Mita）"}
				},
				"required": ["from", "to"]
			}`),
		},
		{
			Name:        "get_next_departures",
			Description: "駅から次に発車する列車を、本日のダイヤで時刻順に返す。minutesFromNow は現在から何分後か。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"stationId": {"type": "string", "description": "駅の id（find_station の結果）"},
					"direction": {"type": "string", "description": "方向の id（例: odpt.RailDirection:Northbound）。省略すると全方向"},
					"after": {"type": "string", "description": "この時刻 HH:MM 以降。省略すると現在時刻"},
					"limit": {"type": "integer", "minimum": 1, "maximum": 10, "description": "件数（初期値 5）"}
				},
				"required": ["stationId"]
			}`),
		},
		{
			Name:        "get_timetable",
			Description: "駅の時刻表のうち、指定した時間帯の発車を返す（最大 30 件）。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"stationId": {"type": "string", "description": "駅の id（find_station の結果）"},
					"direction": {"type": "string", "description": "方向の id。省略すると全方向"},
					"calendar": {"type": "string", "enum": ["weekday", "saturday", "holiday"], "description": "ダイヤ。省略すると本日のダイヤ"},
					"from": {"type": "string", "description": "開始時刻 HH:MM"},
					"to": {"type": "string", "description": "終了時刻 HH:MM"}
				},
				"required": ["stationId"]
			}`),
		},
		{
			Name:        "get_train_location",
			Description: "列車の現在位置と遅れを返す。trainId は get_next_departures などの結果の列車 id。",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"trainId": {"type": "string", "description": "列車の id（例: odpt.Train:Toei.Mita.1234T）"}
				},
				"required": ["trainId"]
			}`),
		},
	}
}

// toolError は、道具の失敗を AI に伝える結果。AI はこれを読んで、聞き返すか別の方法を試す
type toolError struct {
	Error string `json:"error"`
}

func (t *Tools) Call(ctx context.Context, call ToolCall) ToolOutput {

	out, err := t.call(ctx, call)
	if err != nil {
		if out.Label == "" {
			out.Label = call.Name
		}
		out.Content = toolError{Error: err.Error()}
	}
	return out
}

func decodeArgs(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("引数を読めません: %v", err)
	}
	return nil
}

func (t *Tools) call(ctx context.Context, call ToolCall) (ToolOutput, error) {

	switch call.Name {

	case "find_station":
		var args struct {
			Name string `json:"name"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return ToolOutput{}, err
		}
		return ToolOutput{
			Label:   fmt.Sprintf("「%s」の駅を検索", args.Name),
			Content: map[string]any{"candidates": t.backend.FindStations(args.Name)},
		}, nil

	case "get_train_status":
		out := ToolOutput{Label: "運行状況を確認"}
		conds, err := t.backend.GetRailwayConditions(ctx)
		if err != nil {
			return out, errors.New("運行状況を取得できませんでした")
		}
		out.Content = map[string]any{"railways": conds}
		return out, nil

	case "search_route":
		return t.searchRoute(ctx, call.Arguments)

	case "get_next_departures":
		var args struct {
			StationID string `json:"stationId"`
			Direction string `json:"direction"`
			After     string `json:"after"`
			Limit     int    `json:"limit"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return ToolOutput{}, err
		}

		limit := args.Limit
		if limit <= 0 {
			limit = defaultNextDepartures
		}
		limit = min(limit, maxNextDepartures)

		out := ToolOutput{Label: fmt.Sprintf("%s駅の次の発車を確認", t.backend.StationName(args.StationID))}
		d, err := t.backend.GetDepartures(ctx, service.DepartureQuery{
			Station:       args.StationID,
			RailDirection: args.Direction,
			From:          args.After,
			Limit:         limit,
		})
		if err != nil {
			return out, departureError(err)
		}
		out.Content = d
		return out, nil

	case "get_timetable":
		var args struct {
			StationID string `json:"stationId"`
			Direction string `json:"direction"`
			Calendar  string `json:"calendar"`
			From      string `json:"from"`
			To        string `json:"to"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return ToolOutput{}, err
		}

		out := ToolOutput{Label: fmt.Sprintf("%s駅の時刻表を確認", t.backend.StationName(args.StationID))}

		cal := ""
		if args.Calendar != "" {
			var ok bool
			if cal, ok = calendarIDs[args.Calendar]; !ok {
				return out, fmt.Errorf("calendar は weekday・saturday・holiday のどれかです")
			}
		}

		d, err := t.backend.GetDepartures(ctx, service.DepartureQuery{
			Station:       args.StationID,
			RailDirection: args.Direction,
			Calendar:      cal,
			From:          args.From,
			To:            args.To,
			Limit:         maxTimetableEntries,
		})
		if err != nil {
			return out, departureError(err)
		}
		out.Content = d
		return out, nil

	case "get_train_location":
		var args struct {
			TrainID string `json:"trainId"`
		}
		if err := decodeArgs(call.Arguments, &args); err != nil {
			return ToolOutput{}, err
		}

		out := ToolOutput{Label: "列車の位置を確認"}
		loc, err := t.backend.GetTrainLocation(ctx, args.TrainID)
		if errors.Is(err, service.ErrTrainNotFound) {
			return out, errors.New("列車が見つかりません。trainId は時刻表の結果の列車 id を使ってください")
		}
		if err != nil {
			return out, errors.New("列車の位置を取得できませんでした")
		}
		out.Label = fmt.Sprintf("列車 %s の位置を確認", loc.TrainNumber)
		out.Content = loc
		return out, nil
	}

	return ToolOutput{}, fmt.Errorf("%s という道具はありません", call.Name)
}

func departureError(err error) error {
	switch {
	case errors.Is(err, service.ErrStationNotFound):
		return errors.New("駅が見つかりません。stationId は find_station の結果の id を使ってください")
	case errors.Is(err, service.ErrInvalidDepartureQuery):
		return errors.New("時刻は HH:MM で指定してください")
	}
	return errors.New("時刻表を取得できませんでした")
}

// 経路の結果を、AI に必要な項目に絞った形
type routeResult struct {
	DelayApplied      bool           `json:"delayApplied"`
	SuspendedRailways []string       `json:"suspendedRailways"`
	Journeys          []routeJourney `json:"journeys"`
}

type routeJourney struct {
	DepartureTime   string     `json:"departureTime"`
	ArrivalTime     string     `json:"arrivalTime"`
	DurationMinutes int        `json:"durationMinutes"`
	Transfers       int        `json:"transfers"`
	Legs            []routeLeg `json:"legs"`
}

type routeLeg struct {
	Railway      string `json:"railway"`
	RailwayName  string `json:"railwayName"`
	TrainType    string `json:"trainType"`
	Destination  string `json:"destination,omitempty"`
	From         string `json:"from"`
	To           string `json:"to"`
	Departure    string `json:"departureTime"`
	Arrival      string `json:"arrivalTime"`
	DelayMinutes int    `json:"delayMinutes,omitempty"`
}

func (t *Tools) searchRoute(ctx context.Context, raw json.RawMessage) (ToolOutput, error) {

	var args struct {
		From          string   `json:"from"`
		To            string   `json:"to"`
		DepartAt      string   `json:"departAt"`
		ArriveBy      string   `json:"arriveBy"`
		MaxTransfers  *int     `json:"maxTransfers"`
		AvoidRailways []string `json:"avoidRailways"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return ToolOutput{}, err
	}

	label := fmt.Sprintf("%s → %s の経路を検索", t.backend.StationName(args.From), t.backend.StationName(args.To))
	if len(args.AvoidRailways) > 0 {
		names := make([]string, 0, len(args.AvoidRailways))
		for _, id := range args.AvoidRailways {
			names = append(names, t.backend.RailwayName(id))
		}
		label += fmt.Sprintf("（%sを除外）", strings.Join(names, "・"))
	}
	out := ToolOutput{Label: label}

	q := service.JourneyQuery{
		From:         args.From,
		To:           args.To,
		DepartAt:     args.DepartAt,
		ArriveBy:     args.ArriveBy,
		MaxTransfers: route.DefaultMaxTransfers,
		Avoid:        args.AvoidRailways,
	}
	if args.MaxTransfers != nil {
		q.MaxTransfers = *args.MaxTransfers
	}

	search, err := t.backend.SearchJourneys(ctx, q)
	switch {
	case errors.Is(err, service.ErrStationNotFound):
		return out, errors.New("駅が見つかりません。from と to は find_station の結果の id を使ってください")
	case errors.Is(err, service.ErrInvalidJourneyQuery):
		return out, fmt.Errorf("条件が不正です: %v", err)
	case err != nil:
		return out, errors.New("経路を探せませんでした")
	}

	result := routeResult{
		DelayApplied:      search.DelayApplied,
		SuspendedRailways: []string{},
		Journeys:          []routeJourney{},
	}

	for _, r := range search.SuspendedRailways {
		result.SuspendedRailways = append(result.SuspendedRailways, r.ID)
	}

	for i, j := range search.Journeys {
		if i >= maxJourneys {
			break
		}

		item := routeJourney{
			DepartureTime:   j.DepartureTime,
			ArrivalTime:     j.ArrivalTime,
			DurationMinutes: clockDiff(j.DepartureTime, j.ArrivalTime),
			Transfers:       j.Transfers,
		}

		for _, l := range j.Legs {
			item.Legs = append(item.Legs, routeLeg{
				Railway:      l.Railway,
				RailwayName:  l.RailwayName,
				TrainType:    l.TrainTypeName,
				Destination:  l.DestinationName,
				From:         l.FromName,
				To:           l.ToName,
				Departure:    l.DepartureTime,
				Arrival:      l.ArrivalTime,
				DelayMinutes: l.DelayMinutes,
			})
		}

		result.Journeys = append(result.Journeys, item)
	}

	out.Content = result
	out.Journeys = search
	return out, nil
}

// clockDiff は "HH:MM" の2つの時刻の差（分）。日をまたぐ場合は翌日として数える
func clockDiff(from, to string) int {
	f, ok1 := clockMinutes(from)
	t, ok2 := clockMinutes(to)
	if !ok1 || !ok2 {
		return 0
	}
	d := t - f
	if d < 0 {
		d += 24 * 60
	}
	return d
}

func clockMinutes(v string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	return h*60 + m, true
}
