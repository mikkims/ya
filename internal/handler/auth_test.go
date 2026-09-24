package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikkims/ya/internal/model/dto"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
)

func TestUserURLs(t *testing.T) {
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))

	createRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	cookies := createResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != userCookieName {
		t.Fatalf("cookies = %v, want %q cookie", cookies, userCookieName)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	listRequest.AddCookie(cookies[0])
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var urls []dto.UserURL
	if err := json.NewDecoder(listResponse.Body).Decode(&urls); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(urls) != 1 || urls[0].OriginalURL != "https://example.com" || urls[0].ShortURL != createResponse.Body.String() {
		t.Fatalf("response = %#v", urls)
	}
}

func TestUserURLsEmpty(t *testing.T) {
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if len(response.Result().Cookies()) != 1 {
		t.Fatal("new user cookie was not set")
	}
}

func TestUserURLsRejectsCookieWithoutUserID(t *testing.T) {
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	request.AddCookie(&http.Cookie{Name: userCookieName, Value: signUserID("")})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestInvalidCookieIsReplaced(t *testing.T) {
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	request.AddCookie(&http.Cookie{Name: userCookieName, Value: "invalid"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value == "invalid" {
		t.Fatalf("cookie was not replaced: %v", cookies)
	}
}

func TestDeleteUserURLs(t *testing.T) {
	router := NewRouter("http://localhost:8080", service.NewShortener(storage.NewMemory()))
	createRequest := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	cookie := createResponse.Result().Cookies()[0]
	id := strings.TrimPrefix(createResponse.Body.String(), "http://localhost:8080/")

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/user/urls", strings.NewReader(`["`+id+`"]`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRequest.AddCookie(cookie)
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d, want %d", deleteResponse.Code, http.StatusAccepted)
	}

	deadline := time.Now().Add(time.Second)
	for {
		getRequest := httptest.NewRequest(http.MethodGet, "/"+id, nil)
		getRequest.AddCookie(cookie)
		getResponse := httptest.NewRecorder()
		router.ServeHTTP(getResponse, getRequest)
		if getResponse.Code == http.StatusGone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("get status = %d, want eventual %d", getResponse.Code, http.StatusGone)
		}
		time.Sleep(time.Millisecond)
	}
}
