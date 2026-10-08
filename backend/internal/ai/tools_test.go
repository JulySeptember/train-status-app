package ai_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/ai"
	"train-status-app/backend/internal/ai/fake"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/service"
)

// trainClient は、運行情報と列車位置を決めた値で返す service.TrainClient
type trainClient struct {
	statuses  []model.TrainStatus
	locations []model.TrainLocation
}

func (c *trainClient) GetTrainStatus(context.Context) ([]model.TrainStatus, error) {
	return c.statuses, nil
}

func (c *trainClient) GetTrainLocations(context.Context) ([]model.TrainLocation, error) {
	return c.locations, nil
}

func newTools(t *testing.T, c *trainClient) *ai.Tools {
	t.Helper()

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}
	return ai.NewTools(service.New(c, loader))
}

// call は道具を呼び、AI に渡す JSON を返す
func call(t *testing.T, tools *ai.Tools, name string, args any) (ai.ToolOutput, map[string]any) {
	t.Helper()

	out := tools.Call(t.Context(), fake.Call(name, args))

	raw, err := json.Marshal(out.Content)
	if err != nil {
		t.Fatal(err)
	}
	var content map[string]any
	if err := json.Unmarshal(raw, &content); err != nil {
		t.Fatalf("content must be a JSON object: %s", raw)
	}
	return out, content
}

func TestToolDefinitions(t *testing.T) {
	tools := newTools(t, &trainClient{})

	for _, d := range tools.Definitions() {
		var schema map[string]any
		if err := json.Unmarshal(d.Parameters, &schema); err != nil || schema["type"] != "object" {
			t.Errorf("%s: invalid parameters %s", d.Name, d.Parameters)
		}
		if d.Description == "" {
			t.Errorf("%s: description is empty", d.Name)
		}
	}
}

func TestToolFindStation(t *testing.T) {
	tools := newTools(t, &trainClient{})

	out, content := call(t, tools, "find_station", map[string]string{"name": "春日駅"})

	candidates := content["candidates"].([]any)
	if len(candidates) != 2 || out.Label != "「春日駅」の駅を検索" {
		t.Fatalf("unexpected result %v %q", content, out.Label)
	}

	_, content = call(t, tools, "find_station", map[string]string{"name": "渋谷"})
	if len(content["candidates"].([]any)) != 0 {
		t.Fatalf("expected no candidates, got %v", content)
	}
}

func TestToolTrainStatus(t *testing.T) {
	tools := newTools(t, &trainClient{
		statuses: []model.TrainStatus{
			{Railway: "odpt.Railway:Toei.Mita", TrainInformationText: model.LocalizedString{Ja: "三田線は、運転を見合わせています。"}},
		},
	})

	_, content := call(t, tools, "get_train_status", nil)

	found := false
	for _, r := range content["railways"].([]any) {
		r := r.(map[string]any)
		if r["railway"] == "odpt.Railway:Toei.Mita" {
			found = r["state"] == "suspended"
		}
	}
	if !found {
		t.Fatalf("expected Mita to be suspended: %v", content)
	}
}

func TestToolSearchRoute(t *testing.T) {
	tools := newTools(t, &trainClient{})

	out, content := call(t, tools, "search_route", map[string]any{
		"from":          "odpt.Station:Toei.Mita.Kasuga",
		"to":            "odpt.Station:Toei.Asakusa.Asakusa",
		"departAt":      "10:00",
		"avoidRailways": []string{"odpt.Railway:Toei.Oedo"},
	})

	if out.Label != "春日 → 浅草 の経路を検索（大江戸線を除外）" {
		t.Fatalf("unexpected label %q", out.Label)
	}
	if out.Journeys == nil || len(out.Journeys.Journeys) == 0 {
		t.Fatal("expected journeys for the screen")
	}

	journeys := content["journeys"].([]any)
	if len(journeys) == 0 || len(journeys) > 4 {
		t.Fatalf("unexpected number of journeys %d", len(journeys))
	}

	for _, j := range journeys {
		j := j.(map[string]any)
		if j["durationMinutes"].(float64) <= 0 {
			t.Fatalf("duration is not calculated: %v", j)
		}
		for _, l := range j["legs"].([]any) {
			if l.(map[string]any)["railway"] == "odpt.Railway:Toei.Oedo" {
				t.Fatalf("avoided railway is used: %v", j)
			}
		}
	}
}

func TestToolErrors(t *testing.T) {
	tools := newTools(t, &trainClient{})

	tests := []struct {
		name string
		args any
		want string
	}{
		{"search_route", map[string]string{"from": "春日", "to": "浅草"}, "find_station"},
		{"search_route", map[string]any{"from": "odpt.Station:Toei.Mita.Kasuga", "to": "odpt.Station:Toei.Asakusa.Asakusa", "departAt": "10:00", "arriveBy": "11:00"}, "条件が不正"},
		{"get_next_departures", map[string]string{"stationId": "春日"}, "find_station"},
		{"get_timetable", map[string]string{"stationId": "odpt.Station:Toei.Asakusa.Asakusa", "calendar": "sunday"}, "calendar"},
		{"get_timetable", map[string]string{"stationId": "odpt.Station:Toei.Asakusa.Asakusa", "from": "8時"}, "HH:MM"},
		{"get_train_location", map[string]string{"trainId": "unknown"}, "列車が見つかりません"},
		{"unknown_tool", nil, "ありません"},
	}

	for _, tt := range tests {
		_, content := call(t, tools, tt.name, tt.args)
		msg, _ := content["error"].(string)
		if !strings.Contains(msg, tt.want) {
			t.Errorf("%s %v: expected error containing %q, got %v", tt.name, tt.args, tt.want, content)
		}
	}
}

func TestToolDepartures(t *testing.T) {
	tools := newTools(t, &trainClient{})

	out, content := call(t, tools, "get_next_departures", map[string]any{
		"stationId": "odpt.Station:Toei.Asakusa.Asakusa",
		"limit":     50,
	})
	if out.Label != "浅草駅の次の発車を確認" {
		t.Fatalf("unexpected label %q", out.Label)
	}
	// 件数は最大 10 件に絞る
	if n := len(content["departures"].([]any)); n > 10 {
		t.Fatalf("expected at most 10 departures, got %d", n)
	}

	_, content = call(t, tools, "get_timetable", map[string]any{
		"stationId": "odpt.Station:Toei.Asakusa.Asakusa",
		"calendar":  "weekday",
	})
	if n := len(content["departures"].([]any)); n != 30 {
		t.Fatalf("expected 30 timetable entries, got %d", n)
	}
	if content["calendarName"] != "平日" {
		t.Fatalf("unexpected calendar %v", content["calendarName"])
	}
}
