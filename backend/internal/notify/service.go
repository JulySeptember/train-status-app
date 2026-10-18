package notify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"train-status-app/backend/internal/notify/webpush"
)

// pushHosts は、購読の endpoint として受け付ける push サービスのホスト（末尾が一致すればよい）。
// endpoint にはサーバーから POST するので、利用者が送った任意の URL に送らないようにする
var pushHosts = []string{
	"fcm.googleapis.com",         // Chrome・Edge（Android を含む）
	".push.services.mozilla.com", // Firefox
	".notify.windows.com",        // 古い Edge
	"web.push.apple.com",         // Safari（macOS・iOS）
	".push.apple.com",            // Safari
}

// Catalog は路線があるかと、その名前を答える（service.Service が満たす）
type Catalog interface {
	HasRailway(id string) bool
	RailwayName(id string) string
}

// Keys は VAPID の鍵を返す。Lambda では初めて使うときに SSM から読む
type Keys interface {
	Keys(ctx context.Context) (*webpush.Keys, error)
}

type Service struct {
	store   Store
	catalog Catalog
	keys    Keys
	sender  func(*webpush.Keys) Sender
	now     func() time.Time
}

// NewService を作る。keys が nil なら、購読と試しの通知は ErrUnavailable になる（条件の登録はできる）
func NewService(store Store, catalog Catalog, keys Keys, subject string) *Service {
	return &Service{
		store:   store,
		catalog: catalog,
		keys:    keys,
		sender:  func(k *webpush.Keys) Sender { return webpush.NewSender(k, subject) },
		now:     time.Now,
	}
}

// PublicKey は、ブラウザの購読に使う VAPID の公開鍵
func (s *Service) PublicKey(ctx context.Context) (string, error) {
	if s.keys == nil {
		return "", ErrUnavailable
	}
	k, err := s.keys.Keys(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return k.Public, nil
}

// Alerts は、利用者の通知の条件を返す。データの更新で無くなった路線は返さない
func (s *Service) Alerts(ctx context.Context, userID string) ([]Alert, error) {
	alerts, err := s.store.ListAlerts(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := []Alert{}
	for _, a := range alerts {
		if s.catalog.HasRailway(a.RailwayID) {
			result = append(result, a)
		}
	}
	slices.SortFunc(result, func(a, b Alert) int { return strings.Compare(a.RailwayID, b.RailwayID) })
	return result, nil
}

func validateAlert(a Alert) error {
	if len(a.Days) == 0 || len(a.Days) > 2 {
		return fmt.Errorf("%w: days", ErrInvalid)
	}
	for i, d := range a.Days {
		if (d != Weekday && d != Holiday) || slices.Contains(a.Days[:i], d) {
			return fmt.Errorf("%w: day %q", ErrInvalid, d)
		}
	}
	start, err := parseClock(a.Start)
	if err != nil {
		return err
	}
	end, err := parseClock(a.End)
	if err != nil {
		return err
	}
	if start >= end {
		return fmt.Errorf("%w: start must be before end", ErrInvalid)
	}
	return nil
}

// SetAlert は、路線の通知の条件を登録する（すでにあれば置き換える）
func (s *Service) SetAlert(ctx context.Context, userID string, alert Alert) error {

	if !s.catalog.HasRailway(alert.RailwayID) {
		return fmt.Errorf("%w: unknown railway %s", ErrInvalid, alert.RailwayID)
	}
	if err := validateAlert(alert); err != nil {
		return err
	}

	alerts, err := s.store.ListAlerts(ctx, userID)
	if err != nil {
		return err
	}
	count := 0
	for _, a := range alerts {
		if a.RailwayID == alert.RailwayID {
			return s.store.PutAlert(ctx, userID, alert)
		}
		if s.catalog.HasRailway(a.RailwayID) {
			count++
		}
	}
	if count >= MaxAlerts {
		return fmt.Errorf("%w: alerts", ErrLimit)
	}

	return s.store.PutAlert(ctx, userID, alert)
}

func (s *Service) RemoveAlert(ctx context.Context, userID, railwayID string) error {
	if railwayID == "" {
		return fmt.Errorf("%w: railway", ErrInvalid)
	}
	return s.store.DeleteAlert(ctx, userID, railwayID)
}

// validateSubscription は、endpoint が既知の push サービスの HTTPS の URL で、鍵の長さが正しいかを確かめる
func validateSubscription(sub webpush.Subscription) error {

	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(sub.Endpoint) > 1024 {
		return fmt.Errorf("%w: endpoint", ErrInvalid)
	}
	host := strings.ToLower(u.Hostname())
	if !slices.ContainsFunc(pushHosts, func(h string) bool {
		if strings.HasPrefix(h, ".") {
			return strings.HasSuffix(host, h)
		}
		return host == h
	}) {
		return fmt.Errorf("%w: unsupported push service %s", ErrInvalid, host)
	}

	p256dh, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.P256dh, "="))
	if err != nil || len(p256dh) != 65 {
		return fmt.Errorf("%w: p256dh", ErrInvalid)
	}
	auth, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	if err != nil || len(auth) != 16 {
		return fmt.Errorf("%w: auth", ErrInvalid)
	}
	return nil
}

// Subscribe は、端末の購読を登録する。上限を超えたら、いちばん古い端末を外す
func (s *Service) Subscribe(ctx context.Context, userID string, sub webpush.Subscription) error {

	if s.keys == nil {
		return ErrUnavailable
	}

	// ブラウザによっては base64url にパディングが付くので、外してから保存する
	sub.Keys.P256dh = strings.TrimRight(sub.Keys.P256dh, "=")
	sub.Keys.Auth = strings.TrimRight(sub.Keys.Auth, "=")

	if err := validateSubscription(sub); err != nil {
		return err
	}

	subs, err := s.store.ListSubscriptions(ctx, userID)
	if err != nil {
		return err
	}
	others := slices.DeleteFunc(subs, func(o Subscription) bool { return o.Endpoint == sub.Endpoint })
	slices.SortFunc(others, func(a, b Subscription) int { return a.CreatedAt.Compare(b.CreatedAt) })
	for len(others) >= MaxSubscriptions {
		if err := s.store.DeleteSubscription(ctx, userID, others[0].Endpoint); err != nil {
			return err
		}
		others = others[1:]
	}

	return s.store.PutSubscription(ctx, userID, Subscription{Subscription: sub, CreatedAt: s.now()})
}

func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("%w: endpoint", ErrInvalid)
	}
	return s.store.DeleteSubscription(ctx, userID, endpoint)
}

// SendTest は、利用者のすべての端末に試しの通知を送る。送れた端末の数を返す
func (s *Service) SendTest(ctx context.Context, userID string) (int, error) {

	if s.keys == nil {
		return 0, ErrUnavailable
	}
	keys, err := s.keys.Keys(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	subs, err := s.store.ListSubscriptions(ctx, userID)
	if err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, ErrNoSubscription
	}

	ok, err := s.store.TakeTestSlot(ctx, userID, s.now(), TestInterval)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrTooSoon
	}

	payload, err := json.Marshal(Message{
		Title: "通知のテスト",
		Body:  "この端末で運行情報の通知を受け取れます。",
		URL:   "/account",
		Tag:   "test",
	})
	if err != nil {
		return 0, err
	}

	sent := deliver(ctx, s.store, s.sender(keys), userID, subs, payload, time.Minute)
	if sent == 0 {
		return 0, ErrNoSubscription
	}
	return sent, nil
}

// deliver は、利用者の端末に並列に送り、送れた数を返す。無くなった購読（ErrGone）は消す
func deliver(ctx context.Context, store Store, sender Sender, userID string, subs []Subscription, payload []byte, ttl time.Duration) int {

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		sent int
	)
	for _, sub := range subs {
		wg.Go(func() {
			err := sender.Send(ctx, sub.Subscription, payload, ttl)
			switch {
			case errors.Is(err, webpush.ErrGone):
				if err := store.DeleteSubscription(ctx, userID, sub.Endpoint); err != nil {
					log.Printf("notify: delete gone subscription: %v", err)
				}
			case err != nil:
				// endpoint は利用者を特定できるので出さない（webpush のエラーはホストだけ）
				log.Printf("notify: send: %v", err)
			default:
				mu.Lock()
				sent++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return sent
}

// KeyLoader は、VAPID の鍵を初めて使うときに読み、以後は使い回す。読めなければ、少し待ってから読み直す
type KeyLoader struct {
	load func(ctx context.Context) (string, error)

	mu      sync.Mutex
	keys    *webpush.Keys
	retryAt time.Time
	lastErr error
}

func NewKeyLoader(load func(ctx context.Context) (string, error)) *KeyLoader {
	return &KeyLoader{load: load}
}

func (l *KeyLoader) Keys(ctx context.Context) (*webpush.Keys, error) {

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.keys != nil {
		return l.keys, nil
	}
	if time.Now().Before(l.retryAt) {
		return nil, l.lastErr
	}

	private, err := l.load(ctx)
	if err == nil {
		l.keys, err = webpush.ParseKeys(private)
	}
	if err != nil {
		l.retryAt = time.Now().Add(time.Minute)
		l.lastErr = err
		return nil, err
	}
	return l.keys, nil
}
