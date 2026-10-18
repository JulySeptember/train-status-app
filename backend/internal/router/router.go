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

	// Journey
	mux.HandleFunc("GET /api/journeys", h.Journeys)

	// AI Agent
	mux.HandleFunc("POST /api/chat", h.Chat)

	// Auth
	mux.HandleFunc("GET /api/auth/config", h.AuthConfig)
	mux.HandleFunc("POST /api/auth/google", h.Login)
	mux.HandleFunc("POST /api/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/me", h.Me)
	mux.HandleFunc("DELETE /api/me", h.DeleteMe)

	// Favorites
	mux.HandleFunc("GET /api/me/favorites", h.Favorites)
	mux.HandleFunc("PUT /api/me/favorites/{kind}/{id}", h.AddFavorite)
	mux.HandleFunc("DELETE /api/me/favorites/{kind}/{id}", h.RemoveFavorite)

	// ほかのサイトから Cookie 付きで状態を変えるリクエストを送らせない（CSRF 対策）。
	// Sec-Fetch-Site（無ければ Origin と Host）を見て、別のオリジンからの POST・PUT・DELETE などを 403 にする
	return http.NewCrossOriginProtection().Handler(mux)
}
