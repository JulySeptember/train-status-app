// Package guide は、駅の中の案内（何両目に乗るか・どこへ歩くか）の表の型。
//
// 表は cmd/gen-assets -kind guide が、JR東日本の歩行空間ネットワークデータ・構内地図（チャレンジ限定ライセンス）と、
// このディレクトリの駅ごとの対応表（<駅>.yaml）から作り、assets/extra/station_guide.json に置く（コミットしない）。
// 設計は docs/design/station-guide.md。
package guide

import (
	"encoding/json"
	"fmt"
)

// 歩く速さ（m/分）。乗り換え時間の式（docs/design/multi-operator.md 6.2）と同じ
const WalkMetersPerMinute = 80

// CarMeters は1両の長さ（m）
const CarMeters = 20

// ホームを出るときに使う設備（Entry.Via）
const (
	ViaStairs    = "stairs"
	ViaEscalator = "escalator"
	ViaElevator  = "elevator"
	ViaSlope     = "slope"
)

type Table struct {
	Stations []Station `json:"stations"`
}

// Station は1つの駅（同じ名前の JR の駅）の案内。
type Station struct {
	// この案内を使う駅ID（路線ごとの駅。例: odpt.Station:JR-East.Yamanote.Shinjuku）
	StationIDs []string `json:"stationIds"`

	// 乗り換え先の他社の駅ID → 使う改札の名前
	Transfers map[string]string `json:"transfers"`

	Entries []Entry `json:"entries"`
}

// Entry は、ある列車（路線・方向）で降りて、ある行き先へ歩くときの案内。
type Entry struct {
	Railway   string `json:"railway"`
	Direction string `json:"direction"`

	Target Target `json:"target"`

	// 前から何両目に乗るとよいか。両数が決まらない路線では 0
	CarFromFront int `json:"carFromFront,omitempty"`
	Cars         int `json:"cars,omitempty"`

	// ホームを出るときに使う設備。平らな通路だけで行けるときは空
	Via string `json:"via,omitempty"`

	WalkMeters  int `json:"walkMeters"`
	WalkMinutes int `json:"walkMinutes"`
}

// Target は行き先。JR の別のホーム（路線・方向・番線）か、改札（名前）のどちらか。
type Target struct {
	Railway   string `json:"railway,omitempty"`
	Direction string `json:"direction,omitempty"`
	Track     string `json:"track,omitempty"`

	Gate string `json:"gate,omitempty"`
}

// Decode は表を読み込み、行き先がホームか改札のどちらか1つになっているかを確かめる。
func Decode(data []byte) (*Table, error) {

	var t Table
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	for _, s := range t.Stations {
		for _, e := range s.Entries {
			platform := e.Target.Railway != "" && e.Target.Direction != ""
			gate := e.Target.Gate != ""
			if platform == gate {
				return nil, fmt.Errorf("%s %s: target must be either a platform or a gate: %+v", e.Railway, e.Direction, e.Target)
			}
		}
	}

	return &t, nil
}
