package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mikkims/ya/internal/auth"
	"github.com/mikkims/ya/internal/handler"
	"github.com/mikkims/ya/internal/model/dto"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
)

func exampleRouter() (http.Handler, *storage.Memory, *service.Shortener, func()) {
	old := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	memory := storage.NewMemory()
	shortener := service.NewShortener(memory)
	return handler.NewRouter("http://short.test", shortener), memory, shortener, func() { gin.SetMode(old) }
}
func exampleRequest(router http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost || method == http.MethodDelete {
		contentType := "application/json"
		if path == "/" {
			contentType = "text/plain"
		}
		req.Header.Set("Content-Type", contentType)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
func exampleCookie() *http.Cookie {
	token, err := auth.NewToken("example-user")
	if err != nil {
		panic(err)
	}
	return &http.Cookie{Name: "user_id", Value: token}
}
func exampleID(short string) string {
	u, err := url.Parse(short)
	if err != nil {
		panic(err)
	}
	return strings.TrimPrefix(u.Path, "/")
}
func exampleDecode(rec *httptest.ResponseRecorder, value any) {
	if err := json.Unmarshal(rec.Body.Bytes(), value); err != nil {
		panic(err)
	}
}

func ExampleNewRouter_shortenText() {
	router, _, _, restore := exampleRouter()
	defer restore()
	rec := exampleRequest(router, http.MethodPost, "/", "https://example.com/page", nil)
	fmt.Println(rec.Code, strings.HasPrefix(rec.Body.String(), "http://short.test/"), strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"))
	// Output: 201 true true
}
func ExampleNewRouter_shortenJSON() {
	router, _, _, restore := exampleRouter()
	defer restore()
	first := exampleRequest(router, http.MethodPost, "/api/shorten", `{"url":"https://example.com/page"}`, nil)
	second := exampleRequest(router, http.MethodPost, "/api/shorten", `{"url":"https://example.com/page"}`, nil)
	var a, b dto.ShortenResponse
	exampleDecode(first, &a)
	exampleDecode(second, &b)
	fmt.Println(first.Code, second.Code, a.Result == b.Result && strings.HasPrefix(a.Result, "http://short.test/"))
	// Output: 201 409 true
}
func ExampleNewRouter_follow() {
	router, memory, _, restore := exampleRouter()
	defer restore()
	if err := memory.Save(context.Background(), "known", "https://example.com/page"); err != nil {
		panic(err)
	}
	rec := exampleRequest(router, http.MethodGet, "/known", "", nil)
	fmt.Println(rec.Code, rec.Header().Get("Location"))
	// Output: 307 https://example.com/page
}
func ExampleNewRouter_shortenBatch() {
	router, memory, _, restore := exampleRouter()
	defer restore()
	rec := exampleRequest(router, http.MethodPost, "/api/shorten/batch", `[{"correlation_id":"one","original_url":"https://example.com/one"},{"correlation_id":"two","original_url":"https://example.com/two"}]`, nil)
	var items []dto.BatchShortenResponse
	exampleDecode(rec, &items)
	valid := len(items) == 2 && items[0].CorrelationID == "one" && items[1].CorrelationID == "two" && strings.HasPrefix(items[0].ShortURL, "http://short.test/") && strings.HasPrefix(items[1].ShortURL, "http://short.test/")
	if valid {
		for i, original := range []string{"https://example.com/one", "https://example.com/two"} {
			got, found, err := memory.Get(context.Background(), exampleID(items[i].ShortURL))
			if err != nil {
				panic(err)
			}
			valid = valid && found && got == original
		}
	}
	fmt.Println(rec.Code, valid)
	// Output: 201 true
}
func ExampleNewRouter_userURLs() {
	router, _, _, restore := exampleRouter()
	defer restore()
	cookie := exampleCookie()
	created := exampleRequest(router, http.MethodPost, "/", "https://example.com/page", cookie)
	rec := exampleRequest(router, http.MethodGet, "/api/user/urls", "", cookie)
	var items []dto.UserURL
	exampleDecode(rec, &items)
	fmt.Println(created.Code, rec.Code, len(items) == 1 && items[0].OriginalURL == "https://example.com/page" && items[0].ShortURL == created.Body.String())
	// Output: 201 200 true
}
func ExampleNewRouter_deleteUserURLs() {
	router, _, shortener, restore := exampleRouter()
	defer restore()
	cookie := exampleCookie()
	created := exampleRequest(router, http.MethodPost, "/", "https://example.com/page", cookie)
	id := exampleID(created.Body.String())
	payload, err := json.Marshal([]string{id})
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- shortener.RunDeleteWorker(ctx) }()
	rec := exampleRequest(router, http.MethodDelete, "/api/user/urls", string(payload), cookie)
	cancel()
	if err := <-done; err != nil {
		panic(err)
	}
	follow := exampleRequest(router, http.MethodGet, "/"+id, "", cookie)
	fmt.Println(rec.Code, follow.Code)
	// Output: 202 410
}

type examplePinger struct{}

func (examplePinger) PingContext(context.Context) error { return nil }
func ExampleNewRouterWithDatabase_ping() {
	old := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	defer gin.SetMode(old)
	router := handler.NewRouterWithDatabase("http://short.test", service.NewShortener(storage.NewMemory()), examplePinger{})
	rec := exampleRequest(router, http.MethodGet, "/ping", "", nil)
	fmt.Println(rec.Code)
	// Output: 200
}
