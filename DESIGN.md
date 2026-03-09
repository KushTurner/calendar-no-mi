# calendar-no-mi — Design Document

> **Status:** v1 implemented
> **Last updated:** 2026-03-09

---

## 1. Problem Statement

Creating calendar events is friction-heavy: open app → tap fields → pick time → save. The goal of **calendar-no-mi** is to let a user say (or type) something like:

> "Lunch with Sarah next Tuesday at noon for an hour at Nobu"

…and have a Google Calendar event created automatically — no form, no tapping.

**v1 scope (implemented):**
- Accept natural-language text via an HTTP endpoint (Bruno collection in `bruno/`)
- Parse event details with an LLM (OpenAI)
- Write to Google Calendar — single user, no conflict detection, no clarification loop
- Return the created event URL

**Deferred to v2+:**
- Conflict detection and `force` override
- Clarification flow (multi-turn LLM)
- Discord text bot
- Multi-user OAuth onboarding

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Inputs                                  │
│                                                                 │
│  ┌──────────────┐                                               │
│  │  HTTP Client │                                               │
│  │  (Bruno /    │                                               │
│  │   cURL / app)│                                               │
│  └──────┬───────┘                                               │
│         │  POST /event                                          │
└─────────┼─────────────────────────────────────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────────────────┐
│                     internal/handler                            │
│                                                                 │
│   HTTPHandler                                                   │
│   - bearer token auth                                           │
│   - JSON decode                                                 │
│   - build User from config                                      │
└──────────────────────────┬──────────────────────────────────────┘
                           │ (user, text)
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    internal/service                             │
│                                                                 │
│   EventService                                                  │
│   - parse text via LLM                                          │
│   - map ParsedEvent → CalendarEvent                             │
│   - create event via CalendarClient                             │
│   - return EventURL                                             │
└───────────────┬───────────────────────────┬─────────────────────┘
                │                           │
                ▼                           ▼
┌──────────────────────┐     ┌──────────────────────────────────┐
│  internal/llm        │     │  internal/calendar               │
│                      │     │                                  │
│  LLMProvider (iface) │     │  CalendarClient (iface)          │
│  └─ openAIProvider   │     │  └─ GoogleCalendarClient         │
│                      │     │     - per-user OAuth2            │
│  → returns ParsedEvent│     │     - CreateEvent only (v1)     │
└──────────────────────┘     └──────────────────────────────────┘
```

**Request flow (v1 — happy path only):**

1. `POST /event` arrives at handler — bearer token validated
2. Handler builds `models.User` from config (single user)
3. Handler calls `EventService.CreateFromText(ctx, user, text)`
4. `EventService` calls `LLMProvider.ParseEvent` → `ParsedEvent`
5. `EventService` maps `ParsedEvent` → `models.CalendarEvent`
6. `EventService` calls `CalendarClient.CreateEvent` → event URL
7. Handler returns `201 {"event_url": "..."}`

---

## 3. Go Project Structure

```
calendar-no-mi/
├── cmd/
│   └── server/
│       └── main.go               # wires everything together, starts HTTP server
├── internal/
│   ├── handler/
│   │   ├── http.go               # HTTP handler (chi)
│   │   └── http_test.go
│   ├── service/
│   │   ├── event.go              # EventService (core orchestration)
│   │   └── event_test.go
│   ├── llm/
│   │   ├── provider.go           # LLMProvider interface
│   │   ├── parsed_event.go       # ParsedEvent struct
│   │   ├── openai.go             # OpenAI implementation
│   │   ├── prompt.go             # system prompt construction
│   │   ├── parse.go              # JSON response parser
│   │   ├── factory.go            # NewFromConfig
│   │   ├── prompt_test.go
│   │   └── parse_test.go
│   ├── calendar/
│   │   ├── client.go             # CalendarClient interface
│   │   ├── factory.go            # GoogleCalendarClientFactory
│   │   ├── google.go             # Google Calendar API implementation
│   │   └── google_test.go
│   ├── models/
│   │   └── types.go              # User, CalendarEvent, Result
│   └── config/
│       ├── config.go             # env var loading + validation
│       └── config_test.go
├── .env.example
├── DESIGN.md
├── go.mod
└── go.sum
```

**Conventions:**
- All packages under `internal/` are unexported to the outside world
- Interfaces defined alongside the service that *uses* them (not the implementation)
- No global state; dependencies injected via constructor functions

---

## 4. Component Breakdown

### 4.1 EventService (`internal/service`)

Central orchestrator. Stateless.

```go
type EventService struct {
    llm  llm.LLMProvider
    cal  calendar.CalendarClientFactory
    now  func() time.Time
}

func (s *EventService) CreateFromText(ctx context.Context, user models.User, text string) (models.Result, error)
```

### 4.2 LLMProvider (`internal/llm`)

```go
type LLMProvider interface {
    ParseEvent(ctx context.Context, userText string, now time.Time, timezone string) (*ParsedEvent, error)
}

type ParsedEvent struct {
    Title       string
    Start       time.Time  // RFC3339 with offset; preserves tz info
    End         time.Time  // always set for timed events
    Description string
    Location    string
    AllDay      bool
    StartDate   string     // YYYY-MM-DD, only when AllDay=true
    EndDate     string     // YYYY-MM-DD, only when AllDay=true
}
```

System prompt injects the current datetime and user timezone. LLM returns JSON only (no prose). Default end time: 1 hour after start if not specified.

### 4.3 CalendarClient (`internal/calendar`)

```go
type CalendarClient interface {
    ListEvents(ctx context.Context, start, end time.Time) ([]models.CalendarEvent, error)
    CreateEvent(ctx context.Context, event models.CalendarEvent) (string, error)
    UpdateEvent(ctx context.Context, eventID string, event models.CalendarEvent) error
    GetEvent(ctx context.Context, eventID string) (models.CalendarEvent, error)
}

type CalendarClientFactory interface {
    ForUser(ctx context.Context, user models.User) (CalendarClient, error)
}
```

v1 only calls `CreateEvent`. The rest of the interface is implemented but unused — available for v2 conflict detection and update flows.

### 4.4 HTTP Handler (`internal/handler/http.go`)

Thin adapter: validate bearer token → decode JSON → build User from config → call `EventService` → encode response. All error responses use `Content-Type: application/json`. Bearer token comparison is timing-safe (`crypto/subtle`).

---

## 5. Data Models

```go
// User holds per-user credentials. v1 is single-user; values come from config.
type User struct {
    ID           string
    RefreshToken string
    CalendarID   string
    Timezone     string
    RegisteredAt time.Time
}

// CalendarEvent is the domain representation of a calendar event.
type CalendarEvent struct {
    ID          string
    Title       string
    Start       time.Time
    End         time.Time
    Description string
    Location    string
    Attendees   []string
    AllDay      bool
}

// Result is returned by EventService.
type Result struct {
    Event    *CalendarEvent
    EventURL string
}
```

---

## 6. API

### POST /event

Create a calendar event from natural language.

**Auth:** `Authorization: Bearer <HTTP_BEARER_TOKEN>`

**Request:**
```json
{ "text": "Lunch with Sarah next Tuesday at noon for an hour at Nobu" }
```

**Response 201 — event created:**
```json
{ "event_url": "https://calendar.google.com/calendar/event?eid=..." }
```

**Response 400 — missing/empty text:**
```json
{ "error": "text is required" }
```

**Response 401 — bad token:**
```json
{ "error": "unauthorized" }
```

**Response 500 — internal error:**
```json
{ "error": "internal server error" }
```

---

### GET /health

Returns `200 OK` (chi Heartbeat middleware). No body.

---

## 7. LLM Prompt Strategy

**Design goals:** deterministic output, timezone-correct times, JSON-only response.

**Parameters:** `temperature=0`, `max_tokens=512`, model: `gpt-4o-mini`

The system prompt injects the current datetime (RFC3339 with offset) and the user's IANA timezone name. The LLM must always return this exact JSON shape:

```json
{
  "title": "Lunch with Sarah",
  "start": "2026-03-17T12:00:00+00:00",
  "end":   "2026-03-17T13:00:00+00:00",
  "description": "",
  "location": "Nobu",
  "all_day": false
}
```

Rules baked into the prompt:
- `start`/`end` are RFC3339 with timezone offset matching the user's timezone
- `end` is always explicit; if not mentioned, default to 1 hour after start
- For all-day events: `all_day=true`, `start`/`end` are `YYYY-MM-DD` strings
- Return JSON only — no prose, no markdown fences

---

## 8. Google Calendar Integration

### OAuth 2.0 (v1 — single user)

v1 uses a pre-obtained refresh token stored in `GOOGLE_REFRESH_TOKEN`. No OAuth flow or callback endpoint is needed.

`CalendarClientFactory.ForUser` builds a per-request `oauth2.TokenSource` that auto-refreshes expired access tokens using the stored refresh token.

**Scope:** `https://www.googleapis.com/auth/calendar.events`

**Credentials:** `credentials.json` from Google Cloud Console.

### Apple Calendar sync

Google Calendar natively syncs to Apple Calendar on iPhone via CalDAV when the Google account is added in iOS Settings → Calendar → Accounts. No additional work needed.

---

## 9. Configuration / Env Vars

`.env.example`:

```dotenv
HTTP_PORT=8080
GOOGLE_CREDENTIALS_FILE=credentials.json
GOOGLE_REFRESH_TOKEN=          # required — pre-obtained OAuth refresh token
GOOGLE_CALENDAR_ID=primary
OPENAI_API_KEY=                # required
HTTP_BEARER_TOKEN=             # required — secret for POST /event
DEFAULT_TIMEZONE=Europe/London # IANA timezone name
```

Required fields (`HTTP_BEARER_TOKEN`, `GOOGLE_REFRESH_TOKEN`) are validated at startup; the server will not start if they are missing.

---

## 10. Edge Cases

| # | Input / Scenario | Expected Behavior |
|---|---|---|
| 1 | No date mentioned ("lunch with Bob") | LLM returns ambiguous result — currently surfaces as a parse error |
| 2 | Relative date ("next Tuesday") | Injected current date+time in prompt resolves this |
| 3 | No time mentioned ("meeting tomorrow") | LLM defaults to a reasonable time (prompt asks it to try) |
| 4 | No duration mentioned ("call at 3pm") | Defaults to 1 hour |
| 5 | All-day event ("dentist appointment Friday") | `all_day=true` |
| 6 | Multi-day event ("vacation July 4–7") | Start/end spanning multiple days |
| 7 | LLM returns invalid JSON | Parse error → 500 |
| 8 | LLM timeout / rate limit | Surfaces as 500 |
| 9 | Google Calendar API down | 500 "internal server error" |
| 10 | Expired Google OAuth token | `oauth2.TokenSource` auto-refreshes |
| 11 | Empty text field | 400 "text is required" |
| 12 | Timezone not set | Falls back to `DEFAULT_TIMEZONE` (Europe/London) |

---

## 11. Phased Rollout

### Phase 1 — HTTP MVP (implemented)
- `EventService` + `LLMProvider` (OpenAI) + `GoogleCalendarClient`
- `POST /event`: text in → event created → URL out
- `GET /health`
- Single user via env vars

### Phase 2 — Conflict Detection + Clarification
- `EventService` checks `CalendarClient.ListEvents` before creating
- LLM can return a `clarification` object for ambiguous input
- HTTP handler handles multi-branch `Result` (conflict / clarification / created)

### Phase 3 — Discord Text Bot
- Add `DiscordHandler`
- `!cal` prefix trigger
- Stateless — user re-sends `!cal` with refined text if needed

### Phase 4 — Voice Input (Discord + Whisper)
- Discord voice channel listener → speech-to-text → existing text pipeline

### Phase 5 — Hardening
- Replace single-user env var config with multi-user OAuth + user store
- Rate limiting, structured logging, observability

---

## 12. Future Considerations

| Feature | Notes |
|---|---|
| Multi-user support | Per-user OAuth flow + `users.json` / SQLite user store |
| Discord bot | Phase 3; reuses existing `EventService` unchanged |
| SMS via Twilio | Twilio webhook → HTTP adapter → existing `EventService` |
| Event update / delete | `UpdateEvent`/`GetEvent` already implemented in `CalendarClient`; add `PATCH /event` |
| Natural-language event search | `GET /events/search?q=` + LLM → date range → `Events.List` |
| Recurring events | Requires Google Calendar `recurrence` field + LLM support |
