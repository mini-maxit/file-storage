package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// LoggingMiddleware logs details of each HTTP request.
func LoggingMiddleware(next http.Handler, log *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)
		protocol := "http"
		if r.TLS != nil {
			protocol = "https"
		}
		log.Infof("method=%s path=%s host=%s service=%dms bytes=%d protocol=%s", r.Method, r.URL.Path, r.URL.Hostname(), time.Since(start).Milliseconds(), r.ContentLength, protocol)
	})
}
