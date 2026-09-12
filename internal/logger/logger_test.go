package logger

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestMiddleware(t *testing.T) {
	var output bytes.Buffer
	log := zerolog.New(&output)
	handler := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("response"))
	}))
	request := httptest.NewRequest(http.MethodPost, "/?source=test", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("log records = %d, want 2; output: %s", len(lines), output.String())
	}

	var requestLog map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &requestLog); err != nil {
		t.Fatalf("decode request log: %v", err)
	}
	if requestLog["level"] != "info" || requestLog["uri"] != "/?source=test" || requestLog["method"] != http.MethodPost {
		t.Errorf("unexpected request log: %v", requestLog)
	}
	if duration, ok := requestLog["duration"].(float64); !ok || duration < 0 {
		t.Errorf("duration = %v, want a non-negative number", requestLog["duration"])
	}

	var responseLog map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("decode response log: %v", err)
	}
	if responseLog["level"] != "info" || responseLog["status"] != float64(http.StatusCreated) || responseLog["size"] != float64(len("response")) {
		t.Errorf("unexpected response log: %v", responseLog)
	}
}
