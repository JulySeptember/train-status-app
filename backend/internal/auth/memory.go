package auth

import (
	"context"
	"sync"
	"time"

	"train-status-app/backend/internal/favorite"
	"train-status-app/backend/internal/notify"
)

// MemoryStore はメモリ上の Store と favorite.Store・notify.Store。手元での実行（USERS_TABLE が無いとき）とテストで使う
type MemoryStore struct {
	mu            sync.Mutex
	users         map[string]User
	sessions      map[string]Session
	favorites     map[string]map[string]favorite.Item       // 利用者 ID → 種類と ID → お気に入り
	alerts        map[string]map[string]notify.Alert        // 利用者 ID → 路線 ID → 通知の条件
	subscriptions map[string]map[string]notify.Subscription // 利用者 ID → endpoint → 購読
	tested        map[string]time.Time                      // 利用者 ID → 試しの通知を送った時刻
	states        map[string]notify.Level
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:         map[string]User{},
		sessions:      map[string]Session{},
		favorites:     map[string]map[string]favorite.Item{},
		alerts:        map[string]map[string]notify.Alert{},
		subscriptions: map[string]map[string]notify.Subscription{},
		tested:        map[string]time.Time{},
		states:        map[string]notify.Level{},
	}
}

func (m *MemoryStore) EnsureUser(_ context.Context, userID string, now time.Time) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u, ok := m.users[userID]; ok {
		return u, nil
	}
	u := User{ID: userID, CreatedAt: now}
	m.users[userID] = u
	return u, nil
}

func (m *MemoryStore) GetUser(_ context.Context, userID string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[userID]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[s.UserID]; !ok {
		return ErrNotFound
	}
	m.sessions[s.Hash] = s
	return nil
}

func (m *MemoryStore) GetSession(_ context.Context, hash string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[hash]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (m *MemoryStore) ExtendSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[s.Hash]; !ok {
		return ErrNotFound
	}
	m.sessions[s.Hash] = s
	return nil
}

func (m *MemoryStore) DeleteSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, s.Hash)
	return nil
}

func (m *MemoryStore) DeleteUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.users, userID)
	delete(m.favorites, userID)
	delete(m.alerts, userID)
	delete(m.subscriptions, userID)
	delete(m.tested, userID)
	for hash, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, hash)
		}
	}
	return nil
}

func (m *MemoryStore) ListFavorites(_ context.Context, userID string) ([]favorite.Item, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var items []favorite.Item
	for _, item := range m.favorites[userID] {
		items = append(items, item)
	}
	return items, nil
}

func (m *MemoryStore) PutFavorite(_ context.Context, userID string, item favorite.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[userID]; !ok {
		return favorite.ErrUserNotFound
	}
	if m.favorites[userID] == nil {
		m.favorites[userID] = map[string]favorite.Item{}
	}
	key := string(item.Kind) + "#" + item.ID
	if _, ok := m.favorites[userID][key]; !ok {
		m.favorites[userID][key] = item
	}
	return nil
}

func (m *MemoryStore) DeleteFavorite(_ context.Context, userID string, kind favorite.Kind, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.favorites[userID], string(kind)+"#"+id)
	return nil
}

func (m *MemoryStore) ListAlerts(_ context.Context, userID string) ([]notify.Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var alerts []notify.Alert
	for _, a := range m.alerts[userID] {
		alerts = append(alerts, a)
	}
	return alerts, nil
}

func (m *MemoryStore) PutAlert(_ context.Context, userID string, alert notify.Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[userID]; !ok {
		return notify.ErrUserNotFound
	}
	if m.alerts[userID] == nil {
		m.alerts[userID] = map[string]notify.Alert{}
	}
	m.alerts[userID][alert.RailwayID] = alert
	return nil
}

func (m *MemoryStore) DeleteAlert(_ context.Context, userID, railwayID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.alerts[userID], railwayID)
	return nil
}

func (m *MemoryStore) AlertsForRailway(_ context.Context, railwayID string) ([]notify.UserAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []notify.UserAlert
	for userID, alerts := range m.alerts {
		if a, ok := alerts[railwayID]; ok {
			result = append(result, notify.UserAlert{UserID: userID, Alert: a})
		}
	}
	return result, nil
}

func (m *MemoryStore) ListSubscriptions(_ context.Context, userID string) ([]notify.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var subs []notify.Subscription
	for _, s := range m.subscriptions[userID] {
		subs = append(subs, s)
	}
	return subs, nil
}

func (m *MemoryStore) PutSubscription(_ context.Context, userID string, sub notify.Subscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.users[userID]; !ok {
		return notify.ErrUserNotFound
	}
	if m.subscriptions[userID] == nil {
		m.subscriptions[userID] = map[string]notify.Subscription{}
	}
	m.subscriptions[userID][sub.Endpoint] = sub
	return nil
}

func (m *MemoryStore) DeleteSubscription(_ context.Context, userID, endpoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.subscriptions[userID], endpoint)
	return nil
}

func (m *MemoryStore) TakeTestSlot(_ context.Context, userID string, now time.Time, interval time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if last, ok := m.tested[userID]; ok && now.Sub(last) < interval {
		return false, nil
	}
	m.tested[userID] = now
	return true, nil
}

func (m *MemoryStore) LoadStates(context.Context) (map[string]notify.Level, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]notify.Level, len(m.states))
	for k, v := range m.states {
		result[k] = v
	}
	return result, nil
}

func (m *MemoryStore) SaveStates(_ context.Context, states map[string]notify.Level) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.states = make(map[string]notify.Level, len(states))
	for k, v := range states {
		m.states[k] = v
	}
	return nil
}
