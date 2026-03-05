//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gil1ges/taskforge-api/internal/domain"
)

func TestTasksCreateListUpdateHistory(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	registerUser(t, srv, "tasks-user@example.com", "password123")
	token := loginUser(t, srv, "tasks-user@example.com", "password123")
	teamID := createTeam(t, srv, token, "platform")

	taskID := createTask(t, srv, token, map[string]any{
		"team_id": teamID,
		"title":   "first task",
		"status":  "todo",
	})

	code, body := doJSON(t, srv.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/tasks?team_id=%d", srv.URL, teamID), token, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /tasks status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	tasks := decodeTasks(t, body)
	if len(tasks) != 1 {
		t.Fatalf("tasks len = %d, want 1", len(tasks))
	}
	if tasks[0].ID != taskID {
		t.Fatalf("task id = %d, want %d", tasks[0].ID, taskID)
	}

	code, body = doJSON(t, srv.Client(), http.MethodPut, fmt.Sprintf("%s/api/v1/tasks/%d", srv.URL, taskID), token, map[string]any{
		"status": "in_progress",
	})
	if code != http.StatusOK {
		t.Fatalf("PUT /tasks/{id} status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	updated := decodeTask(t, body)
	if updated.Status != domain.StatusInProgress {
		t.Fatalf("updated status = %v, want %v", updated.Status, domain.StatusInProgress)
	}

	code, body = doJSON(t, srv.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/tasks/%d/history", srv.URL, taskID), token, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /tasks/{id}/history status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}
	history := decodeHistory(t, body)
	if len(history) == 0 {
		t.Fatal("expected non-empty task history")
	}
	foundStatusChange := false
	for _, h := range history {
		if h.FieldName == "status" {
			foundStatusChange = true
			break
		}
	}
	if !foundStatusChange {
		t.Fatalf("history does not contain status change: %+v", history)
	}
}

func TestTasksListWorksWhenRedisUnavailable(t *testing.T) {
	env := requireIntegration(t)
	srv := env.newAPIServer(t, env.redisAddr, testRateLimitPerM)

	registerUser(t, srv, "cache-user@example.com", "password123")
	token := loginUser(t, srv, "cache-user@example.com", "password123")
	teamID := createTeam(t, srv, token, "infra")
	createTask(t, srv, token, map[string]any{
		"team_id": teamID,
		"title":   "cache baseline task",
		"status":  "todo",
	})

	code, body := doJSON(t, srv.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/tasks?team_id=%d", srv.URL, teamID), token, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /tasks with redis status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}

	unavailableRedis := "127.0.0.1:1"
	srvNoRedis := env.newAPIServer(t, unavailableRedis, testRateLimitPerM)

	code, body = doJSON(t, srvNoRedis.Client(), http.MethodGet, fmt.Sprintf("%s/api/v1/tasks?team_id=%d", srvNoRedis.URL, teamID), token, nil)
	if code == http.StatusInternalServerError {
		t.Fatalf("GET /tasks returned 500 when redis unavailable, body=%s", string(body))
	}
	if code != http.StatusOK {
		t.Fatalf("GET /tasks status = %d, want %d, body=%s", code, http.StatusOK, string(body))
	}

	if !strings.HasPrefix(strings.TrimSpace(string(body)), "[") {
		t.Fatalf("expected tasks array json, body=%s", string(body))
	}
}
