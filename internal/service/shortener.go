package service

import (
	"context"
	"errors"
	"math/rand"

	"github.com/mikkims/ya/internal/auth"
	"github.com/mikkims/ya/internal/storage"
)

const (
	idLength        = 8
	alphabet        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxSaveAttempts = 3
)

var (
	ErrSaveAttemptsExceeded = errors.New("failed to save short URL after maximum attempts")
	ErrEmptyBatch           = errors.New("URL batch is empty")
	ErrOriginalURLExists    = errors.New("original URL already exists")
	ErrURLDeleted           = storage.ErrURLDeleted
)

type Shortener struct {
	storage    URLStorage
	generateID func() string
}

type URLStorage interface {
	Save(ctx context.Context, id, originalURL string) error
	SaveBatch(ctx context.Context, urls []storage.URL) ([]storage.URL, error)
	Get(ctx context.Context, id string) (string, bool, error)
}

type userURLStorage interface {
	GetByUser(ctx context.Context, userID string) ([]storage.URL, error)
}

type userURLDeleter interface {
	Delete(ctx context.Context, ids []string, userID string) error
}

func (s *Shortener) SaveBatch(ctx context.Context, originalURLs []string) ([]string, error) {
	if len(originalURLs) == 0 {
		return nil, ErrEmptyBatch
	}

	for range maxSaveAttempts {
		urls := make([]storage.URL, len(originalURLs))
		ids := make([]string, len(originalURLs))
		usedIDs := make(map[string]struct{}, len(originalURLs))
		for i, originalURL := range originalURLs {
			id := s.generateUniqueID(usedIDs)
			usedIDs[id] = struct{}{}
			ids[i] = id
			urls[i] = storage.URL{ID: id, OriginalURL: originalURL, UserID: auth.UserID(ctx)}
		}

		savedURLs, err := s.storage.SaveBatch(ctx, urls)
		if errors.Is(err, storage.ErrIDExists) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for i := range savedURLs {
			ids[i] = savedURLs[i].ID
		}
		return ids, nil
	}

	return nil, ErrSaveAttemptsExceeded
}

func NewShortener(storage URLStorage) *Shortener {
	return &Shortener{
		storage:    storage,
		generateID: generateID,
	}
}

func (s *Shortener) Save(ctx context.Context, originalURL string) (string, error) {
	for range maxSaveAttempts {
		id := s.generateID()
		err := s.storage.Save(ctx, id, originalURL)
		var originalURLExists *storage.OriginalURLExistsError
		if errors.As(err, &originalURLExists) {
			return originalURLExists.ID, ErrOriginalURLExists
		}
		if errors.Is(err, storage.ErrIDExists) {
			continue
		}
		if err != nil {
			return "", err
		}

		return id, nil
	}

	return "", ErrSaveAttemptsExceeded
}

func (s *Shortener) Get(ctx context.Context, id string) (string, bool, error) {
	return s.storage.Get(ctx, id)
}

func (s *Shortener) GetByUser(ctx context.Context, userID string) ([]storage.URL, error) {
	storage, ok := s.storage.(userURLStorage)
	if !ok {
		return nil, errors.New("storage does not support user URLs")
	}
	return storage.GetByUser(ctx, userID)
}

func (s *Shortener) Delete(ids []string, userID string) {
	storage, ok := s.storage.(userURLDeleter)
	if !ok {
		return
	}
	ids = append([]string(nil), ids...)
	go func() {
		_ = storage.Delete(context.Background(), ids, userID)
	}()
}

func generateID() string {
	id := make([]byte, idLength)
	for i := range id {
		id[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(id)
}

func (s *Shortener) generateUniqueID(used map[string]struct{}) string {
	for {
		id := s.generateID()
		if _, exists := used[id]; !exists {
			return id
		}
	}
}
