package service

// 方向の日本語名（フロントの lib/odpt.ts の directionLabel と同じ）
var railDirectionNames = map[string]string{
	"odpt.RailDirection:Southbound":       "南行",
	"odpt.RailDirection:Northbound":       "北行",
	"odpt.RailDirection:Eastbound":        "東行",
	"odpt.RailDirection:Westbound":        "西行",
	"odpt.RailDirection:InnerLoop":        "内回り",
	"odpt.RailDirection:OuterLoop":        "外回り",
	"odpt.RailDirection:Inbound":          "上り",
	"odpt.RailDirection:Outbound":         "下り",
	"odpt.RailDirection:Toei.Waseda":      "早稲田方面",
	"odpt.RailDirection:Toei.Minowabashi": "三ノ輪橋方面",
}

// ダイヤ種別の日本語名
var calendarNames = map[string]string{
	"odpt.Calendar:Weekday":         "平日",
	"odpt.Calendar:Saturday":        "土曜",
	"odpt.Calendar:Holiday":         "休日",
	"odpt.Calendar:SaturdayHoliday": "土休日",
}

func railDirectionName(id string) string {
	if name, ok := railDirectionNames[id]; ok {
		return name
	}
	return lastSegment(id)
}
