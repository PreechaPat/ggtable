// Package middle holds HTTP middleware shared by all routes.
package middle

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yumyai/ggtable/logger"
	"go.uber.org/zap"
)

// RequestIDHeader carries the ID that ties a response to its log line.
const RequestIDHeader = "X-Request-ID"

// statusRecorder captures the status code a handler writes.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// RequestID gives every response an X-Request-ID header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(RequestIDHeader, "req-"+uuid.New().String())
		next.ServeHTTP(w, r)
	})
}

// Logging logs one line per request after it is served. Health checks are
// logged at debug level so the container healthcheck does not flood the log.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rec.status),
			zap.Duration("duration", time.Since(start)),
			zap.String("request_id", w.Header().Get(RequestIDHeader)),
		}
		if r.URL.Path == "/api/v1/health" {
			logger.Debug("HTTP request", fields...)
			return
		}
		logger.Info("HTTP request", fields...)
	})
}
