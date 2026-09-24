package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

type compressWriter struct {
	w              http.ResponseWriter
	zw             *gzip.Writer
	wroteHeader    bool
	shouldCompress bool
}

func newCompressWriter(w http.ResponseWriter) *compressWriter {
	return &compressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header возвращает заголовки базового ResponseWriter.
func (c *compressWriter) Header() http.Header {
	return c.w.Header()
}

// Write записывает тело ответа. При первой записи определяет необходимость сжатия по Content-Type.
// Если сжатие включено, данные направляются в gzip.Writer, иначе — напрямую клиенту.
func (c *compressWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.checkAndSetCompression(http.StatusOK)
		c.w.WriteHeader(http.StatusOK)
	}

	if c.shouldCompress {
		return c.zw.Write(p)
	}
	return c.w.Write(p)
}

func (c *compressWriter) checkAndSetCompression(statusCode int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true

	if statusCode >= 300 {
		return
	}

	contentType := c.w.Header().Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") ||
		strings.HasPrefix(contentType, "text/html") ||
		strings.HasPrefix(contentType, "text/plain") {

		c.shouldCompress = true

		c.w.Header().Set("Content-Encoding", "gzip")
		c.w.Header().Add("Vary", "Accept-Encoding")
		c.w.Header().Del("Content-Length")
	}
}

// WriteHeader перехватывает установку статуса для принятия решения о сжатии до отправки заголовков.
func (c *compressWriter) WriteHeader(statusCode int) {
	c.checkAndSetCompression(statusCode)
	c.w.WriteHeader(statusCode)
}

// Close завершает операцию. Сбрасывает буфер gzip.Writer в сеть, если сжатие было активно.
// Должен вызываться через defer сразу после создания экземпляра.
func (c *compressWriter) Close() error {
	if !c.wroteHeader {
		c.checkAndSetCompression(http.StatusOK)
		c.w.WriteHeader(http.StatusOK)
	}

	if c.shouldCompress {
		return c.zw.Close()
	}
	return nil
}

// GzipResponseMiddleware — middleware для включения Gzip-сжатия ответов.
// Проверяет наличие "gzip" в заголовке Accept-Encoding запроса клиента.
// Если клиент поддерживает сжатие, заменяет стандартный ResponseWriter на compressWriter.
func GzipResponseMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding := r.Header.Get("Accept-Encoding")

		if !strings.Contains(acceptEncoding, "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		cw := newCompressWriter(w)
		defer func() {
			_ = cw.Close()
		}()

		next.ServeHTTP(cw, r)
	})
}
