package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"train-status-app/backend/assets"
	"train-status-app/backend/internal/model"
	"train-status-app/backend/internal/service"
)

type nopClient struct{}

func (nopClient) GetTrainStatus(ctx context.Context) ([]model.TrainStatus, error) {
	return nil, nil
}

func (nopClient) GetTrainLocations(ctx context.Context) ([]model.TrainLocation, error) {
	return nil, nil
}

func TestJourneys(t *testing.T) {

	loader, err := assets.New()
	if err != nil {
		t.Fatal(err)
	}

	h := New(service.New(nopClient{}, loader))

	const (
		kasuga  = "odpt.Station:Toei.Mita.Kasuga"
		asakusa = "odpt.Station:Toei.Asakusa.Asakusa"
	)

	tests := []struct {
		name   string
		params url.Values
		want   int
	}{
		{"成功", url.Values{"from": {kasuga}, "to": {asakusa}, "departAt": {"10:00"}}, http.StatusOK},
		{"路線を避ける", url.Values{"from": {kasuga}, "to": {asakusa}, "avoid": {"odpt.Railway:Toei.Oedo,odpt.Railway:Toei.Shinjuku"}}, http.StatusOK},
		{"到着駅なし", url.Values{"from": {kasuga}}, http.StatusBadRequest},
		{"存在しない駅", url.Values{"from": {kasuga}, "to": {"odpt.Station:Toei.Mita.Unknown"}}, http.StatusNotFound},
		{"出発と到着の両方", url.Values{"from": {kasuga}, "to": {asakusa}, "departAt": {"10:00"}, "arriveBy": {"11:00"}}, http.StatusBadRequest},
		{"時刻の形式", url.Values{"from": {kasuga}, "to": {asakusa}, "departAt": {"10"}}, http.StatusBadRequest},
		{"乗り換え回数が数でない", url.Values{"from": {kasuga}, "to": {asakusa}, "maxTransfers": {"a"}}, http.StatusBadRequest},
		{"乗り換え回数の上限", url.Values{"from": {kasuga}, "to": {asakusa}, "maxTransfers": {"4"}}, http.StatusBadRequest},
		{"運行状況を反映しない", url.Values{"from": {kasuga}, "to": {asakusa}, "realtime": {"false"}}, http.StatusOK},
		{"運行状況の指定が真偽値でない", url.Values{"from": {kasuga}, "to": {asakusa}, "realtime": {"no"}}, http.StatusBadRequest},
		{"存在しない路線", url.Values{"from": {kasuga}, "to": {asakusa}, "avoid": {"odpt.Railway:Toei.Unknown"}}, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, "/api/journeys?"+tt.params.Encode(), nil)
			rec := httptest.NewRecorder()

			h.Journeys(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}

			if rec.Code == http.StatusOK {
				var body service.JourneySearch
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Journeys == nil {
					t.Error("journeys must not be null")
				}
			}
		})
	}
}
