package middleware

import (
	"context"
	"net/http"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/go-chi/jwtauth/v5"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

func UserIDFromContext(ctx context.Context) (uint64, bool) {
	v := ctx.Value(userIDKey)
	id, ok := v.(uint64)
	return id, ok
}

func AuthRequired(tokenAuth *jwtauth.JWTAuth) func(http.Handler) http.Handler {
	verifier := jwtauth.Verifier(tokenAuth)
	authenticator := jwtauth.Authenticator(tokenAuth)

	return func(next http.Handler) http.Handler {
		return verifier(authenticator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, claims, _ := jwtauth.FromContext(r.Context())
			raw, ok := claims["user_id"]
			if !ok {
				http.Error(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
				return
			}
			f, ok := raw.(float64)
			if !ok {
				http.Error(w, domain.ErrUnauthorized.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, uint64(f))
			next.ServeHTTP(w, r.WithContext(ctx))
		})))
	}
}
