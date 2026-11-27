package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/mikkims/ya/internal/auth"
)

var (
	// ErrIDExists означает занятый идентификатор короткого URL.
	ErrIDExists = errors.New("short URL ID already exists")
	// ErrOriginalURLExists означает, что исходный URL уже сохранён.
	ErrOriginalURLExists = errors.New("original URL already exists")
	// ErrURLDeleted означает, что URL помечен удалённым.
	ErrURLDeleted = errors.New("URL is deleted")
)

// OriginalURLExistsError содержит идентификатор уже сохранённого исходного URL.
type OriginalURLExistsError struct {
	// ID идентификатор короткого URL.
	ID string
}

// Error возвращает описание ошибки существующего URL.
func (e *OriginalURLExistsError) Error() string {
	return ErrOriginalURLExists.Error()
}

// Unwrap возвращает ErrOriginalURLExists.
func (e *OriginalURLExistsError) Unwrap() error {
	return ErrOriginalURLExists
}

// URL описывает сохранённую запись URL.
type URL struct {
	// ID идентификатор короткого URL.
	ID string
	// OriginalURL исходный URL.
	OriginalURL string
	// UserID идентификатор пользователя.
	UserID string
	// Deleted признак удаления.
	Deleted bool
}

// Memory хранит URL в памяти процесса.
type Memory struct {
	urls         map[string]URL
	originalURLs map[string]string
	mu           sync.Mutex
}

// NewMemory создаёт пустое хранилище в памяти.
func NewMemory() *Memory {
	return &Memory{
		urls:         make(map[string]URL),
		originalURLs: make(map[string]string),
	}
}

// Save сохраняет идентификатор и исходный URL с контекстом или возвращает ошибку.
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

// SaveBatch сохраняет пакет записей и возвращает соответствующие записи или ошибку.
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

// Get ищет URL по идентификатору и возвращает результат, признак наличия и ошибку.
func (s *Memory) Get(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedURL, ok := s.urls[id]
	if ok && storedURL.Deleted {
		return "", false, ErrURLDeleted
	}
	return storedURL.OriginalURL, ok, nil
}

// GetByUser возвращает неудалённые URL пользователя или ошибку.
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

// Delete помечает URL указанного пользователя удалёнными.
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
