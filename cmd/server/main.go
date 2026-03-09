package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
		log.Fatalf("config: %v", err)
	}

	calFactory, err := calendar.NewGoogleCalendarClientFactory(cfg.GoogleCredentialsFile)
	if err != nil {
		log.Fatalf("calendar factory: %v", err)
	}

	llmProvider, err := llm.NewFromConfig(cfg)
	if err != nil {
		log.Fatalf("llm provider: %v", err)
	}

	svc := service.NewEventService(llmProvider, calFactory)
	h := handler.NewHandler(cfg, svc)

	r := chi.NewRouter()
	r.Use(middleware.Heartbeat("/health"))
	r.Use(middleware.Logger)
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
		log.Printf("listening on http://localhost:%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
