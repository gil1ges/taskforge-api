package middleware

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
)

func RateLimit(perMin int) func(http.Handler) http.Handler {
	return httprate.Limit(
		perMin,
		1*time.Minute,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			if uid, ok := UserIDFromContext(r.Context()); ok {
				return "u:" + strconv.FormatUint(uid, 10), nil
			}
			return "ip:" + clientIP(r), nil
		}),
	)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
