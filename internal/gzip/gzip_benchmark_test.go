package gzip

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkJSON(b *testing.B) {
	body := []byte(`{"urls":[{"original_url":"https://example.com/path","short_url":"http://localhost/id"}]}`)
	h := MiddlewareGzip(zerolog.Nop())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK || !strings.EqualFold(w.Header().Get("Content-Encoding"), "gzip") {
			b.Fatal("unexpected response")
		}
	}
}
