//go:build integration

package integration

import (
	"net/http"
	"testing"
)

func TestAuthRegisterAndLogin(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	registerUser(t, srv, "alice@example.com", "password123")
	token := loginUser(t, srv, "alice@example.com", "password123")
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestAuthLoginInvalidPassword(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	registerUser(t, srv, "bob@example.com", "password123")

	code, body := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/v1/login", "", map[string]any{
		"email":    "bob@example.com",
		"password": "wrong-password",
	})
	if code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want %d, body=%s", code, http.StatusUnauthorized, string(body))
	}
}

func TestRateLimitReturns429(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, 2)

	for i := 0; i < 2; i++ {
		code, body := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/health", "", nil)
		if code != http.StatusOK {
			t.Fatalf("health request #%d status = %d, want %d, body=%s", i+1, code, http.StatusOK, string(body))
		}
	}

	code, body := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/health", "", nil)
	if code != http.StatusTooManyRequests {
		t.Fatalf("third health status = %d, want %d, body=%s", code, http.StatusTooManyRequests, string(body))
	}
}
