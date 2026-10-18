package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"train-status-app/backend/internal/notify"
	"train-status-app/backend/internal/notify/webpush"
)

// 購読・条件の本文の上限
const maxNotifyBody = 4 << 10

// PushConfig は、ブラウザが購読するための VAPID の公開鍵
type PushConfig struct {
	PublicKey string `json:"publicKey"`
}

// AlertRequest は、通知の条件（路線はパスで指定する）
type AlertRequest struct {
	Days  []notify.Day `json:"days"`
	Start string       `json:"start"`
	End   string       `json:"end"`
}

// UnsubscribeRequest は、外す端末の購読
type UnsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// TestResult は、試しの通知を送れた端末の数
type TestResult struct {
	Sent int `json:"sent"`
}

// WithNotify は、運行情報の通知を使えるようにする（ログインも要る）
func (h *Handler) WithNotify(n *notify.Service) *Handler {
	h.notify = n
	return h
}

func (h *Handler) notifySession(w http.ResponseWriter, r *http.Request) (string, bool) {
	if h.notify == nil {
		writeMessage(w, http.StatusServiceUnavailable, "通知は使えません")
		return "", false
	}
	sess, ok := h.session(w, r)
	if !ok {
		return "", false
	}
	return sess.UserID, true
}

func writeNotifyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notify.ErrInvalid):
		writeMessage(w, http.StatusBadRequest, "登録できない内容です")
	case errors.Is(err, notify.ErrLimit):
		writeMessage(w, http.StatusConflict, fmt.Sprintf("通知する路線は%d路線まで登録できます", notify.MaxAlerts))
	case errors.Is(err, notify.ErrUserNotFound):
		writeMessage(w, http.StatusUnauthorized, "ログインしていません")
	case errors.Is(err, notify.ErrNoSubscription):
		writeMessage(w, http.StatusConflict, "通知を受け取る端末がありません")
	case errors.Is(err, notify.ErrTooSoon):
		writeMessage(w, http.StatusTooManyRequests, "1分ほどあけてから、もう一度お試しください")
	case errors.Is(err, notify.ErrUnavailable):
		log.Printf("notify: %v", err)
		writeMessage(w, http.StatusServiceUnavailable, "通知は使えません")
	default:
		log.Printf("notify: %v", err)
		writeMessage(w, http.StatusInternalServerError, "エラーが発生しました")
	}
}

func decodeNotifyBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxNotifyBody)).Decode(v); err != nil {
		writeMessage(w, http.StatusBadRequest, "リクエストが正しくありません")
		return false
	}
	return true
}

// PushConfig godoc
//
//	@Summary	Get the VAPID public key
//	@Tags		Notifications
//	@Produce	json
//	@Success	200	{object}	handler.PushConfig
//	@Failure	503	{object}	map[string]string
//	@Router		/api/push/config [get]
func (h *Handler) PushConfig(w http.ResponseWriter, r *http.Request) {
	if h.notify == nil {
		writeMessage(w, http.StatusServiceUnavailable, "通知は使えません")
		return
	}
	key, err := h.notify.PublicKey(r.Context())
	if err != nil {
		writeNotifyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, PushConfig{PublicKey: key})
}

// Alerts godoc
//
//	@Summary	Get notification settings
//	@Tags		Notifications
//	@Produce	json
//	@Success	200	{array}		notify.Alert
//	@Failure	401	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me/alerts [get]
func (h *Handler) Alerts(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	alerts, err := h.notify.Alerts(r.Context(), userID)
	if err != nil {
		writeNotifyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alerts)
}

// SetAlert godoc
//
//	@Summary		Set the notification setting of a railway
//	@Description	days are weekday and/or holiday. start and end are HH:MM in 30-minute steps (start <= now < end).
//	@Tags			Notifications
//	@Accept			json
//	@Param			railwayId	path	string					true	"railway ID"
//	@Param			body		body	handler.AlertRequest	true	"days and time window"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		409	{object}	map[string]string	"limit reached"
//	@Failure		500	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/api/me/alerts/{railwayId} [put]
func (h *Handler) SetAlert(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	var req AlertRequest
	if !decodeNotifyBody(w, r, &req) {
		return
	}
	err := h.notify.SetAlert(r.Context(), userID, notify.Alert{
		RailwayID: r.PathValue("railwayId"),
		Days:      req.Days,
		Start:     req.Start,
		End:       req.End,
	})
	if err != nil {
		writeNotifyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveAlert godoc
//
//	@Summary	Remove the notification setting of a railway
//	@Tags		Notifications
//	@Param		railwayId	path	string	true	"railway ID"
//	@Success	204
//	@Failure	401	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me/alerts/{railwayId} [delete]
func (h *Handler) RemoveAlert(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	if err := h.notify.RemoveAlert(r.Context(), userID, r.PathValue("railwayId")); err != nil {
		writeNotifyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Subscribe godoc
//
//	@Summary		Register this device for push notifications
//	@Description	The body is PushSubscription.toJSON() of the browser. Only known push services are accepted.
//	@Tags			Notifications
//	@Accept			json
//	@Param			body	body	webpush.Subscription	true	"push subscription"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		401	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Failure		503	{object}	map[string]string
//	@Router			/api/me/push-subscriptions [post]
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	var sub webpush.Subscription
	if !decodeNotifyBody(w, r, &sub) {
		return
	}
	if err := h.notify.Subscribe(r.Context(), userID, sub); err != nil {
		writeNotifyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Unsubscribe godoc
//
//	@Summary	Stop push notifications on this device
//	@Tags		Notifications
//	@Accept		json
//	@Param		body	body	handler.UnsubscribeRequest	true	"endpoint of the subscription"
//	@Success	204
//	@Failure	400	{object}	map[string]string
//	@Failure	401	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me/push-subscriptions [delete]
func (h *Handler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	var req UnsubscribeRequest
	if !decodeNotifyBody(w, r, &req) {
		return
	}
	if err := h.notify.Unsubscribe(r.Context(), userID, req.Endpoint); err != nil {
		writeNotifyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SendTestNotification godoc
//
//	@Summary	Send a test notification to the user's devices
//	@Tags		Notifications
//	@Produce	json
//	@Success	200	{object}	handler.TestResult
//	@Failure	401	{object}	map[string]string
//	@Failure	409	{object}	map[string]string	"no device"
//	@Failure	429	{object}	map[string]string
//	@Failure	500	{object}	map[string]string
//	@Failure	503	{object}	map[string]string
//	@Router		/api/me/push-subscriptions/test [post]
func (h *Handler) SendTestNotification(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.notifySession(w, r)
	if !ok {
		return
	}
	sent, err := h.notify.SendTest(r.Context(), userID)
	if err != nil {
		writeNotifyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, TestResult{Sent: sent})
}
