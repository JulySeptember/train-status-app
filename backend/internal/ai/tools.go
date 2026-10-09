package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	Operators() []service.OperatorCoverage
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

// Scope は、AI への指示に書く対応範囲（扱う事業者と、経路検索に使えない事業者）。
func (t *Tools) Scope() string {

	var all, noRoute []string
	for _, op := range t.backend.Operators() {
		all = append(all, op.Name)
		if !op.RouteSearch {
			noRoute = append(noRoute, op.Name)
		}
	}

	scope := fmt.Sprintf("- 対象は次の事業者の路線（東京都内）です: %s。これらの駅どうしなら、事業者をまたいでも search_route で経路を探せます。\n", strings.Join(all, "・"))
	if len(noRoute) > 0 {
		scope += fmt.Sprintf("- %sの駅は、運行状況と時刻表には答えられますが、search_route では経路を探せません（近くに同じ名前のほかの事業者の駅があれば、そこから探せます）。search_route がエラーを返したら、そのことを伝えてください。\n", strings.Join(noRoute, "・"))
	}
	return scope
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
			Description: "駅名から駅を探す。同じ名前の駅は路線ごとに返す。" +
				"見つからなければ candidates は空で、このアプリが扱う駅ではない。駅の id は必ずこの結果から使う。",
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
			Description: "全路線の現在の運行状況を返す。state は normal（平常）・delayed（遅延）・suspended（運転見合わせ）・unknown（運行情報を取得できない）。" +
				"delayMinutes は走っている列車の遅れの目安（分）。",
			Parameters: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name: "search_route",
			Description: "2つの駅の間の経路を探す。現在の遅れと運転見合わせは反映済み（見合わせ中の路線は使わない）。" +
				"結果は乗り換え回数ごとの候補。時刻は HH:MM、durationMinutes は所要時間（分）。" +
				"walkBeforeMinutes・walkAfterMinutes は、出発駅から乗る駅まで・降りる駅から到着駅まで歩く時間（分。時刻に含まれる）。" +
				"10分以上遅れている路線が関わるときは、その路線を使う経路と避けた経路をアプリが比べ、comparison に返す（faster が到着の早い方）。",
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

// bigDelayMinutes 以上遅れている路線は、使う経路と避けた経路を比べる（設計書 5.2 の「大きく遅れている」）
const bigDelayMinutes = 10

// 経路の結果を、AI に必要な項目に絞った形
type routeResult struct {
	DelayApplied      bool           `json:"delayApplied"`
	SuspendedRailways []string       `json:"suspendedRailways"`
	Journeys          []routeJourney `json:"journeys"`

	// 10分以上遅れている路線が関わるときの比較（アプリが探して計算したもの）
	Comparison *routeComparison `json:"comparison,omitempty"`
}

// routeComparison は、遅れている路線を使う場合と避けた場合の比較
type routeComparison struct {
	// 遅れている路線（id）
	DelayedRailways []string `json:"delayedRailways"`

	// journeys が遅れている路線を使う経路なら "uses_delayed"、避けた経路なら "avoids_delayed"
	Journeys string `json:"journeys"`

	// もう一方の経路
	Alternative []routeJourney `json:"alternative"`

	// もう一方の最も早い到着が、journeys の最も早い到着より何分遅いか（負なら早い）
	AlternativeArrivesLaterMinutes int `json:"alternativeArrivesLaterMinutes"`

	// 到着が早いのはどちらか（"journeys" または "alternative"）
	Faster string `json:"faster"`
}

type routeJourney struct {
	DepartureTime   string     `json:"departureTime"`
	ArrivalTime     string     `json:"arrivalTime"`
	DurationMinutes int        `json:"durationMinutes"`
	Transfers       int        `json:"transfers"`
	Legs            []routeLeg `json:"legs"`

	// 出発駅から最初に乗る駅まで・最後に降りる駅から到着駅まで歩く時間（分）。時刻に含まれている
	WalkBeforeMinutes int `json:"walkBeforeMinutes,omitempty"`
	WalkAfterMinutes  int `json:"walkAfterMinutes,omitempty"`
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
		label += fmt.Sprintf("（%sを除外）", t.railwayNames(args.AvoidRailways))
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
		Journeys:          trimJourneys(search.Journeys),
	}
	for _, r := range search.SuspendedRailways {
		result.SuspendedRailways = append(result.SuspendedRailways, r.ID)
	}

	out.Journeys = search

	// 遅れている路線が関わるなら、使う場合と避けた場合を比べる。
	// AI に探し直しを任せると、比べずに避けた経路だけを探すことがあるため（評価セットで確認）
	if search.DelayApplied {
		if cmp, alt := t.compareDelayed(ctx, q, search); cmp != nil {
			result.Comparison = cmp
			out.Label += fmt.Sprintf("（遅れている%sを使う経路と比較）", t.railwayNames(cmp.DelayedRailways))
			// 画面には到着の早い方を表示する（AI もそちらを推薦する）
			if cmp.Faster == "alternative" {
				out.Journeys = alt
			}
		}
	}

	out.Content = result
	return out, nil
}

// compareDelayed は、10分以上遅れている路線について、もう一方の経路（避けていれば使う経路、使っていれば避けた経路）を探して比べる
func (t *Tools) compareDelayed(ctx context.Context, q service.JourneyQuery, search *service.JourneySearch) (*routeComparison, *service.JourneySearch) {

	conds, err := t.backend.GetRailwayConditions(ctx)
	if err != nil {
		return nil, nil
	}

	delayed := make(map[string]bool)
	for _, c := range conds {
		if c.State == service.RailwayDelayed && c.DelayMinutes >= bigDelayMinutes {
			delayed[c.Railway] = true
		}
	}
	if len(delayed) == 0 {
		return nil, nil
	}

	alt := q
	cmp := &routeComparison{}

	// 遅れている路線を避けて探していたら、使う経路と比べる
	var keep []string
	for _, id := range q.Avoid {
		if delayed[id] {
			cmp.DelayedRailways = append(cmp.DelayedRailways, id)
		} else {
			keep = append(keep, id)
		}
	}

	if len(cmp.DelayedRailways) > 0 {
		cmp.Journeys = "avoids_delayed"
		alt.Avoid = keep
	} else {
		// 遅れている路線を使う経路なら、避けた経路と比べる
		used := make(map[string]bool)
		for _, j := range search.Journeys {
			for _, l := range j.Legs {
				if delayed[l.Railway] && !used[l.Railway] {
					used[l.Railway] = true
					cmp.DelayedRailways = append(cmp.DelayedRailways, l.Railway)
				}
			}
		}
		if len(cmp.DelayedRailways) == 0 {
			return nil, nil
		}
		cmp.Journeys = "uses_delayed"
		alt.Avoid = append(slices.Clone(q.Avoid), cmp.DelayedRailways...)
	}

	altSearch, err := t.backend.SearchJourneys(ctx, alt)
	if err != nil {
		return nil, nil
	}

	cmp.Alternative = trimJourneys(altSearch.Journeys)

	main, okMain := earliestArrival(search.Journeys)
	other, okOther := earliestArrival(altSearch.Journeys)

	switch {
	case okMain && okOther:
		cmp.AlternativeArrivesLaterMinutes = other - main
		cmp.Faster = "journeys"
		if other < main {
			cmp.Faster = "alternative"
		}
	case okOther:
		cmp.Faster = "alternative"
	default:
		cmp.Faster = "journeys"
	}

	return cmp, altSearch
}

func (t *Tools) railwayNames(ids []string) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, t.backend.RailwayName(id))
	}
	return strings.Join(names, "・")
}

func trimJourneys(journeys []service.Journey) []routeJourney {

	result := []routeJourney{}

	for i, j := range journeys {
		if i >= maxJourneys {
			break
		}

		item := routeJourney{
			DepartureTime:   j.DepartureTime,
			ArrivalTime:     j.ArrivalTime,
			DurationMinutes: clockDiff(j.DepartureTime, j.ArrivalTime),
			Transfers:       j.Transfers,

			WalkBeforeMinutes: j.WalkBeforeMinutes,
			WalkAfterMinutes:  j.WalkAfterMinutes,
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

		result = append(result, item)
	}

	return result
}

// earliestArrival は、経路のうち最も早い到着（運行日の0時からの分）
func earliestArrival(journeys []service.Journey) (int, bool) {
	best, ok := 0, false
	for _, j := range journeys {
		if m, valid := serviceMinutes(j.ArrivalTime); valid && (!ok || m < best) {
			best, ok = m, true
		}
	}
	return best, ok
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

// serviceMinutes は "HH:MM" を運行日の0時からの分にする（3時前は +24時間）
func serviceMinutes(v string) (int, bool) {
	m, ok := clockMinutes(v)
	if ok && m < calendar.ServiceDayStartHour*60 {
		m += 24 * 60
	}
	return m, ok
}

func clockMinutes(v string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(v, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	return h*60 + m, true
}
