package calendar

import (
	"slices"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, jst)
}

// 内閣府「国民の祝日」一覧と突き合わせる
func TestIsHoliday(t *testing.T) {

	cases := map[int][]string{
		2025: {
			"01-01", "01-13", "02-11", "02-23", "02-24", "03-20",
			"04-29", "05-03", "05-04", "05-05", "05-06", "07-21",
			"08-11", "09-15", "09-23", "10-13", "11-03", "11-23", "11-24",
		},
		2026: {
			"01-01", "01-12", "02-11", "02-23", "03-20", "04-29",
			"05-03", "05-04", "05-05", "05-06", "07-20", "08-11",
			"09-21", "09-22", "09-23", "10-12", "11-03", "11-23",
		},
	}

	for year, want := range cases {

		got := make([]string, 0)

		for d := date(year, time.January, 1); d.Year() == year; d = d.AddDate(0, 0, 1) {
			if IsHoliday(d) {
				got = append(got, d.Format("01-02"))
			}
		}

		if !slices.Equal(got, want) {
			t.Fatalf(
				"%d: expected %v, got %v",
				year,
				want,
				got,
			)
		}
	}
}

func TestServiceDate(t *testing.T) {

	// 0時台は前日の運行日
	late := time.Date(2026, time.October, 8, 0, 30, 0, 0, jst)

	if got := ServiceDate(late); !got.Equal(date(2026, time.October, 7)) {
		t.Fatalf("unexpected service date %v", got)
	}

	// UTCで渡されてもJSTで判定する
	morning := time.Date(2026, time.October, 7, 21, 0, 0, 0, time.UTC)

	if got := ServiceDate(morning); !got.Equal(date(2026, time.October, 8)) {
		t.Fatalf("unexpected service date %v", got)
	}
}

func TestCalendars(t *testing.T) {

	cases := []struct {
		name string
		at   time.Time
		want []string
	}{
		{
			name: "weekday",
			at:   time.Date(2026, time.October, 7, 12, 0, 0, 0, jst),
			want: []string{Weekday},
		},
		{
			name: "saturday",
			at:   time.Date(2026, time.October, 10, 12, 0, 0, 0, jst),
			want: []string{Saturday, SaturdayHoliday},
		},
		{
			name: "sunday",
			at:   time.Date(2026, time.October, 11, 12, 0, 0, 0, jst),
			want: []string{Holiday, SaturdayHoliday},
		},
		{
			name: "national holiday on weekday",
			at:   time.Date(2026, time.October, 12, 12, 0, 0, 0, jst),
			want: []string{Holiday, SaturdayHoliday},
		},
		{
			name: "after midnight belongs to previous holiday",
			at:   time.Date(2026, time.October, 13, 0, 30, 0, 0, jst),
			want: []string{Holiday, SaturdayHoliday},
		},
		{
			name: "year end",
			at:   time.Date(2026, time.December, 30, 12, 0, 0, 0, jst),
			want: []string{Holiday, SaturdayHoliday},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Calendars(tc.at); !slices.Equal(got, tc.want) {
				t.Fatalf(
					"expected %v, got %v",
					tc.want,
					got,
				)
			}
		})
	}
}
