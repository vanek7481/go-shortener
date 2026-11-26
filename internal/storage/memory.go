package storage

import (
	"strconv"
	"sync"
)

type Link struct {
	ID          string
	OriginalURL string
}

// Memory protects both indexes with one lock so creation is atomic.
type Memory struct {
	mu    sync.RWMutex
	next  uint64
	urls  map[string]string
	users map[string][]Link
}

func New() *Memory {
	return &Memory{urls: make(map[string]string), users: make(map[string][]Link)}
}

func (m *Memory) Create(userID, original string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Checking the map also prevents collisions if the counter wraps.
	var id string
	for {
		m.next++
		id = strconv.FormatUint(m.next, 36)
		if _, exists := m.urls[id]; !exists {
			break
		}
	}
	m.urls[id] = original
	m.users[userID] = append(m.users[userID], Link{ID: id, OriginalURL: original})
	return id
}

func (m *Memory) Get(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	url, ok := m.urls[id]
	return url, ok
}

func (m *Memory) List(userID string) []Link {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Link(nil), m.users[userID]...)
}
