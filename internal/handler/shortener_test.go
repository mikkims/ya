package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	appgzip "github.com/mikkims/ya/internal/gzip"
	"github.com/mikkims/ya/internal/model/dto"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/rs/zerolog"
)

type pingerStub struct {
	err    error
	called bool
}

type shortenerStub struct {
	batchIDs  []string
	batchErr  error
	batchURLs []string
	getErr    error
}

func (s *shortenerStub) SaveBatch(_ context.Context, originalURLs []string) ([]string, error) {
	s.batchURLs = append([]string(nil), originalURLs...)
	return s.batchIDs, s.batchErr
}

func (s *shortenerStub) Save(_ context.Context, _ string) (string, error) {
	return "unused", nil
}

func (s *shortenerStub) Get(_ context.Context, _ string) (string, bool, error) {
	return "", false, s.getErr
}

func (p *pingerStub) PingContext(ctx context.Context) error {
	p.called = true
	if ctx == nil {
		return errors.New("nil context")
	}
	return p.err
}

func TestPingDatabase(t *testing.T) {
	tests := []struct {
		name       string
		pinger     *pingerStub
		wantStatus int
		wantCall   bool
	}{
		{name: "success", pinger: &pingerStub{}, wantStatus: http.StatusOK, wantCall: true},
		{name: "database error", pinger: &pingerStub{err: errors.New("database unavailable")}, wantStatus: http.StatusInternalServerError, wantCall: true},
		{name: "database is not configured", wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))
			if tt.pinger != nil {
				router = NewRouterWithDatabase("http://localhost:8080", service.NewShortener(storage.NewMemory()), tt.pinger)
			}
			request := httptest.NewRequest(http.MethodGet, "/ping", nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Body.Len() != 0 {
				t.Errorf("response body = %q, want empty", response.Body.String())
			}
			if tt.pinger != nil && tt.pinger.called != tt.wantCall {
				t.Errorf("PingContext called = %v, want %v", tt.pinger.called, tt.wantCall)
			}
		})
	}
}

func TestCreateShortURL(t *testing.T) {
	const baseURL = "http://localhost:8080/"
	router := NewRouter(baseURL, service.NewShortener(storage.NewMemory()))

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid URL",
			body:       "https://practicum.yandex.ru/",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			result := response.Result()
			defer func() { _ = result.Body.Close() }()

			if result.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.wantStatus)
				return
			}

			if tt.wantStatus == http.StatusCreated {
				if contentType := result.Header.Get("Content-Type"); contentType != "text/plain" {
					t.Errorf("Content-Type = %q, want %q", contentType, "text/plain")
					return
				}

				shortURL := response.Body.String()
				if !strings.HasPrefix(shortURL, baseURL) {
					t.Errorf("short URL = %q, want prefix %q", shortURL, baseURL)
					return
				}
				const expectedIDLength = 8
				if len(strings.TrimPrefix(shortURL, baseURL)) != expectedIDLength {
					t.Errorf("short URL ID must contain %d characters", expectedIDLength)
					return
				}
			}
		})
	}
}

func TestCreateShortURLJSON(t *testing.T) {
	const baseURL = "http://localhost:8080/"
	router := NewRouter(baseURL, service.NewShortener(storage.NewMemory()))

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid URL",
			body:       `{"url":"https://practicum.yandex.ru"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       `{"url":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty URL",
			body:       `{"url":""}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusCreated {
				return
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", contentType)
			}

			var result dto.ShortenResponse
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !strings.HasPrefix(result.Result, baseURL) {
				t.Errorf("result = %q, want prefix %q", result.Result, baseURL)
			}
			const expectedIDLength = 8
			if len(strings.TrimPrefix(result.Result, baseURL)) != expectedIDLength {
				t.Errorf("short URL ID must contain %d characters", expectedIDLength)
			}
		})
	}
}

func TestCreateShortURLBatch(t *testing.T) {
	shortener := &shortenerStub{batchIDs: []string{"first-id", "second-id"}}
	router := NewRouter("http://localhost:8080", shortener)
	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(`[
        {"correlation_id":"first","original_url":"https://first.example"},
        {"correlation_id":"second","original_url":"https://second.example"}
    ]`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	var got []dto.BatchShortenResponse
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []dto.BatchShortenResponse{
		{CorrelationID: "first", ShortURL: "http://localhost:8080/first-id"},
		{CorrelationID: "second", ShortURL: "http://localhost:8080/second-id"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("response = %#v, want %#v", got, want)
	}
	if !slices.Equal(shortener.batchURLs, []string{"https://first.example", "https://second.example"}) {
		t.Fatalf("saved URLs = %v", shortener.batchURLs)
	}
}

func TestCreateShortURLBatchRejectsInvalidRequest(t *testing.T) {
	for _, body := range []string{"[]", `[{"correlation_id":"first","original_url":""}]`, `[{`} {
		t.Run(body, func(t *testing.T) {
			shortener := &shortenerStub{}
			router := NewRouter("http://localhost:8080", shortener)
			request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if shortener.batchURLs != nil {
				t.Fatalf("SaveBatch called with %v", shortener.batchURLs)
			}
		})
	}
}

func TestCreateShortURLBatchStorageError(t *testing.T) {
	shortener := &shortenerStub{batchErr: errors.New("storage unavailable")}
	router := NewRouter("http://localhost:8080", shortener)
	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(`[{"original_url":"https://example.com"}]`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestCreateShortURLBatchGzip(t *testing.T) {
	shortener := &shortenerStub{batchIDs: []string{"short-id"}}
	router := appgzip.MiddlewareGzip(zerolog.Nop())(NewRouter("http://localhost:8080", shortener))
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`[{"correlation_id":"one","original_url":"https://example.com"}]`)); err != nil {
		t.Fatalf("compress request: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close request compressor: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", &compressed)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated || response.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("status = %d, encoding = %q", response.Code, response.Header().Get("Content-Encoding"))
	}
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatalf("open compressed response: %v", err)
	}
	defer func() { _ = reader.Close() }()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read compressed response: %v", err)
	}
	if !bytes.Contains(body, []byte(`"correlation_id":"one"`)) {
		t.Fatalf("response body = %s", body)
	}
}

func TestGetOriginalURL(t *testing.T) {
	const originalURL = "https://practicum.yandex.ru/"
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))

	createRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(originalURL))
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("failed to prepare shortened URL: status = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	id := strings.TrimPrefix(createResponse.Body.String(), "http://localhost:8080/")

	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantLocation string
	}{
		{
			name:         "existing ID",
			path:         "/" + id,
			wantStatus:   http.StatusTemporaryRedirect,
			wantLocation: originalURL,
		},
		{
			name:       "unknown ID",
			path:       "/unknown",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty ID",
			path:       "/",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid path",
			path:       "/one/two",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			result := response.Result()
			defer func() { _ = result.Body.Close() }()

			if result.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.wantStatus)
				return
			}
			if location := result.Header.Get("Location"); location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", location, tt.wantLocation)
				return
			}
		})
	}
}

func TestGetOriginalURLStorageError(t *testing.T) {
	router := NewRouter("http://localhost:8080", &shortenerStub{getErr: errors.New("storage unavailable")})
	request := httptest.NewRequest(http.MethodGet, "/known", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}
