package service

import (
	"errors"
	"math/rand"

	"github.com/mikkims/ya/internal/storage"
)

const (
	idLength        = 8
	alphabet        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	maxSaveAttempts = 3
)

var ErrSaveAttemptsExceeded = errors.New("failed to save short URL after maximum attempts")

type Shortener struct {
	storage URLStorage
}

type URLStorage interface {
	Save(id, originalURL string) error
	Get(id string) (string, bool)
}

func NewShortener(storage URLStorage) *Shortener {
	return &Shortener{
		storage: storage,
	}
}

func (s *Shortener) Save(originalURL string) (string, error) {
	for range maxSaveAttempts {
		id := generateID()
		err := s.storage.Save(id, originalURL)
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

func (s *Shortener) Get(id string) (string, bool) {
	return s.storage.Get(id)
}

func generateID() string {
	id := make([]byte, idLength)
	for i := range id {
		id[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(id)
}
