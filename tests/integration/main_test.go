//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/gil1ges/taskforge-api/internal/domain"
	"github.com/gil1ges/taskforge-api/internal/http/handler"
	hmw "github.com/gil1ges/taskforge-api/internal/http/middleware"
	mysqlrepo "github.com/gil1ges/taskforge-api/internal/repo/mysql"
	redisrepo "github.com/gil1ges/taskforge-api/internal/repo/redis"
	"github.com/gil1ges/taskforge-api/internal/service"
)

const (
	testJWTSecret     = "integration-test-secret"
	testRateLimitPerM = 1000
)

type integrationEnv struct {
	mysqlDSN      string
	redisAddr     string
	redisPassword string
	redisDB       int

	db    *sqlx.DB
	redis *redis.Client

	mysqlContainer *mysqlcontainer.MySQLContainer
	redisContainer testcontainers.Container
}

var (
	envOnce sync.Once
	envInst *integrationEnv
	envErr  error
)

func TestMain(m *testing.M) {
	if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}

	code := m.Run()

	if envInst != nil {
		if envInst.redis != nil {
			_ = envInst.redis.Close()
		}
		if envInst.db != nil {
			_ = envInst.db.Close()
		}
		if envInst.redisContainer != nil {
			_ = testcontainers.TerminateContainer(envInst.redisContainer)
		}
		if envInst.mysqlContainer != nil {
			_ = testcontainers.TerminateContainer(envInst.mysqlContainer)
		}
	}

	os.Exit(code)
}

func requireIntegration(t *testing.T) *integrationEnv {
	t.Helper()

	testcontainers.SkipIfProviderIsNotHealthy(t)

	envOnce.Do(func() {
		envInst, envErr = setupIntegrationEnv()
	})
	if envErr != nil {
		t.Fatalf("integration setup failed: %v", envErr)
	}
	if err := envInst.resetState(context.Background()); err != nil {
		t.Fatalf("reset state: %v", err)
	}
	return envInst
}

func setupIntegrationEnv() (*integrationEnv, error) {
	rootDir, err := repoRoot()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	mysqlContainer, err := mysqlcontainer.Run(
		ctx,
		"mysql:8.0.36",
		mysqlcontainer.WithDatabase("taskforge"),
		mysqlcontainer.WithUsername("taskforge"),
		mysqlcontainer.WithPassword("taskforge"),
		mysqlcontainer.WithScripts(
			filepath.Join(rootDir, "migrations", "0001_init.up.sql"),
			filepath.Join(rootDir, "migrations", "0002_invites.up.sql"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start mysql container: %w", err)
	}

	mysqlDSN, err := mysqlContainer.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		_ = testcontainers.TerminateContainer(mysqlContainer)
		return nil, fmt.Errorf("mysql connection string: %w", err)
	}
	db, err := sqlx.Connect("mysql", mysqlDSN)
	if err != nil {
		_ = testcontainers.TerminateContainer(mysqlContainer)
		return nil, fmt.Errorf("connect mysql: %w", err)
	}

	redisContainer, err := testcontainers.Run(
		ctx,
		"redis:7-alpine",
		testcontainers.WithExposedPorts("6379/tcp"),
		testcontainers.WithCmd("redis-server", "--appendonly", "no"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("6379/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		_ = db.Close()
		_ = testcontainers.TerminateContainer(mysqlContainer)
		return nil, fmt.Errorf("start redis container: %w", err)
	}

	redisAddr, err := redisContainer.Endpoint(ctx, "")
	if err != nil {
		_ = db.Close()
		_ = testcontainers.TerminateContainer(redisContainer)
		_ = testcontainers.TerminateContainer(mysqlContainer)
		return nil, fmt.Errorf("redis endpoint: %w", err)
	}

	redisPassword := ""
	redisDB := 0
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		_ = db.Close()
		_ = testcontainers.TerminateContainer(redisContainer)
		_ = testcontainers.TerminateContainer(mysqlContainer)
		return nil, fmt.Errorf("connect redis: %w", err)
	}

	return &integrationEnv{
		mysqlDSN:       mysqlDSN,
		redisAddr:      redisAddr,
		redisPassword:  redisPassword,
		redisDB:        redisDB,
		db:             db,
		redis:          rdb,
		mysqlContainer: mysqlContainer,
		redisContainer: redisContainer,
	}, nil
}

func (e *integrationEnv) resetState(ctx context.Context) error {
	if _, err := e.db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS=0`); err != nil {
		return err
	}
	truncateOrder := []string{
		"team_invites",
		"task_comments",
		"task_history",
		"tasks",
		"team_members",
		"teams",
		"users",
	}
	for _, table := range truncateOrder {
		if _, err := e.db.ExecContext(ctx, `TRUNCATE TABLE `+table); err != nil {
			return err
		}
	}
	if _, err := e.db.ExecContext(ctx, `SET FOREIGN_KEY_CHECKS=1`); err != nil {
		return err
	}
	return e.redis.FlushDB(ctx).Err()
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot resolve test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func (e *integrationEnv) newAPIServer(t *testing.T, redisAddr string, rateLimit int) *httptest.Server {
	t.Helper()

	db, err := mysqlrepo.Connect(e.mysqlDSN)
	if err != nil {
		t.Fatalf("connect app mysql: %v", err)
	}
	cache := redisrepo.New(redisAddr, e.redisPassword, e.redisDB, 3*time.Minute)

	usersRepo := mysqlrepo.NewUsersRepo(db.DB)
	teamsRepo := mysqlrepo.NewTeamsRepo(db.DB)
	tasksRepo := mysqlrepo.NewTasksRepo(db.DB)
	invRepo := mysqlrepo.NewInvitesRepo(db.DB)
	reportsRepo := mysqlrepo.NewReportsRepo(db.DB)

	authSvc := service.NewAuthService(usersRepo)
	teamsSvc := service.NewTeamsService(teamsRepo)
	tasksSvc := service.NewTasksService(teamsRepo, tasksRepo, cache)
	invSvc := service.NewInvitesService(teamsRepo, usersRepo, invRepo, time.Hour)

	tokenAuth := jwtauth.New("HS256", []byte(testJWTSecret), nil)

	authH := handler.NewAuthHandler(authSvc, tokenAuth)
	teamsH := handler.NewTeamsHandler(teamsSvc, invSvc)
	tasksH := handler.NewTasksHandler(tasksSvc)
	reportsH := handler.NewReportsHandler(reportsRepo)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(20 * time.Second))
	r.Use(hmw.RateLimit(rateLimit))

	r.Get("/health", handler.Health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login)

		r.Group(func(r chi.Router) {
			r.Use(hmw.AuthRequired(tokenAuth))
			r.Post("/teams", teamsH.Create)
			r.Get("/teams", teamsH.List)
			r.Post("/teams/{id}/invite", teamsH.Invite)
			r.Post("/teams/{id}/accept", teamsH.AcceptInvite)
			r.Post("/tasks", tasksH.Create)
			r.Get("/tasks", tasksH.List)
			r.Put("/tasks/{id}", tasksH.Update)
			r.Get("/tasks/{id}/history", tasksH.History)
			r.Get("/reports/team-summaries", reportsH.TeamSummaries)
			r.Get("/reports/top-creators", reportsH.TopCreators)
			r.Get("/reports/invalid-assignees", reportsH.InvalidAssignees)
		})
	})

	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		srv.Close()
		_ = cache.Close()
		_ = db.Close()
	})
	return srv
}

func registerUser(t *testing.T, srv *httptest.Server, email, password string) uint64 {
	t.Helper()
	code, body := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/v1/register", "", map[string]any{
		"email":    email,
		"password": password,
	})
	if code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", code, string(body))
	}
	resp := decodeMap(t, body)
	id, ok := asUint64(resp["user_id"])
	if !ok || id == 0 {
		t.Fatalf("register response missing user_id: %v", resp)
	}
	return id
}

func loginUser(t *testing.T, srv *httptest.Server, email, password string) string {
	t.Helper()
	code, body := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/v1/login", "", map[string]any{
		"email":    email,
		"password": password,
	})
	if code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", code, string(body))
	}
	resp := decodeMap(t, body)
	token, _ := resp["token"].(string)
	if token == "" {
		t.Fatalf("login response missing token: %v", resp)
	}
	return token
}

func createTeam(t *testing.T, srv *httptest.Server, token, name string) uint64 {
	t.Helper()
	code, body := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/v1/teams", token, map[string]any{
		"name": name,
	})
	if code != http.StatusCreated {
		t.Fatalf("create team status = %d, body = %s", code, string(body))
	}
	resp := decodeMap(t, body)
	id, ok := asUint64(resp["team_id"])
	if !ok || id == 0 {
		t.Fatalf("create team response missing team_id: %v", resp)
	}
	return id
}

func createTask(t *testing.T, srv *httptest.Server, token string, req map[string]any) uint64 {
	t.Helper()
	code, body := doJSON(t, srv.Client(), http.MethodPost, srv.URL+"/api/v1/tasks", token, req)
	if code != http.StatusCreated {
		t.Fatalf("create task status = %d, body = %s", code, string(body))
	}
	resp := decodeMap(t, body)
	id, ok := asUint64(resp["task_id"])
	if !ok || id == 0 {
		t.Fatalf("create task response missing task_id: %v", resp)
	}
	return id
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body any) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return resp.StatusCode, b
}

func decodeMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode json: %v (body=%s)", err, string(b))
	}
	return out
}

func decodeTasks(t *testing.T, b []byte) []domain.Task {
	t.Helper()
	var out []domain.Task
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode tasks: %v (body=%s)", err, string(b))
	}
	return out
}

func decodeTask(t *testing.T, b []byte) domain.Task {
	t.Helper()
	var out domain.Task
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode task: %v (body=%s)", err, string(b))
	}
	return out
}

func decodeHistory(t *testing.T, b []byte) []domain.TaskHistory {
	t.Helper()
	var out []domain.TaskHistory
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode history: %v (body=%s)", err, string(b))
	}
	return out
}

func asUint64(v any) (uint64, bool) {
	f, ok := v.(float64)
	if !ok || f < 0 {
		return 0, false
	}
	return uint64(f), true
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
