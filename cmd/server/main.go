package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kushturner/calendar-no-mi/internal/calendar"
	"github.com/kushturner/calendar-no-mi/internal/config"
	"github.com/kushturner/calendar-no-mi/internal/handler"
	"github.com/kushturner/calendar-no-mi/internal/llm"
	"github.com/kushturner/calendar-no-mi/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// Parse log level; invalid values fall back to INFO.
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		slog.Warn("invalid LOG_LEVEL, defaulting to info", "value", cfg.LogLevel)
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var logger *slog.Logger
	if cfg.AppEnv == "production" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	slog.SetDefault(logger)

	calFactory, err := calendar.NewGoogleCalendarClientFactory(cfg.GoogleCredentialsFile)
	if err != nil {
		logger.Error("calendar factory init failed", "error", err)
		os.Exit(1)
	}

	llmProvider, err := llm.NewFromConfig(cfg)
	if err != nil {
		logger.Error("llm provider init failed", "error", err)
		os.Exit(1)
	}

	svc := service.NewEventService(llmProvider, calFactory)
	h := handler.NewHandler(cfg, svc, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Heartbeat("/health"))
	r.Use(newStructuredLogger(logger))
	r.Use(middleware.Recoverer)

	r.Post("/event", h.CreateEvent)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server listening", "addr", fmt.Sprintf("http://localhost:%s", cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}
}
