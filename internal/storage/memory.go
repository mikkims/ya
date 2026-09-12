package storage

import (
	"context"
	"errors"
	"sync"
)

var ErrIDExists = errors.New("short URL ID already exists")

type URL struct {
	ID          string
	OriginalURL string
}

type Memory struct {
	urls map[string]string
	mu   sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		urls: make(map[string]string),
	}
}

func (s *Memory) Save(_ context.Context, id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	s.urls[id] = originalURL
	return nil
}

func (s *Memory) SaveBatch(_ context.Context, urls []URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[string]struct{}, len(urls))
	for _, url := range urls {
		if _, exists := s.urls[url.ID]; exists {
			return ErrIDExists
		}
		if _, exists := ids[url.ID]; exists {
			return ErrIDExists
		}
		ids[url.ID] = struct{}{}
	}

	for _, url := range urls {
		s.urls[url.ID] = url.OriginalURL
	}
	return nil
}

func (s *Memory) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok, nil
}
