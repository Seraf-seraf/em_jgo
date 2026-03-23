package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/example/em_jgo/internal/app"
	"github.com/example/em_jgo/internal/config"
	pkglogger "github.com/example/em_jgo/internal/logger"
)

func main() {
	configPath := flag.String("config", "configs/config.yml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	logger, closeLogger, err := pkglogger.New(pkglogger.Config{
		Level:     cfg.Logger.Level,
		Format:    cfg.Logger.Format,
		OutputDir: cfg.Logger.OutputDir,
		AddSource: cfg.Logger.AddSource,
		Service:   cfg.Logger.Service,
	})
	if err != nil {
		slog.Error("create logger failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = closeLogger()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("initialize app failed", "error", err)
		os.Exit(1)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := application.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown failed", "error", err)
		}
	}()

	if err := application.Run(); err != nil {
		logger.Error("application stopped with error", "error", err)
		os.Exit(1)
	}
}
