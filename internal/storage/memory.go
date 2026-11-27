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
	ErrURLDeleted        = errors.New("URL is deleted")
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
	Deleted     bool
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

func (s *Memory) SaveBatch(_ context.Context, urls []URL) ([]URL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make(map[string]struct{}, len(urls))
	pending := make(map[string]URL, len(urls))
	result := make([]URL, len(urls))
	for i, url := range urls {
		if existingID, exists := s.originalURLs[url.OriginalURL]; exists {
			result[i] = s.urls[existingID]
			continue
		}
		if existingURL, exists := pending[url.OriginalURL]; exists {
			result[i] = existingURL
			continue
		}
		if _, exists := s.urls[url.ID]; exists {
			return nil, ErrIDExists
		}
		if _, exists := ids[url.ID]; exists {
			return nil, ErrIDExists
		}
		ids[url.ID] = struct{}{}
		pending[url.OriginalURL] = url
		result[i] = url
	}

	for _, url := range pending {
		s.urls[url.ID] = url
		s.originalURLs[url.OriginalURL] = url.ID
	}
	return result, nil
}

func (s *Memory) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedURL, ok := s.urls[id]
	if ok && storedURL.Deleted {
		return "", false, ErrURLDeleted
	}
	return storedURL.OriginalURL, ok, nil
}

func (s *Memory) GetByUser(_ context.Context, userID string) ([]URL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	urls := make([]URL, 0)
	for _, storedURL := range s.urls {
		if storedURL.UserID == userID && !storedURL.Deleted {
			urls = append(urls, storedURL)
		}
	}
	return urls, nil
}

func (s *Memory) Delete(_ context.Context, ids []string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		url, ok := s.urls[id]
		if ok && url.UserID == userID {
			url.Deleted = true
			s.urls[id] = url
		}
	}
	return nil
}
