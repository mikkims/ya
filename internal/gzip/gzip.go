package gzip

import (
	"bytes"
	compressgzip "compress/gzip"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *responseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func MiddlewareGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
			reader, err := compressgzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			defer func(reader *compressgzip.Reader) {
				err := reader.Close()
				if err != nil {

				}
			}(reader)
			r.Body = reader
			r.ContentLength = -1
			r.Header.Del("Content-Encoding")
			r.Header.Del("Content-Length")
		}

		writer := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		writer.flush(acceptsGzip(r.Header.Get("Accept-Encoding")))
	})
}

func (w *responseWriter) flush(acceptsCompression bool) {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}

	if acceptsCompression && compressible(w.Header().Get("Content-Type")) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
		w.ResponseWriter.WriteHeader(status)
		writer := compressgzip.NewWriter(w.ResponseWriter)
		_, _ = writer.Write(w.body.Bytes())
		_ = writer.Close()
		return
	}

	w.ResponseWriter.WriteHeader(status)
	_, _ = w.ResponseWriter.Write(w.body.Bytes())
}

func compressible(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || mediaType == "text/html"
}

func acceptsGzip(header string) bool {
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(value, ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "gzip") {
			continue
		}
		for _, parameter := range parts[1:] {
			name, rawValue, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(name, "q") {
				quality, err := strconv.ParseFloat(rawValue, 64)
				return err == nil && quality > 0
			}
		}
		return true
	}
	return false
}
