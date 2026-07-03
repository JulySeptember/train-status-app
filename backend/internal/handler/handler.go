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

// Health godoc
//
//	@Summary	Health Check
//	@Tags		Health
//	@Produce	json
//	@Success	200	{object}	map[string]string
//	@Router		/health [get]
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

// TrainStatus godoc
//
//	@Summary	Get Train Status
//	@Tags		Status
//	@Produce	json
//	@Success	200	{array}		service.TrainStatus
//	@Failure	500	{object}	map[string]string
//	@Router		/api/status [get]
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

// Railways godoc
//
//	@Summary	Get railways
//	@Tags		Railway
//	@Produce	json
//	@Success	200	{array}		service.Railway
//	@Failure	500	{object}	map[string]string
//	@Router		/api/routes [get]
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

// Stations godoc
//
//	@Summary	Get stations by route
//	@Tags		Station
//	@Produce	json
//	@Param		routeId	path		string	true	"Route ID"
//	@Success	200		{array}		service.Station
//	@Failure	400		{object}	map[string]string
//	@Failure	404		{object}	map[string]string
//	@Router		/api/routes/{routeId}/stations [get]
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

// StationDetail godoc
//
//	@Summary	Get station detail
//	@Tags		Station
//	@Produce	json
//	@Param		stationId	path		string	true	"Station ID"
//	@Success	200			{object}	service.StationDetail
//	@Failure	400			{object}	map[string]string
//	@Failure	404			{object}	map[string]string
//	@Router		/api/stations/{stationId} [get]
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

// TrainLocation godoc
//
//	@Summary	Get train location
//	@Tags		Train
//	@Produce	json
//	@Param		trainNumber	path		string	true	"Train Number"
//	@Success	200			{object}	service.TrainLocation
//	@Failure	400			{object}	map[string]string
//	@Failure	404			{object}	map[string]string
//	@Router		/api/trains/{trainNumber}/location [get]
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

// Fare godoc
//
// @Summary     Get fare
// @Description Get IC card and ticket fare between two stations.
// @Tags        Fare
// @Produce     json
// @Param       from query string true "From station ID" example(odpt.Station:Toei.Oedo.Daimon)
// @Param       to   query string true "To station ID" example(odpt.Station:Toei.Asakusa.Magome)
// @Success     200 {object} service.Fare
// @Failure     400 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Router      /api/fares [get]
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
