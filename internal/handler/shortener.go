package handler

import (
	"math/rand"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	baseURL  = "http://localhost:8080/"
	idLength = 8
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var (
	urls = make(map[string]string)
	mu   sync.RWMutex
)

func NewRouter() http.Handler {
	router := gin.New()
	router.POST("/", createShortURL)
	router.GET("/:id", getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func badRequest(c *gin.Context) {
	c.String(http.StatusBadRequest, "Bad request")
}

func createShortURL(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		badRequest(c)
		return
	}

	id := generateID()
	mu.Lock()
	urls[id] = string(body)
	mu.Unlock()

	c.Data(http.StatusCreated, "text/plain", []byte(baseURL+id))
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
