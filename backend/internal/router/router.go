package router

import (
	"net/http"

	"train-status-app/backend/internal/handler"
)

func New(h *handler.Handler) http.Handler {

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", h.Health)

	// Train Status
	mux.HandleFunc("GET /api/status", h.TrainStatus)

	// Railways
	mux.HandleFunc("GET /api/routes", h.Railways)

	// Stations
	mux.HandleFunc("GET /api/routes/{routeId}/stations", h.Stations)

	// Station Detail
	mux.HandleFunc("GET /api/stations/{stationId}", h.StationDetail)

	// Train Location
	mux.HandleFunc("GET /api/trains/{trainNumber}/location", h.TrainLocation)

	// Fare
	mux.HandleFunc("GET /api/fares", h.Fare)

	return mux
}
