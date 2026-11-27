package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type closeErrorBody struct{ io.Reader }

func (closeErrorBody) Close() error { return errors.New("close failed") }

type responseTransport struct{}

func (responseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: closeErrorBody{Reader: bytes.NewReader(nil)}, Header: make(http.Header)}, nil
}

func TestHTTPCloseError(t *testing.T) {
	h, err := NewHTTP("http://example.test")
	if err != nil {
		t.Fatal(err)
	}
	h.client.Transport = responseTransport{}
	if err := h.Observe(context.Background(), Event{}); err == nil {
		t.Fatal("expected close error")
	}
}

type testObserver struct {
	events []Event
	fail   bool
}

func (o *testObserver) Name() string { return "test" }
func (o *testObserver) Observe(_ context.Context, e Event) error {
	o.events = append(o.events, e)
	if o.fail {
		return errors.New("secret")
	}
	return nil
}
func TestPublisherContinues(t *testing.T) {
	a, b := &testObserver{fail: true}, &testObserver{}
	e := Event{Timestamp: 42, Action: "shorten", UserID: "user", URL: "https://example.test"}
	NewPublisher(a, b).Publish(context.Background(), e)
	if len(a.events) != 1 || len(b.events) != 1 || b.events[0] != e {
		t.Fatal("event was not delivered to both observers")
	}
}
func TestFileAppendConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit")
	if err := os.WriteFile(path, []byte("existing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := NewFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f.Observe(context.Background(), Event{URL: "line\nbreak"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := data.Close(); err != nil {
			t.Errorf("close audit file: %v", err)
		}
	}()
	s := bufio.NewScanner(data)
	if !s.Scan() || s.Text() != "existing" {
		t.Fatal("existing content lost")
	}
	count := 0
	for s.Scan() {
		var e Event
		if err := json.Unmarshal(s.Bytes(), &e); err != nil || e.URL != "line\nbreak" {
			t.Fatal("invalid JSON line", err)
		}
		count++
	}
	if s.Err() != nil || count != 50 {
		t.Fatalf("lines: %d, error: %v", count, s.Err())
	}
}
func TestHTTP(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("incorrect request")
		}
		var event Event
		if json.NewDecoder(r.Body).Decode(&event) != nil || event.URL != "https://example.test" {
			t.Error("incorrect body")
		}
		w.Header().Set("Location", "/other")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	h, err := NewHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if h.Observe(context.Background(), Event{URL: "https://example.test"}) == nil || calls != 1 {
		t.Fatal("redirect should fail without retry")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if h.Observe(ctx, Event{}) == nil {
		t.Fatal("cancelled request succeeded")
	}
	for _, u := range []string{"relative", "ftp://example.test", "http://"} {
		if _, err := NewHTTP(u); err == nil {
			t.Errorf("accepted %q", u)
		}
	}
}
