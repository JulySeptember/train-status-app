package service

import (
	"fmt"
	"slices"
	"time"

	"train-status-app/backend/assets/slim"
	"train-status-app/backend/internal/calendar"
)

// 列車位置（odpt:Train）が配信されていないときの、本日のダイヤから見た列車の状態
const (
	// 都営線内の最初の駅をまだ出発していない（始発駅で発車を待っている、直通先から来る前など）
	NotRunningBeforeDeparture = "beforeDeparture"

	// 都営線内の最後の駅に着いた
	NotRunningFinished = "finished"

	// ダイヤでは都営線内を走っているはずだが、位置が配信されていない（始発駅での遅れなど）
	NotRunningNoData = "noData"
)

// scheduleGraceMinutes は、位置が配信されていない列車を出発前・運行終了とみなす、時刻表の時刻の前後の余裕（分）
const scheduleGraceMinutes = 2

// trainSchedule は、本日のダイヤでの列車の都営線内の最初と最後の停車駅と時刻（運行日の0時からの分）
type trainSchedule struct {
	railway       string
	railDirection string
	trainType     string
	destination   string

	firstStation string
	firstTime    int
	lastStation  string
	lastTime     int
}

// todaySchedule は、列車ID の本日のダイヤを列車時刻表から探す。
// 常駐するデータを増やさないよう、索引は作らずに毎回探す（約5,600本）
func (s *Service) todaySchedule(trainID string, now time.Time) (trainSchedule, bool) {

	tt := s.assets.TrainTimetables()
	calendars := calendar.Calendars(now)

	for _, train := range tt.Trains {

		if tt.String(train.Train) != trainID ||
			!slices.Contains(calendars, tt.String(train.Calendar)) ||
			len(train.Stops) == 0 {
			continue
		}

		first := train.Stops[0]
		last := train.Stops[len(train.Stops)-1]

		return trainSchedule{
			railway:       tt.String(train.Railway),
			railDirection: tt.String(train.RailDirection),
			trainType:     tt.String(train.TrainType),
			destination:   tt.String(train.Destination),

			firstStation: tt.String(first.Station),
			firstTime:    int(stopTime(first.Departure, first.Arrival)),
			lastStation:  tt.String(last.Station),
			lastTime:     int(stopTime(last.Arrival, last.Departure)),
		}, true
	}

	return trainSchedule{}, false
}

// stopTime は、優先する時刻が無ければもう一方の時刻を返す
func stopTime(preferred, other int16) int16 {
	if preferred == slim.NoTime {
		return other
	}
	return preferred
}

// describeNotRunning は、位置が配信されていない列車について、本日のダイヤから状態と案内文を作る
func (s *Service) describeNotRunning(item *TrainLocation, sc trainSchedule, now time.Time) {

	item.RailwayID = sc.railway
	item.Railway = s.railwayNames[sc.railway]
	item.RailDirection = sc.railDirection
	item.TrainTypeID = sc.trainType
	item.TrainType = s.trainTypeName(sc.trainType)
	if sc.destination != "" {
		item.Destination = s.stationName(sc.destination)
	}

	minutes := serviceDayMinutes(now)

	switch {
	// 時刻表は分単位で、odpt:Train の配信も数十秒遅れるので、出発・到着の前後に余裕を持たせる。
	// 余裕が無いと、定刻で出発した直後の列車を「遅れてまだ出発していない」と案内してしまう
	case minutes < sc.firstTime+scheduleGraceMinutes:
		item.NotRunning = NotRunningBeforeDeparture
		item.ScheduledStationID = sc.firstStation
		item.ScheduledStation = s.stationName(sc.firstStation)
		item.ScheduledTime = formatClock(sc.firstTime)
		item.Message = fmt.Sprintf(
			"%sを%sに出発する予定です。出発すると位置を表示します",
			item.ScheduledStation, item.ScheduledTime,
		)

	case minutes >= sc.lastTime-scheduleGraceMinutes:
		item.NotRunning = NotRunningFinished
		item.ScheduledStationID = sc.lastStation
		item.ScheduledStation = s.stationName(sc.lastStation)
		item.ScheduledTime = formatClock(sc.lastTime)
		item.Message = fmt.Sprintf(
			"%sに%sに着き、都営線内の運行を終えました",
			item.ScheduledStation, item.ScheduledTime,
		)

	default:
		item.NotRunning = NotRunningNoData
		item.ScheduledStationID = sc.firstStation
		item.ScheduledStation = s.stationName(sc.firstStation)
		item.ScheduledTime = formatClock(sc.firstTime)
		item.Message = fmt.Sprintf(
			"%sを%sに出発する予定の列車ですが、位置が配信されていません。遅れて、まだ出発していないことがあります",
			item.ScheduledStation, item.ScheduledTime,
		)
	}
}
