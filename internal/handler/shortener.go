package handler

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/mikkims/ya/internal/model/dto"
)

type URLShortener interface {
	Save(originalURL string) (string, error)
	Get(id string) (string, bool)
}

type Pinger interface {
	PingContext(ctx context.Context) error
}

type handler struct {
	baseURL  string
	service  URLShortener
	database Pinger
}

func NewRouter(baseURL string, service URLShortener) http.Handler {
	return NewRouterWithDatabase(baseURL, service, nil)
}

func NewRouterWithDatabase(baseURL string, service URLShortener, database Pinger) http.Handler {
	h := &handler{
		baseURL:  baseURL,
		service:  service,
		database: database,
	}

	router := gin.New()
	router.POST("/", h.createShortURL)
	router.POST("/api/shorten", h.createShortURLJSON)
	router.GET("/ping", h.pingDatabase)
	router.GET("/:id", h.getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func (h *handler) pingDatabase(c *gin.Context) {
	if h.database == nil || h.database.PingContext(c.Request.Context()) != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func (h *handler) createShortURLJSON(c *gin.Context) {
	var request dto.ShortenRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.URL == "" {
		badRequest(c)
		return
	}

	id, err := h.service.Save(request.URL)
	if err != nil {
		internalServerError(c)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		badRequest(c)
		return
	}

	c.JSON(http.StatusCreated, dto.ShortenResponse{Result: shortURL})
}

func badRequest(c *gin.Context) {
	c.String(http.StatusBadRequest, "Bad request")
}

func internalServerError(c *gin.Context) {
	c.String(http.StatusInternalServerError, "Internal server error")
}

func (h *handler) createShortURL(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil || len(body) == 0 {
		badRequest(c)
		return
	}

	id, err := h.service.Save(string(body))
	if err != nil {
		internalServerError(c)
		return
	}

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
