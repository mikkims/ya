package dto

// ShortenRequest содержит исходный URL запроса.
type ShortenRequest struct {
	// URL исходный URL.
	URL string `json:"url"`
}

// ShortenResponse содержит короткий URL ответа.
type ShortenResponse struct {
	// Result полученный короткий URL.
	Result string `json:"result"`
}

// BatchShortenRequest описывает элемент пакетного запроса.
type BatchShortenRequest struct {
	// CorrelationID идентификатор для сопоставления запроса и ответа.
	CorrelationID string `json:"correlation_id"`
	// OriginalURL исходный URL.
	OriginalURL string `json:"original_url"`
}

// BatchShortenResponse описывает элемент пакетного ответа.
type BatchShortenResponse struct {
	// CorrelationID идентификатор для сопоставления запроса и ответа.
	CorrelationID string `json:"correlation_id"`
	// ShortURL короткий URL.
	ShortURL string `json:"short_url"`
}

// UserURL содержит пару URL пользователя.
type UserURL struct {
	// ShortURL короткий URL.
	ShortURL string `json:"short_url"`
	// OriginalURL исходный URL.
	OriginalURL string `json:"original_url"`
}
