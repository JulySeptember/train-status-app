// Package calendar は運行日に適用されるODPTのダイヤ種別を判定する。
package calendar

import (
	"time"
)

const (
	Weekday         = "odpt.Calendar:Weekday"
	Saturday        = "odpt.Calendar:Saturday"
	Holiday         = "odpt.Calendar:Holiday"
	SaturdayHoliday = "odpt.Calendar:SaturdayHoliday"
)

// serviceDayStartHour より前の時刻は前日の運行日として扱う（終電は0時台まで走るため）
const serviceDayStartHour = 3

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// ServiceDate は時刻 t が属する運行日を返す。
func ServiceDate(t time.Time) time.Time {
	t = t.In(jst)

	if t.Hour() < serviceDayStartHour {
		t = t.AddDate(0, 0, -1)
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, jst)
}

// Calendars は時刻 t の運行日に該当するダイヤ種別を返す。
// 路線によって「土曜」「休日」を分けるものと「土休日」にまとめるものがあるため、
// 当てはまり得る種別をすべて返す。
func Calendars(t time.Time) []string {
	d := ServiceDate(t)

	if d.Weekday() == time.Sunday || IsHoliday(d) || isYearEndHoliday(d) {
		return []string{Holiday, SaturdayHoliday}
	}

	if d.Weekday() == time.Saturday {
		return []string{Saturday, SaturdayHoliday}
	}

	return []string{Weekday}
}

// isYearEndHoliday は年末年始（12/30〜1/3）かを返す。
// 都営交通はこの期間を土休日ダイヤで運行する。
func isYearEndHoliday(d time.Time) bool {
	switch d.Month() {
	case time.December:
		return d.Day() >= 30
	case time.January:
		return d.Day() <= 3
	}

	return false
}

// IsHoliday は d が国民の祝日・振替休日・国民の休日かを返す。
// 2022年以降の祝日法に基づく。
func IsHoliday(d time.Time) bool {
	if isNationalHoliday(d) {
		return true
	}

	return isSubstituteHoliday(d) || isCitizensHoliday(d)
}

func isNationalHoliday(d time.Time) bool {
	y, m, day := d.Date()

	switch m {
	case time.January:
		return day == 1 || day == nthMonday(y, m, 2)
	case time.February:
		return day == 11 || day == 23
	case time.March:
		return day == vernalEquinoxDay(y)
	case time.April:
		return day == 29
	case time.May:
		return day >= 3 && day <= 5
	case time.July:
		return day == nthMonday(y, m, 3)
	case time.August:
		return day == 11
	case time.September:
		return day == nthMonday(y, m, 3) || day == autumnalEquinoxDay(y)
	case time.October:
		return day == nthMonday(y, m, 2)
	case time.November:
		return day == 3 || day == 23
	}

	return false
}

// isSubstituteHoliday は、日曜の祝日以降で最初の祝日でない日（振替休日）かを返す。
func isSubstituteHoliday(d time.Time) bool {
	if isNationalHoliday(d) {
		return false
	}

	for prev := d.AddDate(0, 0, -1); isNationalHoliday(prev); prev = prev.AddDate(0, 0, -1) {
		if prev.Weekday() == time.Sunday {
			return true
		}
	}

	return false
}

// isCitizensHoliday は祝日に挟まれた平日（国民の休日）かを返す。
func isCitizensHoliday(d time.Time) bool {
	if isNationalHoliday(d) || d.Weekday() == time.Sunday {
		return false
	}

	return isNationalHoliday(d.AddDate(0, 0, -1)) &&
		isNationalHoliday(d.AddDate(0, 0, 1))
}

func nthMonday(year int, month time.Month, n int) int {
	first := time.Date(year, month, 1, 0, 0, 0, 0, jst)
	offset := (int(time.Monday) - int(first.Weekday()) + 7) % 7

	return 1 + offset + (n-1)*7
}

// 春分日・秋分日の近似式（1980〜2099年で有効）
func vernalEquinoxDay(year int) int {
	return equinoxDay(year, 20.8431)
}

func autumnalEquinoxDay(year int) int {
	return equinoxDay(year, 23.2488)
}

func equinoxDay(year int, base float64) int {
	y := year - 1980

	return int(base+0.242194*float64(y)) - y/4
}
