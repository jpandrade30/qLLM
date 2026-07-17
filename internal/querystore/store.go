package querystore

import (
	"sync"
	"time"

	"qLLM/internal/protocol"

	"github.com/google/uuid"
)

type Entry struct {
	Response  *protocol.QueryResponse
	ExpiresAt time.Time
}

type Store struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]*Entry
}

func New(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	s := &Store{ttl: ttl, data: make(map[string]*Entry)}
	go s.reap()
	return s
}

func (s *Store) NewID() string {
	return uuid.NewString()
}

func (s *Store) Put(resp *protocol.QueryResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[resp.QueryID] = &Entry{Response: resp, ExpiresAt: time.Now().Add(s.ttl)}
}

func (s *Store) Get(id string) (*protocol.QueryResponse, *protocol.ProtocolError) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[id]
	if !ok || time.Now().After(e.ExpiresAt) {
		return nil, protocol.NewError(protocol.ErrNotFound, "query not found: "+id, nil)
	}
	return e.Response, nil
}

func (s *Store) Update(resp *protocol.QueryResponse) {
	s.Put(resp)
}

func (s *Store) reap() {
	t := time.NewTicker(30 * time.Second)
	for range t.C {
		now := time.Now()
		s.mu.Lock()
		for k, e := range s.data {
			if now.After(e.ExpiresAt) {
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
	}
}
