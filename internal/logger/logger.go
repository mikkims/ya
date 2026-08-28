package logger

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type responseData struct {
	status int
	size   int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.responseData.status != 0 {
		return
	}
	w.responseData.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	if w.responseData.status == 0 {
		w.responseData.status = http.StatusOK
	}
	size, err := w.ResponseWriter.Write(data)
	w.responseData.size += size
	return size, err
}

func Middleware(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			response := &responseData{}
			writer := &loggingResponseWriter{ResponseWriter: w, responseData: response}

			next.ServeHTTP(writer, r)

			if response.status == 0 {
				response.status = http.StatusOK
			}
			log.Info().
				Str("uri", r.RequestURI).
				Str("method", r.Method).
				Dur("duration", time.Since(startedAt)).
				Msg("request handled")
			log.Info().
				Int("status", response.status).
				Int("size", response.size).
				Msg("response sent")
		})
	}
}
