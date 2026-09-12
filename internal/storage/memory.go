package storage

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrIDExists          = errors.New("short URL ID already exists")
	ErrOriginalURLExists = errors.New("original URL already exists")
)

type OriginalURLExistsError struct {
	ID string
}

func (e *OriginalURLExistsError) Error() string {
	return ErrOriginalURLExists.Error()
}

func (e *OriginalURLExistsError) Unwrap() error {
	return ErrOriginalURLExists
}

type URL struct {
	ID          string
	OriginalURL string
}

type Memory struct {
	urls         map[string]string
	originalURLs map[string]string
	mu           sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		urls:         make(map[string]string),
		originalURLs: make(map[string]string),
	}
}

func (s *Memory) Save(_ context.Context, id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, exists := s.originalURLs[originalURL]; exists {
		return &OriginalURLExistsError{ID: existingID}
	}
	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	s.urls[id] = originalURL
	s.originalURLs[originalURL] = id
	return nil
}

func (s *Memory) SaveBatch(_ context.Context, urls []URL) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[string]struct{}, len(urls))
	originalURLs := make(map[string]string, len(urls))
	for _, url := range urls {
		if existingID, exists := s.originalURLs[url.OriginalURL]; exists {
			return &OriginalURLExistsError{ID: existingID}
		}
		if existingID, exists := originalURLs[url.OriginalURL]; exists {
			return &OriginalURLExistsError{ID: existingID}
		}
		if _, exists := s.urls[url.ID]; exists {
			return ErrIDExists
		}
		if _, exists := ids[url.ID]; exists {
			return ErrIDExists
		}
		ids[url.ID] = struct{}{}
		originalURLs[url.OriginalURL] = url.ID
	}

	for _, url := range urls {
		s.urls[url.ID] = url.OriginalURL
		s.originalURLs[url.OriginalURL] = url.ID
	}
	return nil
}

func (s *Memory) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok, nil
}
