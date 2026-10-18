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
	"train-status-app/backend/internal/favorite"
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

type stubCatalog struct{}

func (stubCatalog) HasStation(id string) bool {
	return id == "odpt.Station:Toei.Asakusa.Asakusa" || id == "odpt.Station:Toei.Oedo.Shinjuku"
}
func (stubCatalog) HasRailway(id string) bool { return id == "odpt.Railway:Toei.Mita" }

func TestFavorites(t *testing.T) {

	store := auth.NewMemoryStore()
	h := handler.New(nil, nil).
		WithAuth(auth.NewService(stubVerifier{}, store), "client-id", true).
		WithFavorites(favorite.NewService(store, stubCatalog{}))
	app := New(h)

	if rec := do(t, app, http.MethodGet, "/api/me/favorites", "", nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("favorites without login: %d", rec.Code)
	}

	cookie := sessionCookie(t, do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin"), "__Host-session")

	// ID は encodeURIComponent した形で届く
	for _, path := range []string{
		"/api/me/favorites/stations/odpt.Station%3AToei.Oedo.Shinjuku",
		"/api/me/favorites/railways/odpt.Railway%3AToei.Mita",
		"/api/me/favorites/journeys/odpt.Station%3AToei.Asakusa.Asakusa~odpt.Station%3AToei.Oedo.Shinjuku",
	} {
		if rec := do(t, app, http.MethodPut, path, "", cookie, "same-origin"); rec.Code != http.StatusNoContent {
			t.Fatalf("PUT %s: %d %s", path, rec.Code, rec.Body)
		}
	}

	rec := do(t, app, http.MethodGet, "/api/me/favorites", "", cookie, "")
	want := `{"stations":["odpt.Station:Toei.Oedo.Shinjuku"],"railways":["odpt.Railway:Toei.Mita"],"journeys":[{"from":"odpt.Station:Toei.Asakusa.Asakusa","to":"odpt.Station:Toei.Oedo.Shinjuku"}]}`
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("favorites: %d %s", rec.Code, rec.Body)
	}

	if rec := do(t, app, http.MethodPut, "/api/me/favorites/stations/odpt.Station%3ANowhere", "", cookie, "same-origin"); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown station: %d", rec.Code)
	}
	if rec := do(t, app, http.MethodPut, "/api/me/favorites/trains/x", "", cookie, "same-origin"); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown kind: %d", rec.Code)
	}
	if rec := do(t, app, http.MethodPut, "/api/me/favorites/railways/odpt.Railway%3AToei.Mita", "", cookie, "cross-site"); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site: %d", rec.Code)
	}

	if rec := do(t, app, http.MethodDelete, "/api/me/favorites/railways/odpt.Railway%3AToei.Mita", "", cookie, "same-origin"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	// データの更新で無くなった駅も外せる
	if rec := do(t, app, http.MethodDelete, "/api/me/favorites/stations/odpt.Station%3ANowhere", "", cookie, "same-origin"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete removed station: %d", rec.Code)
	}
	rec = do(t, app, http.MethodGet, "/api/me/favorites", "", cookie, "")
	if !strings.Contains(rec.Body.String(), `"railways":[]`) {
		t.Fatalf("railway should be removed: %s", rec.Body)
	}

	// 退会したら、お気に入りも消える
	if rec := do(t, app, http.MethodDelete, "/api/me", "", cookie, "same-origin"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete account: %d", rec.Code)
	}
	if items, _ := store.ListFavorites(t.Context(), "u1"); len(items) != 0 {
		t.Fatalf("favorites left after account deletion: %+v", items)
	}
}

func TestFavoritesLimitMessage(t *testing.T) {

	store := auth.NewMemoryStore()
	app := New(handler.New(nil, nil).
		WithAuth(auth.NewService(stubVerifier{}, store), "client-id", true).
		WithFavorites(favorite.NewService(store, stubCatalog{})))

	cookie := sessionCookie(t, do(t, app, http.MethodPost, "/api/auth/google", `{"credential":"good"}`, nil, "same-origin"), "__Host-session")

	saved := favorite.Limits[favorite.Railways]
	favorite.Limits[favorite.Railways] = 0
	t.Cleanup(func() { favorite.Limits[favorite.Railways] = saved })

	rec := do(t, app, http.MethodPut, "/api/me/favorites/railways/odpt.Railway%3AToei.Mita", "", cookie, "same-origin")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "0件まで") {
		t.Fatalf("limit: %d %s", rec.Code, rec.Body)
	}
}

func TestFavoritesDisabled(t *testing.T) {
	app := newAuthApp(true)
	if rec := do(t, app, http.MethodGet, "/api/me/favorites", "", nil, ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("favorites without service: %d", rec.Code)
	}
}
