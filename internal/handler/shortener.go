package handler

import (
	"math/rand"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	idLength = 8
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var (
	urls = make(map[string]string)
	mu   sync.RWMutex
)

func NewRouter(baseURL string) http.Handler {
	router := gin.New()
	router.POST("/", func(c *gin.Context) {
		createShortURL(c, baseURL)
	})
	router.GET("/:id", getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func badRequest(c *gin.Context) {
	c.String(http.StatusBadRequest, "Bad request")
}

func createShortURL(c *gin.Context, baseURL string) {
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		badRequest(c)
		return
	}

	id := generateID()
	mu.Lock()
	urls[id] = string(body)
	mu.Unlock()

	shortURL := strings.TrimRight(baseURL, "/") + "/" + id
	c.Data(http.StatusCreated, "text/plain", []byte(shortURL))
}

func getOriginalURL(c *gin.Context) {
	id := c.Param("id")
	mu.RLock()
	originalURL, ok := urls[id]
	mu.RUnlock()
	if !ok {
		badRequest(c)
		return
	}

	c.Header("Location", originalURL)
	c.Status(http.StatusTemporaryRedirect)
}

func generateID() string {
	id := make([]byte, idLength)
	for i := range id {
		id[i] = alphabet[rand.Intn(len(alphabet))]
	}

	return string(id)
}
