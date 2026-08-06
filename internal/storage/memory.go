package storage

import "sync"

type Memory struct {
	urls map[string]string
	mu   sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		urls: make(map[string]string),
	}
}

func (s *Memory) Save(id, originalURL string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return false
	}

	s.urls[id] = originalURL
	return true
}

func (s *Memory) Get(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok
}
