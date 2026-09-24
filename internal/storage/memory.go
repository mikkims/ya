package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/mikkims/ya/internal/auth"
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
	UserID      string
}

type Memory struct {
	urls         map[string]URL
	originalURLs map[string]string
	mu           sync.Mutex
}

func NewMemory() *Memory {
	return &Memory{
		urls:         make(map[string]URL),
		originalURLs: make(map[string]string),
	}
}

func (s *Memory) Save(ctx context.Context, id, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, exists := s.originalURLs[originalURL]; exists {
		return &OriginalURLExistsError{ID: existingID}
	}
	if _, exists := s.urls[id]; exists {
		return ErrIDExists
	}

	s.urls[id] = URL{ID: id, OriginalURL: originalURL, UserID: auth.UserID(ctx)}
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
		s.urls[url.ID] = url
		s.originalURLs[url.OriginalURL] = url.ID
	}
	return nil
}

func (s *Memory) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedURL, ok := s.urls[id]
	return storedURL.OriginalURL, ok, nil
}

func (s *Memory) GetByUser(_ context.Context, userID string) ([]URL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	urls := make([]URL, 0)
	for _, storedURL := range s.urls {
		if storedURL.UserID == userID {
			urls = append(urls, storedURL)
		}
	}
	return urls, nil
}
