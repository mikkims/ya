package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateShortURL(t *testing.T) {
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

			createShortURL(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusCreated {
				if contentType := result.Header.Get("Content-Type"); contentType != "text/plain" {
					t.Errorf("Content-Type = %q, want %q", contentType, "text/plain")
				}

				shortURL := response.Body.String()
				if !strings.HasPrefix(shortURL, baseURL) {
					t.Errorf("short URL = %q, want prefix %q", shortURL, baseURL)
				}
				if len(strings.TrimPrefix(shortURL, baseURL)) != idLength {
					t.Errorf("short URL ID must contain %d characters", idLength)
				}
			}
		})
	}
}

func TestGetOriginalURL(t *testing.T) {
	const (
		id          = "EwHXdJfB"
		originalURL = "https://practicum.yandex.ru/"
	)

	mu.Lock()
	urls[id] = originalURL
	mu.Unlock()

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

			getOriginalURL(response, request)

			result := response.Result()
			defer result.Body.Close()

			if result.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", result.StatusCode, tt.wantStatus)
			}
			if location := result.Header.Get("Location"); location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", location, tt.wantLocation)
			}
		})
	}
}
