package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// structuredLogFormatter implements middleware.LogFormatter backed by slog.
type structuredLogFormatter struct {
	logger *slog.Logger
}

// newStructuredLogger returns a chi middleware that logs requests via slog.
func newStructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&structuredLogFormatter{logger: logger})
}

// NewLogEntry creates a new log entry for the request.
func (f *structuredLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &structuredLogEntry{
		logger:  f.logger,
		request: r,
	}
}

// structuredLogEntry implements middleware.LogEntry.
type structuredLogEntry struct {
	logger  *slog.Logger
	request *http.Request
}

func (e *structuredLogEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	e.logger.InfoContext(
		e.request.Context(),
		"request",
		"method", e.request.Method,
		"path", e.request.URL.Path,
		"status", status,
		"bytes", bytes,
		"duration_ms", elapsed.Milliseconds(),
		"request_id", middleware.GetReqID(e.request.Context()),
	)
}

func (e *structuredLogEntry) Panic(v interface{}, stack []byte) {
	e.logger.ErrorContext(
		e.request.Context(),
		"panic",
		"error", v,
		"stack", string(stack),
		"request_id", middleware.GetReqID(e.request.Context()),
	)
}
