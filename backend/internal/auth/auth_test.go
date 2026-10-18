package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubVerifier struct {
	sub string
	err error
}

func (s stubVerifier) Verify(context.Context, string) (string, error) { return s.sub, s.err }

func newTestService(now *time.Time, v Verifier) (*Service, *MemoryStore) {
	store := NewMemoryStore()
	s := NewService(v, store)
	s.now = func() time.Time { return *now }
	return s, store
}

func TestLoginAndAuthenticate(t *testing.T) {

	ctx := context.Background()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s, store := newTestService(&now, stubVerifier{sub: "u1"})

	token, user, err := s.Login(ctx, "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != "u1" || !user.CreatedAt.Equal(now) {
		t.Fatalf("unexpected user %+v", user)
	}

	// セッション ID そのものは保存しない
	if _, err := store.GetSession(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatal("raw token must not be stored")
	}

	sess, err := s.Authenticate(ctx, token)
	if err != nil || sess.UserID != "u1" {
		t.Fatalf("unexpected session %+v %v", sess, err)
	}

	// 2回目のログインでも、作成日は変わらない
	now = now.Add(time.Hour)
	_, user2, err := s.Login(ctx, "id-token")
	if err != nil || !user2.CreatedAt.Equal(user.CreatedAt) {
		t.Fatalf("created at changed: %+v %v", user2, err)
	}
}

func TestLoginInvalidToken(t *testing.T) {
	now := time.Now()
	s, _ := newTestService(&now, stubVerifier{err: ErrInvalidToken})
	if _, _, err := s.Login(context.Background(), "bad"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestAuthenticateExpiryAndExtension(t *testing.T) {

	ctx := context.Background()
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	now := start
	s, store := newTestService(&now, stubVerifier{sub: "u1"})

	token, _, err := s.Login(ctx, "id-token")
	if err != nil {
		t.Fatal(err)
	}

	// 1日たっていなければ延ばさない
	now = start.Add(23 * time.Hour)
	sess, err := s.Authenticate(ctx, token)
	if err != nil || !sess.ExpiresAt.Equal(start.Add(SessionTTL)) {
		t.Fatalf("should not extend yet: %+v %v", sess, err)
	}

	// 1日たったら延ばす
	now = start.Add(25 * time.Hour)
	sess, err = s.Authenticate(ctx, token)
	if err != nil || !sess.ExpiresAt.Equal(now.Add(SessionTTL)) {
		t.Fatalf("should extend: %+v %v", sess, err)
	}
	stored, _ := store.GetSession(ctx, sess.Hash)
	if !stored.ExpiresAt.Equal(now.Add(SessionTTL)) {
		t.Fatalf("extension not stored: %+v", stored)
	}

	// 期限が過ぎたら使えない
	now = now.Add(SessionTTL)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}

	if _, err := s.Authenticate(ctx, ""); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated for empty token, got %v", err)
	}
	if _, err := s.Authenticate(ctx, "unknown"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated for unknown token, got %v", err)
	}
}

func TestLogoutAndDeleteAccount(t *testing.T) {

	ctx := context.Background()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s, store := newTestService(&now, stubVerifier{sub: "u1"})

	a, _, _ := s.Login(ctx, "id-token")
	b, _, _ := s.Login(ctx, "id-token")

	if err := s.Logout(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, a); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("logged-out session should not be usable")
	}
	if err := s.Logout(ctx, a); err != nil {
		t.Fatalf("logging out twice should succeed: %v", err)
	}

	sess, err := s.Authenticate(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAccount(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, b); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("sessions should be deleted with the account")
	}
	if _, err := store.GetUser(ctx, "u1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("user should be deleted")
	}
	if _, err := s.Me(ctx, sess); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}
