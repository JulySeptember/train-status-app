package auth

import (
	"context"
	"sync"
	"time"

	"train-status-app/backend/internal/favorite"
)

// MemoryStore はメモリ上の Store と favorite.Store。手元での実行（USERS_TABLE が無いとき）とテストで使う
type MemoryStore struct {
	mu        sync.Mutex
	users     map[string]User
	sessions  map[string]Session
	favorites map[string]map[string]favorite.Item // 利用者 ID → 種類と ID → お気に入り
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:     map[string]User{},
		sessions:  map[string]Session{},
		favorites: map[string]map[string]favorite.Item{},
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
