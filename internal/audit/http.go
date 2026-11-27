package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTP отправляет события аудита по HTTP.
type HTTP struct {
	url    string
	client *http.Client
}

// NewHTTP создаёт получателя для абсолютного HTTP или HTTPS URL либо возвращает ошибку.
// Запросы имеют тайм-аут три секунды и не следуют перенаправлениям.
func NewHTTP(address string) (*HTTP, error) {
	u, err := url.Parse(address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("invalid audit URL")
	}
	return &HTTP{url: address, client: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Name возвращает имя получателя.
func (*HTTP) Name() string { return "http" }

// Observe отправляет событие методом POST с указанным контекстом и возвращает ошибку при ответе вне 2xx.
func (h *HTTP) Observe(ctx context.Context, event Event) (result error) {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && result == nil {
			result = err
		}
	}()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("audit HTTP status is not 2xx")
	}
	return nil
}
