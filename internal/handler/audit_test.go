package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mikkims/ya/internal/audit"
	"github.com/mikkims/ya/internal/service"
	"github.com/mikkims/ya/internal/storage"
)

type auditRecorder struct{ events []audit.Event }

func (*auditRecorder) Name() string { return "test" }
func (r *auditRecorder) Observe(_ context.Context, e audit.Event) error {
	r.events = append(r.events, e)
	return nil
}
func TestAuditRoutes(t *testing.T) {
	rec := &auditRecorder{}
	router := NewRouterWithAudit("http://localhost", service.NewShortener(storage.NewMemory()), nil, audit.NewPublisher(rec))
	start := time.Now().Unix()
	req := httptest.NewRequest("POST", "/", strings.NewReader("https://example.test/path"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 201 || len(rec.events) != 1 {
		t.Fatalf("status %d, events %d", w.Code, len(rec.events))
	}
	cookie := w.Result().Cookies()[0]
	id := strings.TrimPrefix(w.Body.String(), "http://localhost/")
	req = httptest.NewRequest("GET", "/"+id, nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 307 || len(rec.events) != 2 {
		t.Fatalf("status %d, events %d", w.Code, len(rec.events))
	}
	req = httptest.NewRequest("POST", "/api/shorten", strings.NewReader(`{"url":"https://other.test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 201 || len(rec.events) != 3 {
		t.Fatalf("status %d, events %d", w.Code, len(rec.events))
	}
	for i, e := range rec.events {
		if e.Timestamp < start || e.Timestamp > time.Now().Unix() || e.UserID == "" || e.UserID != rec.events[0].UserID {
			t.Fatalf("invalid event %d: %+v", i, e)
		}
		b, _ := json.Marshal(e)
		var fields map[string]any
		_ = json.Unmarshal(b, &fields)
		if len(fields) != 4 {
			t.Fatal("incorrect event fields")
		}
	}
	if rec.events[0].Action != "shorten" || rec.events[0].URL != "https://example.test/path" || rec.events[1].Action != "follow" || rec.events[1].URL != rec.events[0].URL || rec.events[2].URL != "https://other.test" {
		t.Fatal("incorrect events")
	}
	req = httptest.NewRequest("POST", "/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || len(rec.events) != 3 {
		t.Fatal("error published")
	}
}
