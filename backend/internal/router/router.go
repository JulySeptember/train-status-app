package router

import (
	"net/http"

	"train-status-app/backend/internal/handler"

	httpSwagger "github.com/swaggo/http-swagger"
)

func New(h *handler.Handler) http.Handler {

	mux := http.NewServeMux()

	mux.Handle(
		"/swagger/",
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
		),
	)
	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Train Status
	mux.HandleFunc("GET /api/status", h.TrainStatus)

	// Railways
	mux.HandleFunc("GET /api/routes", h.Railways)

	// Stations
	mux.HandleFunc("GET /api/stations", h.AllStations)
	mux.HandleFunc("GET /api/routes/{routeId}/stations", h.Stations)

	// Station Detail
	mux.HandleFunc("GET /api/stations/{stationId}", h.StationDetail)

	// Train Location
	mux.HandleFunc("GET /api/trains/{trainId}/location", h.TrainLocation)

	// Fare
	mux.HandleFunc("GET /api/fares", h.Fare)

	// Journey
	mux.HandleFunc("GET /api/journeys", h.Journeys)

	// AI Agent
	mux.HandleFunc("POST /api/chat", h.Chat)

	return mux
}
