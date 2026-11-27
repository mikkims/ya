package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikkims/ya/internal/audit"
	appauth "github.com/mikkims/ya/internal/auth"
	"github.com/mikkims/ya/internal/model/dto"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
)

// URLShortener определяет операции сокращения URL для обработчиков.
type URLShortener interface {
	// Save сохраняет исходный URL и возвращает короткий идентификатор или ошибку.
	Save(ctx context.Context, originalURL string) (string, error)
	// SaveBatch возвращает короткие идентификаторы в порядке исходных URL или ошибку.
	SaveBatch(ctx context.Context, originalURLs []string) ([]string, error)
	// Get ищет URL по идентификатору и возвращает результат, признак наличия и ошибку.
	Get(ctx context.Context, id string) (string, bool, error)
}

type userURLProvider interface {
	GetByUser(ctx context.Context, userID string) ([]storage.URL, error)
}

type userURLDeleter interface {
	Delete(ctx context.Context, ids []string, userID string) error
}

// Pinger проверяет доступность базы с контекстом.
type Pinger interface {
	// PingContext проверяет доступность с указанным контекстом.
	PingContext(ctx context.Context) error
}

type handler struct {
	audit    *audit.Publisher
	baseURL  string
	service  URLShortener
	database Pinger
}

// NewRouter создаёт маршрутизатор сокращения URL без проверки подключения к БД и аудита.
// Переданный service может использовать любое хранилище.
//
// Маршруты:
//   - POST / принимает исходный URL текстом и возвращает короткий URL (201 или 409).
//   - POST /api/shorten принимает JSON с полем url и возвращает result (201 или 409).
//   - POST /api/shorten/batch сохраняет пакет с correlation_id и original_url (201).
//   - GET /{id} возвращает 307 и Location; неизвестный ID даёт 400, удалённый — 410.
//   - GET /api/user/urls возвращает ссылки пользователя из cookie user_id (200 или 204).
//   - DELETE /api/user/urls ставит массив ID в очередь удаления (202).
//   - GET /ping проверяет Pinger; без переданного Pinger возвращает 500.
//
// Для проверки БД и аудита используются NewRouterWithDatabase и NewRouterWithAudit.
func NewRouter(baseURL string, service URLShortener) http.Handler {
	return NewRouterWithDatabase(baseURL, service, nil)
}

// NewRouterWithDatabase создаёт маршрутизатор с Pinger для GET /ping без аудита.
// При nil Pinger или ошибке проверки GET /ping возвращает 500, иначе — 200.
func NewRouterWithDatabase(baseURL string, service URLShortener, database Pinger) http.Handler {
	return NewRouterWithAudit(baseURL, service, database, nil)
}

// NewRouterWithAudit создаёт маршрутизатор с необязательными Pinger и издателем аудита.
// nil publisher отключает аудит; nil database приводит к ответу 500 на GET /ping.
// События аудита публикуются после создания URL с кодом 201 и перехода с кодом 307.
func NewRouterWithAudit(baseURL string, service URLShortener, database Pinger, publisher *audit.Publisher) http.Handler {
	h := &handler{
		audit:    publisher,
		baseURL:  baseURL,
		service:  service,
		database: database,
	}

	router := gin.New()
	router.Use(h.authenticate)
	router.POST("/", h.createShortURL)
	router.POST("/api/shorten", h.createShortURLJSON)
	router.POST("/api/shorten/batch", h.createShortURLBatch)
	router.GET("/api/user/urls", h.getUserURLs)
	router.DELETE("/api/user/urls", h.deleteUserURLs)
	router.GET("/ping", h.pingDatabase)
	router.GET("/:id", h.getOriginalURL)
	router.NoRoute(badRequest)

	return router
}

func (h *handler) createShortURLBatch(c *gin.Context) {
	var request []dto.BatchShortenRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request) == 0 {
		badRequest(c)
		return
	}

	originalURLs := make([]string, len(request))
	for i, item := range request {
		if item.OriginalURL == "" {
			badRequest(c)
			return
		}
		originalURLs[i] = item.OriginalURL
	}

	ids, err := h.service.SaveBatch(c.Request.Context(), originalURLs)
	if err != nil {
		internalServerError(c)
		return
	}

	response := make([]dto.BatchShortenResponse, len(request))
	for i, item := range request {
		shortURL, err := url.JoinPath(h.baseURL, ids[i])
		if err != nil {
			internalServerError(c)
			return
		}
		response[i] = dto.BatchShortenResponse{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortURL,
		}
	}

	c.JSON(http.StatusCreated, response)
}

func (h *handler) deleteUserURLs(c *gin.Context) {
	var ids []string
	if err := c.ShouldBindJSON(&ids); err != nil || len(ids) == 0 {
		badRequest(c)
		return
	}
	deleter, ok := h.service.(userURLDeleter)
	if !ok {
		internalServerError(c)
		return
	}
	if err := deleter.Delete(c.Request.Context(), ids, appauth.UserID(c.Request.Context())); err != nil {
		internalServerError(c)
		return
	}
	c.Status(http.StatusAccepted)
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

	id, err := h.service.Save(c.Request.Context(), request.URL)
	status := http.StatusCreated
	if errors.Is(err, service.ErrOriginalURLExists) {
		status = http.StatusConflict
	} else if err != nil {
		internalServerError(c)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		badRequest(c)
		return
	}

	c.JSON(status, dto.ShortenResponse{Result: shortURL})
	if status == http.StatusCreated {
		h.publish(c, "shorten", request.URL)
	}
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

	id, err := h.service.Save(c.Request.Context(), string(body))
	status := http.StatusCreated
	if errors.Is(err, service.ErrOriginalURLExists) {
		status = http.StatusConflict
	} else if err != nil {
		internalServerError(c)
		return
	}

	shortURL, err := url.JoinPath(h.baseURL, id)
	if err != nil {
		badRequest(c)
		return
	}
	c.Data(status, "text/plain", []byte(shortURL))
	if status == http.StatusCreated {
		h.publish(c, "shorten", string(body))
	}
}

func (h *handler) getOriginalURL(c *gin.Context) {
	id := c.Param("id")
	originalURL, ok, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrURLDeleted) {
			c.Status(http.StatusGone)
			return
		}
		internalServerError(c)
		return
	}
	if !ok {
		badRequest(c)
		return
	}

	c.Header("Location", originalURL)
	c.Status(http.StatusTemporaryRedirect)
	h.publish(c, "follow", originalURL)
}

func (h *handler) publish(c *gin.Context, action, originalURL string) {
	h.audit.Publish(c.Request.Context(), audit.Event{Timestamp: time.Now().Unix(), Action: action, UserID: appauth.UserID(c.Request.Context()), URL: originalURL})
}

func (h *handler) getUserURLs(c *gin.Context) {
	ownerID := appauth.UserID(c.Request.Context())
	if ownerID == "" {
		c.Status(http.StatusUnauthorized)
		return
	}

	provider, ok := h.service.(userURLProvider)
	if !ok {
		internalServerError(c)
		return
	}
	urls, err := provider.GetByUser(c.Request.Context(), ownerID)
	if err != nil {
		internalServerError(c)
		return
	}
	if len(urls) == 0 {
		c.Status(http.StatusNoContent)
		return
	}

	response := make([]dto.UserURL, len(urls))
	for i, storedURL := range urls {
		shortURL, err := url.JoinPath(h.baseURL, storedURL.ID)
		if err != nil {
			internalServerError(c)
			return
		}
		response[i] = dto.UserURL{ShortURL: shortURL, OriginalURL: storedURL.OriginalURL}
	}
	c.JSON(http.StatusOK, response)
}
