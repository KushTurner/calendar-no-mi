# calendar-no-mi

Natural-language calendar event creation. Converts free-text input to Google Calendar events via HTTP endpoint and Discord bot.

## Quick Commands

```bash
go build ./...          # Build all packages
go test ./...           # Run all tests
go test ./... -v        # Verbose test output
go vet ./...            # Static analysis
go run cmd/server/main.go  # Run the server
bruno run --env local      # Run Bruno API collection against local server
```

## Architecture

```
cmd/server/main.go       # Entrypoint
internal/
  handler/               # HTTP and Discord handlers
  service/               # EventService — core orchestration
  llm/                   # LLMProvider interface + Claude/OpenAI impls
  calendar/              # CalendarClient interface + Google impl
  auth/                  # Discord allowlist
  config/                # Env var loading (godotenv)
```

## Key Conventions

- All external integrations behind interfaces (`LLMProvider`, `CalendarClient`) for testability
- LLM returns JSON only — either `event` or `clarification` object
- Conflict detection before any write; never silent overwrite
- Session state is in-memory map with 10-minute TTL
- Discord: `!cal` prefix trigger
- HTTP: bearer token auth

## Environment

See `.env.example` for required vars.
