package handler

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

type URLShortener interface {
	Save(originalURL string) string
	Get(id string) (string, bool)
}

type handler struct {
	baseURL string
	service URLShortener
}

func NewRouter(baseURL string, service URLShortener) http.Handler {
	h := &handler{
		baseURL: baseURL,
		service: service,
	}

	router := gin.New()
	router.POST("/", h.createShortURL)
	router.GET("/:id", h.getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func badRequest(c *gin.Context) {
	c.String(http.StatusBadRequest, "Bad request")
}

func (h *handler) createShortURL(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		badRequest(c)
		return
	}

	id := h.service.Save(string(body))

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		badRequest(c)
		return
	}
	c.Data(http.StatusCreated, "text/plain", []byte(shortURL))
}

func (h *handler) getOriginalURL(c *gin.Context) {
	id := c.Param("id")
	originalURL, ok := h.service.Get(id)
	if !ok {
		badRequest(c)
		return
	}

	c.Header("Location", originalURL)
	c.Status(http.StatusTemporaryRedirect)
}
