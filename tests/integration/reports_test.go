//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestReportsEndpoints(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	ownerID := registerUser(t, srv, "report-owner@example.com", "password123")
	outsiderID := registerUser(t, srv, "report-outsider@example.com", "password123")
	token := loginUser(t, srv, "report-owner@example.com", "password123")

	teamID := createTeam(t, srv, token, "analytics")
	taskID := createTask(t, srv, token, map[string]any{
		"team_id": teamID,
		"title":   "report task",
		"status":  "done",
	})

	if _, err := env.db.Exec(`UPDATE tasks SET assignee_id=? WHERE id=?`, outsiderID, taskID); err != nil {
		t.Fatalf("inject invalid assignee: %v", err)
	}

	code, body := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/reports/team-summaries", token, nil)
	if code != http.StatusOK {
		t.Fatalf("team-summaries status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	var teamSummaries []map[string]any
	if err := json.Unmarshal(body, &teamSummaries); err != nil {
		t.Fatalf("decode team summaries: %v (body=%s)", err, string(body))
	}
	if len(teamSummaries) == 0 {
		t.Fatal("team summaries should not be empty")
	}

	code, body = doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/reports/top-creators", token, nil)
	if code != http.StatusOK {
		t.Fatalf("top-creators status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	var topCreators []map[string]any
	if err := json.Unmarshal(body, &topCreators); err != nil {
		t.Fatalf("decode top creators: %v (body=%s)", err, string(body))
	}
	if len(topCreators) == 0 {
		t.Fatal("top creators should not be empty")
	}

	ownerSeen := false
	for _, row := range topCreators {
		if uid, ok := asUint64(row["user_id"]); ok && uid == ownerID {
			ownerSeen = true
			break
		}
	}
	if !ownerSeen {
		t.Fatalf("top creators does not contain owner %d: %+v", ownerID, topCreators)
	}

	code, body = doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/v1/reports/invalid-assignees", token, nil)
	if code != http.StatusOK {
		t.Fatalf("invalid-assignees status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	var invalid []map[string]any
	if err := json.Unmarshal(body, &invalid); err != nil {
		t.Fatalf("decode invalid assignees: %v (body=%s)", err, string(body))
	}
	if len(invalid) == 0 {
		t.Fatal("invalid assignees should not be empty")
	}

	taskSeen := false
	for _, row := range invalid {
		if tid, ok := asUint64(row["task_id"]); ok && tid == taskID {
			taskSeen = true
			break
		}
	}
	if !taskSeen {
		t.Fatalf("invalid-assignees does not contain task %d: %+v", taskID, invalid)
	}

	code, body = doJSON(t, srv.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/tasks?team_id=%d", srv.URL, teamID), token, nil)
	if code != http.StatusOK {
		t.Fatalf("sanity GET /tasks status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
}
