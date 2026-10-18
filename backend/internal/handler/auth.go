package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"train-status-app/backend/internal/auth"
)

const (
	// 本番（HTTPS）では __Host- を付け、Secure・Path=/ で、ほかのサブドメインから上書きできないようにする。
	// 手元（http://localhost）では Secure が付けられないので、付けない名前にする
	secureSessionCookie = "__Host-session"
	localSessionCookie  = "session"

	// ログインの本文の上限。ID トークンは 2KB ほど
	maxLoginBody = 16 << 10
)

// AuthConfig は、フロントがログインのボタンを出すための設定
type AuthConfig struct {
	GoogleClientID string `json:"googleClientId"`
}

// LoginRequest は、Google Identity Services が返した ID トークン
type LoginRequest struct {
	Credential string `json:"credential"`
}

// LoginResponse は、ログインした利用者
type LoginResponse struct {
	User auth.User `json:"user"`
}

// WithAuth は、ログインを使えるようにする。secureCookie は HTTPS で動くとき（Lambda）に true
func (h *Handler) WithAuth(a *auth.Service, googleClientID string, secureCookie bool) *Handler {
	h.auth = a
	h.googleClientID = googleClientID
	h.secureCookie = secureCookie
	return h
}

func (h *Handler) sessionCookieName() string {
	if h.secureCookie {
		return secureSessionCookie
	}
	return localSessionCookie
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName(),
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) sessionToken(r *http.Request) string {
	c, err := r.Cookie(h.sessionCookieName())
	if err != nil {
		return ""
	}
	return c.Value
}

func writeMessage(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// authAvailable は、ログインが使えなければ 503 を返して false を返す
func (h *Handler) authAvailable(w http.ResponseWriter) bool {
	if h.auth == nil {
		writeMessage(w, http.StatusServiceUnavailable, "ログインは使えません")
		return false
	}
	return true
}

// session は、ログイン中のセッションを返す。ログインしていなければ 401 を返して false を返す
func (h *Handler) session(w http.ResponseWriter, r *http.Request) (auth.Session, bool) {

	if !h.authAvailable(w) {
		return auth.Session{}, false
	}

	sess, err := h.auth.Authenticate(r.Context(), h.sessionToken(r))
	if errors.Is(err, auth.ErrUnauthenticated) {
		h.clearSessionCookie(w)
		writeMessage(w, http.StatusUnauthorized, "ログインしていません")
		return auth.Session{}, false
	}
	if err != nil {
		log.Printf("authenticate: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
		return auth.Session{}, false
	}

	return sess, true
}

// AuthConfig godoc
//
//	@Summary	Get login settings
//	@Tags		Auth
//	@Produce	json
//	@Success	200	{object}	handler.AuthConfig
//	@Failure	503	{object}	map[string]string
//	@Router		/api/auth/config [get]
func (h *Handler) AuthConfig(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable(w) {
		return
	}
	writeJSON(w, http.StatusOK, AuthConfig{GoogleClientID: h.googleClientID})
}

// Login godoc
//
//	@Summary		Log in with a Google ID token
//	@Description	Verifies the ID token from Google Identity Services and sets the session cookie.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		handler.LoginRequest	true	"ID token"
//	@Success		200		{object}	handler.LoginResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Failure		503		{object}	map[string]string
//	@Router			/api/auth/google [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {

	if !h.authAvailable(w) {
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&req); err != nil || req.Credential == "" {
		writeMessage(w, http.StatusBadRequest, "リクエストが正しくありません")
		return
	}

	token, user, err := h.auth.Login(r.Context(), req.Credential)
	if errors.Is(err, auth.ErrInvalidToken) {
		// トークンの中身はログに出さない
		log.Printf("login: %v", err)
		writeMessage(w, http.StatusUnauthorized, "ログインできませんでした")
		return
	}
	if err != nil {
		log.Printf("login: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
		return
	}

	h.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, LoginResponse{User: user})
}

// Logout godoc
//
//	@Summary	Log out
//	@Tags		Auth
//	@Success	204
//	@Failure	503	{object}	map[string]string
//	@Router		/api/auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {

	if !h.authAvailable(w) {
		return
	}

	if err := h.auth.Logout(r.Context(), h.sessionToken(r)); err != nil {
		log.Printf("logout: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
		return
	}

	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me godoc
//
//	@Summary	Get the logged-in user
//	@Tags		Auth
//	@Produce	json
//	@Success	200	{object}	auth.User
//	@Failure	401	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me [get]
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {

	sess, ok := h.session(w, r)
	if !ok {
		return
	}

	user, err := h.auth.Me(r.Context(), sess)
	if errors.Is(err, auth.ErrUnauthenticated) {
		h.clearSessionCookie(w)
		writeMessage(w, http.StatusUnauthorized, "ログインしていません")
		return
	}
	if err != nil {
		log.Printf("me: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// DeleteMe godoc
//
//	@Summary		Delete the account
//	@Description	Deletes all data of the logged-in user and the sessions.
//	@Tags			Auth
//	@Success		204
//	@Failure		401	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/api/me [delete]
func (h *Handler) DeleteMe(w http.ResponseWriter, r *http.Request) {

	sess, ok := h.session(w, r)
	if !ok {
		return
	}

	if err := h.auth.DeleteAccount(r.Context(), sess); err != nil {
		log.Printf("delete account: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
		return
	}

	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
