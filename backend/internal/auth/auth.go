package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	// セッションの有効期限
	SessionTTL = 30 * 24 * time.Hour

	// 残りがこれより短くなったら期限を延ばす（書き込みを1日に1回までにする）
	sessionExtendAfter = 24 * time.Hour
)

var (
	// ErrInvalidToken は、Google の ID トークンが正しくないこと
	ErrInvalidToken = errors.New("auth: invalid id token")

	// ErrUnauthenticated は、セッションが無いか期限が切れていること
	ErrUnauthenticated = errors.New("auth: unauthenticated")

	// ErrNotFound は、Store に項目が無いこと
	ErrNotFound = errors.New("auth: not found")
)

// User は利用者。ID は Google の利用者 ID（sub）で、レスポンスには出さない。
// メールアドレス・名前は保存しない（docs/design/login.md 2章）
type User struct {
	ID        string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// Session はログインの状態。Hash はセッション ID の SHA-256 で、セッション ID そのものは保存しない
type Session struct {
	Hash      string
	UserID    string
	ExpiresAt time.Time
}

// Store は利用者とセッションの保存先。DynamoDB（dynamostore）とメモリ上の実装（MemoryStore）がある。
type Store interface {
	// EnsureUser は利用者がいなければ作り、利用者を返す
	EnsureUser(ctx context.Context, userID string, now time.Time) (User, error)

	// GetUser は利用者を返す。いなければ ErrNotFound
	GetUser(ctx context.Context, userID string) (User, error)

	CreateSession(ctx context.Context, s Session) error

	// GetSession はセッションを返す。無ければ ErrNotFound（期限の確認は Service が行う）
	GetSession(ctx context.Context, hash string) (Session, error)

	// ExtendSession はセッションの期限を s.ExpiresAt にする。もう消えていれば ErrNotFound
	ExtendSession(ctx context.Context, s Session) error

	DeleteSession(ctx context.Context, s Session) error

	// DeleteUser は利用者のデータとセッションをすべて消す
	DeleteUser(ctx context.Context, userID string) error
}

type Service struct {
	verifier Verifier
	store    Store
	now      func() time.Time
}

func NewService(verifier Verifier, store Store) *Service {
	return &Service{verifier: verifier, store: store, now: time.Now}
}

// Login は Google の ID トークンを確かめ、利用者がいなければ作って、新しいセッション ID を返す。
func (s *Service) Login(ctx context.Context, idToken string) (string, User, error) {

	userID, err := s.verifier.Verify(ctx, idToken)
	if err != nil {
		return "", User{}, err
	}

	now := s.now()

	user, err := s.store.EnsureUser(ctx, userID, now)
	if err != nil {
		return "", User{}, fmt.Errorf("ensure user: %w", err)
	}

	token, err := newSessionToken()
	if err != nil {
		return "", User{}, err
	}

	if err := s.store.CreateSession(ctx, Session{
		Hash:      hashToken(token),
		UserID:    userID,
		ExpiresAt: now.Add(SessionTTL),
	}); err != nil {
		return "", User{}, fmt.Errorf("create session: %w", err)
	}

	return token, user, nil
}

// Authenticate はセッション ID からセッションを引く。期限が近ければ延ばす。
// 無いか期限が切れていれば ErrUnauthenticated
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {

	if token == "" {
		return Session{}, ErrUnauthenticated
	}

	sess, err := s.store.GetSession(ctx, hashToken(token))
	if errors.Is(err, ErrNotFound) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	// DynamoDB の TTL はすぐには消さないので、期限は自分で確かめる
	now := s.now()
	if !now.Before(sess.ExpiresAt) {
		return Session{}, ErrUnauthenticated
	}

	if sess.ExpiresAt.Sub(now) < SessionTTL-sessionExtendAfter {
		sess.ExpiresAt = now.Add(SessionTTL)
		err := s.store.ExtendSession(ctx, sess)
		if errors.Is(err, ErrNotFound) {
			return Session{}, ErrUnauthenticated
		}
		if err != nil {
			return Session{}, fmt.Errorf("extend session: %w", err)
		}
	}

	return sess, nil
}

// Me はログイン中の利用者を返す。退会などで利用者がいなければ ErrUnauthenticated
func (s *Service) Me(ctx context.Context, sess Session) (User, error) {
	user, err := s.store.GetUser(ctx, sess.UserID)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrUnauthenticated
	}
	return user, err
}

// Logout はセッションを消す。もう無ければ何もしない
func (s *Service) Logout(ctx context.Context, token string) error {

	if token == "" {
		return nil
	}

	sess, err := s.store.GetSession(ctx, hashToken(token))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	return s.store.DeleteSession(ctx, sess)
}

// DeleteAccount は退会する。利用者のデータとセッションをすべて消す
func (s *Service) DeleteAccount(ctx context.Context, sess Session) error {
	return s.store.DeleteUser(ctx, sess.UserID)
}

// newSessionToken は 32 バイトの乱数のセッション ID を作る
func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
