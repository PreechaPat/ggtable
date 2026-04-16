package middle

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 1. A minimal wrapper just to capture the HTTP status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader intercepts the status code before sending it to the client
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// 2. Logging Middleware
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the writer. We default to 200 OK just in case the handler
			// doesn't explicitly call WriteHeader.
			recorder := &statusRecorder{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			// Let the request pass down the chain to your actual handler
			next.ServeHTTP(recorder, r)

			// Log the result AFTER the handler has finished
			logger.Info("HTTP Request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", recorder.status),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

// 3. Request ID Middleware
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate a simple UUID
			reqID := "req-" + uuid.New().String()

			// Set it in the header so the client/frontend can see it
			w.Header().Set("X-Request-ID", reqID)

			// Pass the request down the chain
			next.ServeHTTP(w, r)
		})
	}
}
