package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"train-status-app/backend/internal/route"
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

		if errors.Is(err, service.ErrExternalAPI) {
			writeJSON(
				w,
				http.StatusBadGateway,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

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
//	@Param		routeId	path	string	true	"Route ID"	example(odpt.Railway:Toei.Asakusa)
//	@Success	200	{array}		service.Station
//	@Failure	400	{object}	map[string]string
//	@Failure	404	{object}	map[string]string
//	@Router		/api/routes/{routeId}/stations [get]
func (h *Handler) Stations(
	w http.ResponseWriter,
	r *http.Request,
) {
	routeID := r.PathValue("routeId")

	if routeID == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "routeId is required",
			},
		)
		return
	}

	data, err := h.service.GetStations(
		r.Context(),
		routeID,
	)
	if err != nil {

		if errors.Is(err, service.ErrStationNotFound) {
			writeJSON(
				w,
				http.StatusNotFound,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// StationDetail godoc
//
//	@Summary	Get station detail
//	@Tags		Station
//	@Produce	json
//	@Param		stationId	path	string	true	"Station ID"	example(odpt.Station:Toei.Asakusa.Shimbashi)
//	@Success	200		{object}	service.StationDetail
//	@Failure	400		{object}	map[string]string
//	@Failure	404		{object}	map[string]string
//	@Router		/api/stations/{stationId} [get]
func (h *Handler) StationDetail(
	w http.ResponseWriter,
	r *http.Request,
) {
	stationID := r.PathValue("stationId")

	if stationID == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "stationId is required",
			},
		)
		return
	}

	data, err := h.service.GetStationDetail(
		r.Context(),
		stationID,
	)
	if err != nil {

		if errors.Is(err, service.ErrStationNotFound) {
			writeJSON(
				w,
				http.StatusNotFound,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

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
//	@Param		trainId	path	string	true	"Train ID"	example(odpt.Train:Toei.Mita.1740T)
//	@Success	200		{object}	service.TrainLocation
//	@Failure	400		{object}	map[string]string
//	@Failure	404		{object}	map[string]string
//	@Failure	502		{object}	map[string]string
//	@Router		/api/trains/{trainId}/location [get]
func (h *Handler) TrainLocation(
	w http.ResponseWriter,
	r *http.Request,
) {
	trainID := r.PathValue("trainId")

	if trainID == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "trainId is required",
			},
		)
		return
	}

	data, err := h.service.GetTrainLocation(
		r.Context(),
		trainID,
	)

	if err != nil {

		if errors.Is(err, service.ErrTrainNotFound) {
			writeJSON(
				w,
				http.StatusNotFound,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

		if errors.Is(err, service.ErrExternalAPI) {
			writeJSON(
				w,
				http.StatusBadGateway,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

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
//	@Summary	Get fare
//	@Description	Get IC card and ticket fare between two stations
//	@Tags		Fare
//	@Produce	json
//	@Param		from	query	string	true	"From station ID"	example(odpt.Station:Toei.Oedo.Daimon)
//	@Param		to	query	string	true	"To station ID"	example(odpt.Station:Toei.Asakusa.Magome)
//	@Success	200	{object}	service.Fare
//	@Failure	400	{object}	map[string]string
//	@Failure	404	{object}	map[string]string
//	@Router		/api/fares [get]
func (h *Handler) Fare(
	w http.ResponseWriter,
	r *http.Request,
) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "from and to are required",
			},
		)
		return
	}

	data, err := h.service.GetFare(
		r.Context(),
		from,
		to,
	)
	if err != nil {

		if errors.Is(err, service.ErrFareNotFound) {
			writeJSON(
				w,
				http.StatusNotFound,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}

// Journeys godoc
//
//	@Summary		Search journeys
//	@Description	Search journeys between two stations. Returns the earliest journey for each number of transfers.
//	@Description	Stations with the same name on different lines (e.g. Shinjuku) are treated as one station.
//	@Description	Times are on the current service day (before 03:00 belongs to the previous day).
//	@Description	Current delays (per railway and direction, within the next hour) are added to the times, and suspended railways are avoided.
//	@Description	If the realtime status cannot be fetched, the timetable is used as is and delayApplied is false.
//	@Tags			Journey
//	@Produce		json
//	@Param			from			query		string	true	"From station ID"							example(odpt.Station:Toei.Mita.Kasuga)
//	@Param			to				query		string	true	"To station ID"								example(odpt.Station:Toei.Asakusa.Asakusa)
//	@Param			departAt		query		string	false	"Departure time (HH:MM). Defaults to now"	example(10:00)
//	@Param			arriveBy		query		string	false	"Arrival time (HH:MM). Cannot be used with departAt"
//	@Param			maxTransfers	query		int		false	"Maximum number of transfers (0-3)"			default(3)
//	@Param			avoid			query		string	false	"Comma-separated railway IDs to avoid"		example(odpt.Railway:Toei.Oedo)
//	@Success		200				{object}	service.JourneySearch
//	@Failure		400				{object}	map[string]string
//	@Failure		404				{object}	map[string]string
//	@Router			/api/journeys [get]
func (h *Handler) Journeys(
	w http.ResponseWriter,
	r *http.Request,
) {
	params := r.URL.Query()

	q := service.JourneyQuery{
		From:         params.Get("from"),
		To:           params.Get("to"),
		DepartAt:     params.Get("departAt"),
		ArriveBy:     params.Get("arriveBy"),
		MaxTransfers: route.DefaultMaxTransfers,
	}

	if q.From == "" || q.To == "" {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "from and to are required",
			},
		)
		return
	}

	if v := params.Get("maxTransfers"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "maxTransfers must be an integer",
				},
			)
			return
		}
		q.MaxTransfers = n
	}

	if v := params.Get("avoid"); v != "" {
		q.Avoid = strings.Split(v, ",")
	}

	data, err := h.service.SearchJourneys(
		r.Context(),
		q,
	)
	if err != nil {

		if errors.Is(err, service.ErrInvalidJourneyQuery) {
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

		if errors.Is(err, service.ErrStationNotFound) {
			writeJSON(
				w,
				http.StatusNotFound,
				map[string]string{
					"error": err.Error(),
				},
			)
			return
		}

		writeError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		data,
	)
}
