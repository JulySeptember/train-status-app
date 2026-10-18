package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"train-status-app/backend/internal/auth"
	"train-status-app/backend/internal/handler"
)

type stubVerifier struct{}

func (stubVerifier) Verify(_ context.Context, idToken string) (string, error) {
	if idToken != "good" {
		return "", auth.ErrInvalidToken
	}
	return "u1", nil
}

func newAuthApp(secure bool) http.Handler {
	h := handler.New(nil, nil).WithAuth(auth.NewService(stubVerifier{}, auth.NewMemoryStore()), "client-id", secure)
	return New(h)
}

func do(t *testing.T, app http.Handler, method, path, body string, cookie *http.Cookie, site string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s cookie in %v", name, rec.Result().Header.Values("Set-Cookie"))
	return nil
}

func TestAuthFlow(t *testing.T) {

	app := newAuthApp(true)

	rec := do(t, app, http.MethodGet, "/api/auth/config", "", nil, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"googleClientId":"client-id"`) {
		t.Fatalf("config: %d %s", rec.Code, rec.Body)
	}

	if rec := do(t, app, http.MethodGet, "/api/me", "", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me without login: %d", rec.Code)
	}

	rec = do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	cookie := sessionCookie(t, rec, "__Host-session")
	if !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode || cookie.Value == "" {
		t.Fatalf("unexpected cookie %+v", cookie)
	}
	// 利用者 ID（Google の sub）はレスポンスに出さない
	if strings.Contains(rec.Body.String(), "u1") {
		t.Fatalf("user id leaked: %s", rec.Body)
	}

	rec = do(t, app, http.MethodGet, "/api/me", "", cookie, "")
	var me map[string]any
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &me) != nil || me["createdAt"] == nil {
		t.Fatalf("me: %d %s", rec.Code, rec.Body)
	}

	rec = do(t, app, http.MethodPost, "/api/auth/logout", "", cookie, "same-origin")
	if rec.Code != http.StatusNoContent || sessionCookie(t, rec, "__Host-session").MaxAge >= 0 {
		t.Fatalf("logout: %d %v", rec.Code, rec.Result().Header.Values("Set-Cookie"))
	}
	if rec := do(t, app, http.MethodGet, "/api/me", "", cookie, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: %d", rec.Code)
	}
}

func TestDeleteMe(t *testing.T) {

	app := newAuthApp(true)

	rec := do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin")
	cookie := sessionCookie(t, rec, "__Host-session")

	// ほかのサイトからは退会させられない
	if rec := do(t, app, http.MethodDelete, "/api/me", "", cookie, "cross-site"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site delete: %d", rec.Code)
	}
	if rec := do(t, app, http.MethodGet, "/api/me", "", cookie, ""); rec.Code != http.StatusOK {
		t.Fatalf("account should remain: %d", rec.Code)
	}

	if rec := do(t, app, http.MethodDelete, "/api/me", "", cookie, "same-origin"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, app, http.MethodGet, "/api/me", "", cookie, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after delete: %d", rec.Code)
	}
	if rec := do(t, app, http.MethodDelete, "/api/me", "", nil, "same-origin"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("delete without login: %d", rec.Code)
	}
}

func TestLoginErrors(t *testing.T) {

	app := newAuthApp(false)

	tests := []struct {
		name string
		body string
		site string
		want int
	}{
		{"invalid token", `{"credential":"bad"}`, "same-origin", http.StatusUnauthorized},
		{"empty credential", `{"credential":""}`, "same-origin", http.StatusBadRequest},
		{"not json", `credential=good`, "same-origin", http.StatusBadRequest},
		{"cross-site", `{"credential":"good"}`, "cross-site", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, app, http.MethodPost, "/api/auth/google", tt.body, nil, tt.site)
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("no cookie should be set")
			}
		})
	}

	// 手元（HTTP）では Secure の付かない名前の Cookie にする
	rec := do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin")
	if c := sessionCookie(t, rec, "session"); c.Secure {
		t.Fatalf("local cookie should not be secure: %+v", c)
	}
}

func TestAuthDisabled(t *testing.T) {

	app := New(handler.New(nil, nil))

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/auth/config"},
		{http.MethodPost, "/api/auth/google"},
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/me"},
		{http.MethodDelete, "/api/me"},
	} {
		if rec := do(t, app, r.method, r.path, `{"credential":"good"}`, nil, "same-origin"); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: got %d, want 503", r.method, r.path, rec.Code)
		}
	}
}

// サーバーでセッションを延ばしたら、Cookie の期限も延ばす
func TestSessionCookieRenewed(t *testing.T) {

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	svc := auth.NewService(stubVerifier{}, auth.NewMemoryStore(), auth.WithClock(func() time.Time { return now }))
	app := New(handler.New(nil, nil).WithAuth(svc, "client-id", true))

	cookie := sessionCookie(t, do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin"), "__Host-session")

	// 1日たっていなければ送り直さない
	now = now.Add(time.Hour)
	if rec := do(t, app, http.MethodGet, "/api/me", "", cookie, ""); rec.Code != http.StatusOK || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("cookie should not be renewed yet: %d %v", rec.Code, rec.Result().Header.Values("Set-Cookie"))
	}

	now = now.Add(2 * 24 * time.Hour)
	rec := do(t, app, http.MethodGet, "/api/me", "", cookie, "")
	renewed := sessionCookie(t, rec, "__Host-session")
	if rec.Code != http.StatusOK || renewed.Value != cookie.Value || renewed.MaxAge != int(auth.SessionTTL.Seconds()) || !renewed.HttpOnly || !renewed.Secure {
		t.Fatalf("cookie should be renewed: %d %+v", rec.Code, renewed)
	}
}

// ログイン中にもう一度ログインしたら、前のセッションは使えなくなる
func TestLoginReplacesSession(t *testing.T) {

	app := newAuthApp(true)

	first := sessionCookie(t, do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin"), "__Host-session")
	second := sessionCookie(t, do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, first, "same-origin"), "__Host-session")

	if rec := do(t, app, http.MethodGet, "/api/me", "", first, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("previous session should be revoked: %d", rec.Code)
	}
	if rec := do(t, app, http.MethodGet, "/api/me", "", second, ""); rec.Code != http.StatusOK {
		t.Fatalf("new session should work: %d", rec.Code)
	}
}
