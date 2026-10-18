package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"train-status-app/backend/internal/favorite"
)

// WithFavorites は、お気に入りを使えるようにする（ログインも要る）
func (h *Handler) WithFavorites(f *favorite.Service) *Handler {
	h.favorites = f
	return h
}

// favoriteSession は、お気に入りを使えてログインしていれば利用者 ID を返す。そうでなければエラーを返して false
func (h *Handler) favoriteSession(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h.favorites == nil {
		writeMessage(w, http.StatusServiceUnavailable, "お気に入りは使えません")
		return "", false
	}
	sess, ok := h.session(w, r)
	if !ok {
		return "", false
	}
	return sess.UserID, true
}

func writeFavoriteError(w http.ResponseWriter, err error) {
	var limit *favorite.LimitError
	switch {
	case errors.Is(err, favorite.ErrInvalid):
		writeMessage(w, http.StatusBadRequest, "登録できない駅・路線・経路です")
	case errors.As(err, &limit):
		writeMessage(w, http.StatusConflict, fmt.Sprintf("お気に入りは%d件まで登録できます", limit.Limit))
	case errors.Is(err, favorite.ErrUserNotFound):
		writeMessage(w, http.StatusUnauthorized, "ログインしていません")
	default:
		log.Printf("favorites: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
	}
}

// Favorites godoc
//
//	@Summary		Get favorites
//	@Description	Stations, railways and journeys the logged-in user registered, in the order added.
//	@Tags			Favorites
//	@Produce		json
//	@Success		200	{object}	favorite.Favorites
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/api/me/favorites [get]
func (h *Handler) Favorites(w http.ResponseWriter, r *http.Request) {

	userID, ok := h.favoriteSession(w, r)
	if !ok {
		return
	}

	favorites, err := h.favorites.List(r.Context(), userID)
	if err != nil {
		writeFavoriteError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, favorites)
}

// AddFavorite godoc
//
//	@Summary		Add a favorite
//	@Description	kind is stations, railways or journeys. The id of a journey is "<from station ID>~<to station ID>".
//	@Tags			Favorites
//	@Param			kind	path	string	true	"stations, railways or journeys"
//	@Param			id		path	string	true	"station ID, railway ID or <from>~<to>"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		409	{object}	map[string]string	"limit reached"
//	@Failure		500	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/api/me/favorites/{kind}/{id} [put]
func (h *Handler) AddFavorite(w http.ResponseWriter, r *http.Request) {

	userID, ok := h.favoriteSession(w, r)
	if !ok {
		return
	}

	if err := h.favorites.Add(r.Context(), userID, favorite.Kind(r.PathValue("kind")), r.PathValue("id")); err != nil {
		writeFavoriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveFavorite godoc
//
//	@Summary	Remove a favorite
//	@Tags		Favorites
//	@Param		kind	path	string	true	"stations, railways or journeys"
//	@Param		id		path	string	true	"station ID, railway ID or <from>~<to>"
//	@Success	204
//	@Failure	400	{object}	map[string]string
//	@Failure	401	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me/favorites/{kind}/{id} [delete]
func (h *Handler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {

	userID, ok := h.favoriteSession(w, r)
	if !ok {
		return
	}

	if err := h.favorites.Remove(r.Context(), userID, favorite.Kind(r.PathValue("kind")), r.PathValue("id")); err != nil {
		writeFavoriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
