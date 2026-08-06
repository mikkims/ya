package service

import (
	"math/rand"
)

const (
	idLength = 8
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type Shortener struct {
	storage URLStorage
}

type URLStorage interface {
	Save(id, originalURL string) bool
	Get(id string) (string, bool)
}

func NewShortener(storage URLStorage) *Shortener {
	return &Shortener{
		storage: storage,
	}
}

func (s *Shortener) Save(originalURL string) string {
	for {
		id := generateID()
		if !s.storage.Save(id, originalURL) {
			continue
		}

		return id
	}
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
