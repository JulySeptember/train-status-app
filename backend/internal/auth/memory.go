package auth

import (
	"context"
	"sync"
	"time"
)

// MemoryStore はメモリ上の Store。手元での実行（USERS_TABLE が無いとき）とテストで使う
type MemoryStore struct {
	mu       sync.Mutex
	users    map[string]User
	sessions map[string]Session
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:    map[string]User{},
		sessions: map[string]Session{},
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
	for hash, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessions, hash)
		}
	}
	return nil
}
