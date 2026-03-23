package app

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/example/em_jgo/internal/config"
	"github.com/example/em_jgo/internal/repository/postgres"
	"github.com/example/em_jgo/internal/service"
	httpapi "github.com/example/em_jgo/internal/transport/http"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type App struct {
	server *http.Server
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.Postgres.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := runMigrations(ctx, pool, logger); err != nil {
		pool.Close()
		return nil, err
	}

	repository := postgres.New(pool, logger)
	serviceLayer := service.New(repository, logger)
	handler := httpapi.NewHandler(serviceLayer, logger)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Recoverer)
	router.Use(accessLogMiddleware(logger))
	httpapi.HandlerFromMux(handler, router)
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.Get("/swagger/doc.yaml", func(w http.ResponseWriter, r *http.Request) {
		spec, err := httpapi.GetSwagger()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
	router.Get("/swagger/*", httpSwagger.Handler(httpSwagger.URL("/swagger/doc.yaml")))

	loader := openapi3.NewLoader()
	if _, err := loader.LoadFromData(mustSwagger()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	return &App{server: server, pool: pool, logger: logger}, nil
}

func (a *App) Run() error {
	const methodCtx = "app.Run"
	a.logger.With("method", methodCtx).Info("starting http server", "addr", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	const methodCtx = "app.Shutdown"
	log := a.logger.With("method", methodCtx)
	log.InfoContext(ctx, "shutting down application")
	defer a.pool.Close()
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}
	return nil
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	const methodCtx = "app.runMigrations"
	log := logger.With("method", methodCtx)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		log.ErrorContext(ctx, "migrations failed", "error", err)
		return fmt.Errorf("run migrations: %w", err)
	}
	log.InfoContext(ctx, "migrations applied")
	return nil
}

func mustSwagger() []byte {
	spec, err := httpapi.GetSwagger()
	if err != nil {
		panic(err)
	}
	return spec
}

func accessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			const methodCtx = "http.middleware.accessLog"
			log := logger.With("method", methodCtx)
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.InfoContext(r.Context(), "request completed", "http_method", r.Method, "path", r.URL.Path, "status", ww.Status(), "duration", time.Since(start).String(), "request_id", chimiddleware.GetReqID(r.Context()))
		})
	}
}
