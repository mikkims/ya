package storage

import (
	"errors"
	"sync"
)

var ErrIDExists = errors.New("short URL ID already exists")

type Memory struct {
	urls map[string]string
	mu   sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		urls: make(map[string]string),
	}
}

func (s *Memory) Save(id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	s.urls[id] = originalURL
	return nil
}

func (s *Memory) Get(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok
}
