package handler

import (
	"encoding/json"
	"net/http"

	"train-status-app/backend/internal/service"
)

type Handler struct {
	service *service.Service
}

func New(s *service.Service) *Handler {
	return &Handler{
		service: s,
	}
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	v any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(
	w http.ResponseWriter,
	err error,
) {
	writeJSON(
		w,
		http.StatusInternalServerError,
		map[string]string{
			"error": err.Error(),
		},
	)
}

// =========================
// Health
// =========================

func (h *Handler) Health(
	w http.ResponseWriter,
	r *http.Request,
) {
	writeJSON(
		w,
		http.StatusOK,
		map[string]string{
			"status": "ok",
		},
	)
}

// =========================
// Train Status
// =========================

func (h *Handler) TrainStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	data, err := h.service.GetTrainStatus(
		r.Context(),
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// =========================
// Railways
// =========================

func (h *Handler) Railways(
	w http.ResponseWriter,
	r *http.Request,
) {
	data, err := h.service.GetRailways(
		r.Context(),
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// =========================
// Stations
// =========================

func (h *Handler) Stations(
	w http.ResponseWriter,
	r *http.Request,
) {

	routeID := r.PathValue("routeId")

	data, err := h.service.GetStations(
		r.Context(),
		routeID,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// =========================
// Station Detail
// =========================

func (h *Handler) StationDetail(
	w http.ResponseWriter,
	r *http.Request,
) {
	stationID := r.PathValue(
		"stationId",
	)

	data, err := h.service.GetStationDetail(
		r.Context(),
		stationID,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// =========================
// Train Location
// =========================

func (h *Handler) TrainLocation(
	w http.ResponseWriter,
	r *http.Request,
) {
	trainNumber := r.PathValue(
		"trainNumber",
	)

	data, err := h.service.GetTrainLocation(
		r.Context(),
		trainNumber,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// =========================
// Fare
// =========================

func (h *Handler) Fare(
	w http.ResponseWriter,
	r *http.Request,
) {
	from := r.URL.Query().Get(
		"from",
	)

	to := r.URL.Query().Get(
		"to",
	)

	data, err := h.service.GetFare(
		r.Context(),
		from,
		to,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}
