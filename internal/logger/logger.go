package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

type levelRangeHandler struct {
	min     slog.Level
	max     slog.Level
	handler slog.Handler
}

func (h *levelRangeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.min && level <= h.max && h.handler.Enabled(ctx, level)
}

func (h *levelRangeHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h *levelRangeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelRangeHandler{min: h.min, max: h.max, handler: h.handler.WithAttrs(attrs)}
}

func (h *levelRangeHandler) WithGroup(name string) slog.Handler {
	return &levelRangeHandler{min: h.min, max: h.max, handler: h.handler.WithGroup(name)}
}

type teeHandler struct {
	handlers []slog.Handler
}

func (h *teeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

func (h *teeHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record.Clone()); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *teeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithAttrs(attrs))
	}

	return &teeHandler{handlers: handlers}
}

func (h *teeHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler.WithGroup(name))
	}

	return &teeHandler{handlers: handlers}
}

type Config struct {
	Level     string
	Format    string
	OutputDir string
	AddSource bool
	Service   string
}

func New(cfg Config) (*slog.Logger, func() error, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}

	stdoutFile, err := os.OpenFile(filepath.Join(cfg.OutputDir, "stdout.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open stdout log: %w", err)
	}

	stderrFile, err := os.OpenFile(filepath.Join(cfg.OutputDir, "stderr.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_ = stdoutFile.Close()
		return nil, nil, fmt.Errorf("open stderr log: %w", err)
	}

	level := parseLevel(cfg.Level)
	stdoutHandler := newHandler(io.MultiWriter(os.Stdout, stdoutFile), cfg, &slog.HandlerOptions{Level: level, AddSource: cfg.AddSource})
	stderrHandler := newHandler(io.MultiWriter(os.Stderr, stderrFile), cfg, &slog.HandlerOptions{Level: slog.LevelError, AddSource: cfg.AddSource})

	log := slog.New((&teeHandler{handlers: []slog.Handler{
		&levelRangeHandler{min: level, max: slog.LevelWarn, handler: stdoutHandler},
		&levelRangeHandler{min: slog.LevelError, max: slog.Level(100), handler: stderrHandler},
	}}).WithAttrs([]slog.Attr{slog.String("service", cfg.Service)}))

	closeFn := func() error {
		var closeErr error
		if err := stdoutFile.Close(); err != nil {
			closeErr = err
		}
		if err := stderrFile.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
		return closeErr
	}

	return log, closeFn, nil
}

func newHandler(writer io.Writer, cfg Config, options *slog.HandlerOptions) slog.Handler {
	if cfg.Format == "text" {
		return slog.NewTextHandler(writer, options)
	}

	return slog.NewJSONHandler(writer, options)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
