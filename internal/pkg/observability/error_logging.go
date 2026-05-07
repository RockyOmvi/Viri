package observability

import (
	"net/http"
	"time"

	"github.com/viri-chain/viri/internal/layer1/logging"
)

func ErrorLoggingMiddleware(next http.Handler, logger *logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		duration := time.Since(start)

		if sw.status >= 400 {
			reqID := RequestIDFromContext(r.Context())
			logger.WithField("request_id", reqID).
				WithField("method", r.Method).
				WithField("path", r.URL.Path).
				WithField("status", sw.status).
				WithField("duration", duration.String()).
				WithField("remote_addr", r.RemoteAddr).
				Warn("HTTP request error")
		}
	})
}
