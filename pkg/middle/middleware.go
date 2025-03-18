package middle

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	zap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func DonothingMiddleWare(someThing string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Logic here

			// Call the next handler
			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

// responseWriter is a minimal wrapper for http.ResponseWriter that allows the
// written HTTP status code to be captured for logging.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        []byte
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

func (rw *responseWriter) Write(body []byte) (int, error) {
	rw.body = body
	return rw.ResponseWriter.Write(body)
}

// LoggingMiddleware logs the incoming HTTP request & its duration.
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := wrapResponseWriter(w)

			defer func() {
				if err := recover(); err != nil {
					wrapped.WriteHeader(http.StatusInternalServerError)
					logger.Error("Internal Server Error",
						zap.Any("panic", err),
						zap.String("stack", string(debug.Stack())),
					)
				}

				duration := time.Since(start)
				logger.Debug("Request completed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.EscapedPath()),
					zap.Int("status", wrapped.Status()),
					zap.Duration("duration", duration),
					zap.String("client_ip", r.RemoteAddr),
					zap.String("user_agent", r.UserAgent()),
					zap.Object("headers", zapcore.ObjectMarshalerFunc(func(enc zapcore.ObjectEncoder) error {
						for k, v := range r.Header {
							enc.AddString(k, fmt.Sprintf("%v", v))
						}
						return nil
					})),
				)

				// Log slow requests
				if duration > 1*time.Second {
					logger.Warn("Slow request",
						zap.String("method", r.Method),
						zap.String("path", r.URL.EscapedPath()),
						zap.Duration("duration", duration),
					)
				}
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", requestID)

			logger := logger.With(zap.String("request_id", requestID))
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "logger", logger)))
		})
	}
}

func generateRequestID() string {
	// Implement your request ID generation logic here
	// For example, you could use a UUID generator
	return "req-" + uuid.New().String()
}

func CreateMiddlewareLogger(level zapcore.Level) *zap.Logger {
	var err error
	config := zap.NewDevelopmentConfig()

	config.Level = zap.NewAtomicLevelAt(level)

	enccoderConfig := zap.NewDevelopmentEncoderConfig()
	zapcore.TimeEncoderOfLayout("Jan _2 15:04:05.000000000")
	enccoderConfig.StacktraceKey = "" // to hide stacktrace info
	config.EncoderConfig = enccoderConfig

	zapLog, err := config.Build(zap.AddCallerSkip(1))

	if err != nil {
		panic(err)
	}

	return zapLog

}
