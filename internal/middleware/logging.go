package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status  int
	size    int
	written bool
}

// WriteHeader сохраняет статус-код, если заголовки еще не были отправлены.
func (rr *responseRecorder) WriteHeader(code int) {
	if rr.written {
		return
	}
	rr.status = code
	rr.written = true
	rr.ResponseWriter.WriteHeader(code)
}

// Write записывает тело ответа.
// Подсчитывает количество байт.
func (rr *responseRecorder) Write(b []byte) (int, error) {
	if rr.status == 0 {
		rr.WriteHeader(http.StatusOK)
	}
	n, err := rr.ResponseWriter.Write(b)
	rr.size += n
	return n, err
}

// Size возвращает объем записанного тела ответа.
func (rr *responseRecorder) Size() int {
	return rr.size
}

// Logger — это middleware обработчик для логирования запросов.
// Фиксирует метод, URI, время выполнения, а также статус и размер ответа от сервера.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rr := &responseRecorder{ResponseWriter: w}
			next.ServeHTTP(rr, r)
			log.Info(
				"server",
				"req.method", r.Method,
				"req.uri", r.RequestURI,
				"req.duration", time.Since(start).Seconds(),
				"res.status", rr.status,
				"res.size", rr.Size(),
			)
		})
	}
}
