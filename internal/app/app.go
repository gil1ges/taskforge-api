package app

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gil1ges/taskforge-api/internal/config"
	"github.com/gil1ges/taskforge-api/internal/http/handler"
	hmw "github.com/gil1ges/taskforge-api/internal/http/middleware"
	"github.com/gil1ges/taskforge-api/internal/observability"
	"github.com/gil1ges/taskforge-api/internal/repo/mysql"
	redisrepo "github.com/gil1ges/taskforge-api/internal/repo/redis"
	"github.com/gil1ges/taskforge-api/internal/service"
)

type App struct {
	HTTP  *http.Server
	DB    *mysql.DB
	Cache *redisrepo.Cache
}

func New(cfg config.Config) (*App, error) {
	db, err := mysql.Connect(cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}

	cache := redisrepo.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.RedisTasksTTL)

	metrics := observability.NewMetrics()
	prometheus.MustRegister(metrics.RequestsTotal, metrics.RequestLatency, metrics.ErrorsTotal)

	usersRepo := mysql.NewUsersRepo(db.DB)
	teamsRepo := mysql.NewTeamsRepo(db.DB)
	tasksRepo := mysql.NewTasksRepo(db.DB)
	invRepo := mysql.NewInvitesRepo(db.DB)
	reportsRepo := mysql.NewReportsRepo(db.DB)

	authSvc := service.NewAuthService(usersRepo)
	teamsSvc := service.NewTeamsService(teamsRepo)

	notifier := service.NewCircuitBreakerNotifier(
		service.NewHTTPInviteNotifier(cfg.InviteNotifyURL, 3*time.Second),
		3,
		30*time.Second,
	)
	invSvc := service.NewInvitesServiceWithNotifier(teamsRepo, usersRepo, invRepo, notifier, cfg.InviteTTL)

	tasksSvc := service.NewTasksService(teamsRepo, tasksRepo, cache)

	tokenAuth := jwtauth.New("HS256", []byte(cfg.JWTSecret), nil)

	authH := handler.NewAuthHandler(authSvc, tokenAuth)
	teamsH := handler.NewTeamsHandler(teamsSvc, invSvc)
	tasksH := handler.NewTasksHandler(tasksSvc)
	reportsH := handler.NewReportsHandler(reportsRepo)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(hmw.Metrics(metrics))
	r.Use(hmw.RateLimit(cfg.RateLimitPerMin))

	r.Get("/health", handler.Health)
	r.Mount("/metrics", promhttp.Handler())

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

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: r,
	}

	return &App{HTTP: srv, DB: db, Cache: cache}, nil
}

func (a *App) Shutdown(ctx context.Context) error {
	_ = a.Cache.Close()
	_ = a.DB.Close()
	return a.HTTP.Shutdown(ctx)
}
