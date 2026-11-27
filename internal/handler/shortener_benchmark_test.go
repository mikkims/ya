package handler

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mikkims/ya/internal/auth"
	appgzip "github.com/mikkims/ya/internal/gzip"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
	"github.com/rs/zerolog"
)

func benchmarkRouter(b *testing.B) (http.Handler, *http.Cookie) {
	b.Helper()
	store := storage.NewMemory()
	for user := range 10 {
		ctx := auth.WithUserID(context.Background(), fmt.Sprintf("user-%d", user))
		for i := range 100 {
			key := fmt.Sprintf("%d-%d", user, i)
			if err := store.Save(ctx, key, "https://example.com/"+key); err != nil {
				b.Fatal(err)
			}
		}
	}
	token, err := auth.NewToken("user-0")
	if err != nil {
		b.Fatal(err)
	}
	return appgzip.MiddlewareGzip(zerolog.Nop())(NewRouter("http://localhost:8080", service.NewShortener(store))), &http.Cookie{Name: userCookieName, Value: token}
}

func BenchmarkRouterUserURLsGzip(b *testing.B) {
	router, cookie := benchmarkRouter(b)
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	request.AddCookie(cookie)
	check := httptest.NewRecorder()
	router.ServeHTTP(check, request)
	if check.Code != http.StatusOK || check.Header().Get("Content-Encoding") != "gzip" {
		b.Fatal("unexpected response")
	}
	zr, err := gzip.NewReader(check.Body)
	if err != nil {
		b.Fatal(err)
	}
	var urls []map[string]string
	if err := json.NewDecoder(zr).Decode(&urls); err != nil {
		b.Fatal(err)
	}
	if err := zr.Close(); err != nil {
		b.Fatal(err)
	}
	if len(urls) != 100 {
		b.Fatalf("got %d URLs", len(urls))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatal(response.Code)
		}
	}
}

func BenchmarkRouter(b *testing.B) {
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
	}{
		{"Get", http.MethodGet, "/0-0", "", http.StatusTemporaryRedirect},
		{"UserURLs", http.MethodGet, "/api/user/urls", "", http.StatusOK},
	} {
		b.Run(tc.name, func(b *testing.B) {
			router, cookie := benchmarkRouter(b)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.AddCookie(cookie)
				if tc.body != "" {
					req.Header.Set("Content-Type", "application/json")
				}
				resp := httptest.NewRecorder()
				router.ServeHTTP(resp, req)
				if resp.Code != tc.status {
					b.Fatal(resp.Code)
				}
				_, _ = io.Copy(io.Discard, resp.Body)
			}
		})
	}
}

func BenchmarkRouterPost(b *testing.B) {
	router, cookie := benchmarkRouter(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i > 0 && i%1000 == 0 {
			b.StopTimer()
			router, cookie = benchmarkRouter(b)
			b.StartTimer()
		}
		body := fmt.Sprintf(`{"url":"https://example.com/new-%d"}`, i%1000)
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusCreated {
			b.Fatal(resp.Code)
		}
	}
}
