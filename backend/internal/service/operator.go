package service

import (
	"strings"

	"train-status-app/backend/internal/client"
)

// 列車位置情報（odpt:Train）が配信されていない路線。
// 事業者ごと配信していないもの（メトロなど）は client.Sources の Location で判定する
var trainLocationUnsupported = map[string]bool{
	"odpt.Railway:Toei.NipporiToneri": true,
}

// delayUnsupported は、odpt:Train の odpt:delay が配信されない（null の）路線
var delayUnsupported = map[string]bool{
	"odpt.Railway:Toei.Arakawa": true,
}

// delayUnsupportedOperators は、odpt:Train の odpt:delay が配信されない事業者
var delayUnsupportedOperators = map[string]bool{
	"Keikyu": true,
}

// operatorLines は、列車位置の案内に使う「〇〇線内」の呼び方（列車位置を配信している事業者）
var operatorLines = map[string]string{
	"Toei":    "都営線",
	"JR-East": "JR線",
	"Keio":    "京王線",
	"Tobu":    "東武線",
	"Keikyu":  "京急線",
}

// operatorOf は、ODPT の ID（odpt.Railway:Toei.Asakusa、odpt.Train:JR-East.Yamanote.1234G、
// odpt.Operator:Toei など）から事業者の名前（Toei、JR-East）を返す。
func operatorOf(id string) string {
	_, rest, ok := strings.Cut(id, ":")
	if !ok {
		return ""
	}
	name, _, _ := strings.Cut(rest, ".")
	return name
}

// locationAvailable は、路線の列車位置情報が配信されているかを返す。
func locationAvailable(railway string) bool {
	return !trainLocationUnsupported[railway] && client.Sources[operatorOf(railway)].Location
}

// delayAvailable は、路線の列車位置情報に遅れが入っているかを返す。
func delayAvailable(railway string) bool {
	return !delayUnsupported[railway] && !delayUnsupportedOperators[operatorOf(railway)]
}

// operatorLine は、事業者の「〇〇線」の呼び方を返す。
func operatorLine(operator string) string {
	if name, ok := operatorLines[operator]; ok {
		return name
	}
	return "この事業者の路線"
}
