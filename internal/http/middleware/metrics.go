package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gil1ges/taskforge-api/internal/observability"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func Metrics(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			start := time.Now()
			next.ServeHTTP(ww, r)
			dur := time.Since(start).Seconds()

			path := r.URL.Path
			status := ww.Status()

			m.RequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(status)).Inc()
			m.RequestLatency.WithLabelValues(r.Method, path).Observe(dur)
			if status >= 400 {
				m.ErrorsTotal.WithLabelValues(path, strconv.Itoa(status)).Inc()
			}
		})
	}
}
