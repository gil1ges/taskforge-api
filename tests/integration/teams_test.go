//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gil1ges/taskforge-api/internal/domain"
)

func TestTeamsRequiresAuth(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	code, body := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/teams", "", nil)
	if code != http.StatusUnauthorized {
		t.Fatalf("GET /teams status = %d, want %d, body=%s", code, http.StatusUnauthorized, string(body))
	}
}

func TestTeamsCreateAndList(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	registerUser(t, srv, "team-owner@example.com", "password123")
	registerUser(t, srv, "team-member@example.com", "password123")

	token := loginUser(t, srv, "team-owner@example.com", "password123")
	memberToken := loginUser(t, srv, "team-member@example.com", "password123")

	teamID := createTeam(t, srv, token, "backend")

	code, body := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/teams", token, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /teams status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}

	var teams []domain.Team
	if err := json.Unmarshal(body, &teams); err != nil {
		t.Fatalf("decode teams: %v (body=%s)", err, string(body))
	}
	if len(teams) != 1 {
		t.Fatalf("teams len = %d, want 1", len(teams))
	}
	if teams[0].ID != teamID {
		t.Fatalf("team id = %d, want %d", teams[0].ID, teamID)
	}

	inviteCode, _ := inviteMember(t, srv, token, teamID, "team-member@example.com", "member")
	if inviteCode == "" {
		t.Fatal("expected non-empty invite code")
	}

	code, body = doJSON(t, srv.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/teams/%d/accept", srv.URL, teamID), memberToken, map[string]any{
		"code": inviteCode,
	})
	if code != http.StatusOK {
		t.Fatalf("POST /teams/{id}/accept status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}

	code, body = doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/teams", memberToken, nil)
	if code != http.StatusOK {
		t.Fatalf("member GET /teams status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	teams = nil
	if err := json.Unmarshal(body, &teams); err != nil {
		t.Fatalf("decode member teams: %v (body=%s)", err, string(body))
	}
	if len(teams) != 1 || teams[0].ID != teamID {
		t.Fatalf("member teams = %+v, want one team id %d", teams, teamID)
	}
}

func inviteMember(t *testing.T, srv *httptest.Server, token string, teamID uint64, email, role string) (string, map[string]any) {
	t.Helper()
	code, body := doJSON(t, srv.Client(), http.MethodPost, fmt.Sprintf("%s/api/v1/teams/%d/invite", srv.URL, teamID), token, map[string]any{
		"email": email,
		"role":  role,
	})
	if code != http.StatusOK {
		t.Fatalf("POST /teams/{id}/invite status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	resp := decodeMap(t, body)
	rawCode, ok := asString(resp["code"])
	if !ok || rawCode == "" {
		t.Fatalf("invite response missing code: %+v", resp)
	}
	return rawCode, resp
}
