package handler

import (
	"math/rand"
	"net/http"
	"net/url"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	idLength = 8
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

type shortener struct {
	baseURL string
	urls    map[string]string
	mu      sync.Mutex
}

func NewRouter(baseURL string) http.Handler {
	service := &shortener{
		baseURL: baseURL,
		urls:    make(map[string]string),
	}

	router := gin.New()
	router.POST("/", service.createShortURL)
	router.GET("/:id", service.getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func badRequest(c *gin.Context) {
	c.String(http.StatusBadRequest, "Bad request")
}

func (s *shortener) createShortURL(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		badRequest(c)
		return
	}

	id := s.save(string(body))

	shortURL, err := url.JoinPath(s.baseURL, id)
	if err != nil {
		badRequest(c)
		return
	}
	c.Data(http.StatusCreated, "text/plain", []byte(shortURL))
}

func (s *shortener) getOriginalURL(c *gin.Context) {
	id := c.Param("id")
	originalURL, ok := s.get(id)
	if !ok {
		badRequest(c)
		return
	}

	c.Header("Location", originalURL)
	c.Status(http.StatusTemporaryRedirect)
}

func (s *shortener) save(originalURL string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for {
		id := generateID()
		if _, exists := s.urls[id]; exists {
			continue
		}

		s.urls[id] = originalURL
		return id
	}
}

func (s *shortener) get(id string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, ok := s.urls[id]
	return originalURL, ok
}

func generateID() string {
	id := make([]byte, idLength)
	for i := range id {
		id[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(id)
}
