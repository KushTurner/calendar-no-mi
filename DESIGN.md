# calendar-no-mi — Design Document

> **Status:** In progress (Phase 1)
> **Last updated:** 2026-03-08

---

## 1. Problem Statement

Creating calendar events is friction-heavy: open app → tap fields → pick time → save. The goal of **calendar-no-mi** is to let a user say (or type) something like:

> "Lunch with Sarah next Tuesday at noon for an hour at Nobu"

…and have a Google Calendar event created automatically — no form, no tapping.

**v1 scope:**
- Accept natural-language text via an HTTP endpoint (Postman-testable) and a Discord text bot
- Parse intent with an LLM (provider-agnostic)
- Detect conflicts and ask for clarification before writing
- Write to Google Calendar

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Inputs                                  │
│                                                                 │
│  ┌──────────────┐          ┌──────────────────────────────┐    │
│  │  HTTP Client │          │       Discord Bot            │    │
│  │  (Postman /  │          │  (!cal trigger)              │    │
│  │   cURL / app)│          └──────────────┬───────────────┘    │
│  └──────┬───────┘                         │                    │
│         │  POST /event                    │ message text        │
└─────────┼─────────────────────────────────┼────────────────────┘
          │                                 │
          ▼                                 ▼
┌─────────────────────────────────────────────────────────────────┐
│                     internal/handler                            │
│                                                                 │
│   HTTPHandler                       DiscordHandler              │
│   - JSON decode                     - user lookup               │
│   - user lookup                     - reply with DM/channel     │
└──────────────────────────┬──────────────────────────────────────┘
                           │ EventRequest
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    internal/service                             │
│                                                                 │
│   EventService (fully stateless)                                │
│   - orchestrates: parse → validate → conflict check → create/  │
│     update                                                      │
└───────────────┬───────────────────────────┬─────────────────────┘
                │                           │
                ▼                           ▼
┌──────────────────────┐     ┌──────────────────────────────────┐
│  internal/llm        │     │  internal/calendar               │
│                      │     │                                  │
│  LLMProvider (iface) │     │  CalendarClient (iface)          │
│  ├─ ClaudeProvider   │     │  └─ GoogleCalendarClient         │
│  └─ OpenAIProvider   │     │     - per-user OAuth2            │
│                      │     │     - event CRUD + list          │
│  → returns parsed    │     └──────────────────────────────────┘
│    intent + event or │
│    ClarificationNeeded│
└──────────────────────┘
```

**Request flow (happy path):**

1. Input arrives at handler → user looked up in user store (must be registered)
2. Handler calls `EventService.CreateFromText(ctx, user, rawText, force)`
3. `EventService` calls `LLMProvider.Parse(ctx, prompt)` → structured `CalendarEvent`
4. `EventService` calls `CalendarClient.ListEvents(ctx, start, end)` → checks overlaps
5. `EventService` calls `CalendarClient.CreateEvent(ctx, event)` → returns event link
6. Handler returns success + event details to caller

**Clarification flow:**

Steps 1–3 same, but LLM returns `ClarificationNeeded` with a question.
Handler sends question back to user. User replies with corrected text → handler
calls `EventService.CreateFromText` again with the refined input.

**Conflict flow (stateless):**

Steps 1–4 same, conflicts found. Service returns conflict details to handler.
Handler returns HTTP 200 with `conflict` field. Client re-submits with `"force": true`
to override.

---

## 3. Go Project Structure

```
calendar-no-mi/
├── cmd/
│   └── server/
│       └── main.go               # wires everything together, starts HTTP + Discord
├── internal/
│   ├── handler/
│   │   ├── http.go               # HTTP handler (chi or stdlib net/http)
│   │   ├── http_test.go
│   │   ├── discord.go            # discordgo event handler
│   │   └── discord_test.go
│   ├── service/
│   │   ├── event.go              # EventService (core orchestration)
│   │   └── event_test.go
│   ├── llm/
│   │   ├── provider.go           # LLMProvider interface
│   │   ├── claude.go             # Anthropic SDK implementation
│   │   ├── openai.go             # OpenAI SDK implementation
│   │   └── prompt.go             # shared prompt construction
│   ├── calendar/
│   │   ├── client.go             # CalendarClient interface
│   │   └── google.go             # Google Calendar API implementation
│   ├── auth/
│   │   ├── oauth.go              # Google OAuth flow + callback handler
│   │   └── userstore.go          # User struct + JSON file persistence
│   └── config/
│       └── config.go             # env var loading
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

Central orchestrator. Fully stateless — no in-memory session map, no mutex.

```go
type EventService struct {
    llm             llm.LLMProvider
    calendarFactory calendar.CalendarClientFactory
}

func (s *EventService) CreateFromText(ctx context.Context, user User, text string, force bool) (Result, error)
func (s *EventService) UpdateFromText(ctx context.Context, user User, text string) (Result, error)
```

`Result` is a sum type: `EventCreated` (with Google Calendar URL), `EventUpdated`, `NeedsClarification` (with question string), or `Conflict` (with conflicting event details).

### 4.2 LLMProvider (`internal/llm`)

```go
type LLMProvider interface {
    Parse(ctx context.Context, req ParseRequest) (ParseResponse, error)
}

type ParseRequest struct {
    UserText    string
    CurrentTime time.Time
    UserTZ      string     // e.g. "America/New_York"
}

type ParseResponse struct {
    Intent              Intent
    Event               *CalendarEvent       // for CREATE or full UPDATE
    TargetDescription   string               // for UPDATE/DELETE: natural-language description of target event
    UpdateFields        *CalendarEvent       // for UPDATE: only changed fields (nil = full replace)
    ClarificationNeeded *ClarificationNeeded // nil if intent is clear
}
```

Provider selected at startup via `LLM_PROVIDER` env var. Both implementations share the same prompt template from `prompt.go`.

### 4.3 CalendarClient (`internal/calendar`)

```go
type CalendarClient interface {
    ListEvents(ctx context.Context, start, end time.Time) ([]CalendarEvent, error) // used for conflict detection and event search
    CreateEvent(ctx context.Context, event CalendarEvent) (string, error)           // returns event URL
    UpdateEvent(ctx context.Context, eventID string, event CalendarEvent) error
    GetEvent(ctx context.Context, eventID string) (CalendarEvent, error)
}

// CalendarClientFactory creates a per-user client from a stored refresh token
type CalendarClientFactory interface {
    ForUser(ctx context.Context, user User) (CalendarClient, error)
}
```

`EventService` calls `ListEvents` directly to check for overlapping events — conflict logic lives in the service layer, not the calendar client.

### 4.4 HTTP Handler (`internal/handler/http.go`)

Thin adapter: decode JSON → look up user in user store → call `EventService` → encode response. Access control is deployment-level (non-public URL); the handler does not implement its own auth token check.

### 4.5 Discord Handler (`internal/handler/discord.go`)

Listens for `messageCreate` events. Only acts on messages starting with `!cal `. Looks up the sender's Discord user ID in the user store. If not registered, prompts them to run `!cal connect` to link their Google Calendar.

**Onboarding command — `!cal connect`:** Initiates the Google OAuth flow for the sender. Bot replies with a unique authorization URL. After the user authorizes, the OAuth callback stores their refresh token and links their calendar.

### 4.6 Auth (`internal/auth`)

**Closed-deployment model:** Access is controlled at the infrastructure level — the Discord bot lives in a private server, and the HTTP endpoint is not publicly exposed. No per-user allowlist in application code.

**Per-user Google OAuth onboarding:**
- Any user in the closed deployment can self-register by initiating the OAuth flow (`!cal connect` / `POST /auth/connect`)
- OAuth callback endpoint (`GET /auth/callback`) handles both Discord and HTTP users
- On successful authorization, user record is written to the user store

**User store (`internal/auth/userstore.go`):**
- Maps userID → `{googleRefreshToken, calendarID, timezone, registeredAt}`
- v1 persistence: `users.json` (gitignored)
- v2: replace with SQLite or Postgres without touching callers

---

## 5. Data Models

```go
type Intent string

const (
    IntentCreate Intent = "CREATE"
    IntentUpdate Intent = "UPDATE"
    IntentDelete Intent = "DELETE"
)

// EventRequest is the raw input from any handler
type EventRequest struct {
    Text   string `json:"text"`
    Force  bool   `json:"force,omitempty"`   // if true, create even if conflicts exist
    UserID string `json:"user_id,omitempty"` // populated by handler, not caller
}

// User represents a registered user with linked Google Calendar
type User struct {
    ID           string    `json:"id"`            // Discord user ID or HTTP client ID
    RefreshToken string    `json:"refresh_token"`
    CalendarID   string    `json:"calendar_id"`
    Timezone     string    `json:"timezone"`
    RegisteredAt time.Time `json:"registered_at"`
}

// CalendarEvent is the parsed, structured event ready for Google Calendar
type CalendarEvent struct {
    Title       string        `json:"title"`
    Start       time.Time     `json:"start"`
    End         time.Time     `json:"end"`
    Description string        `json:"description,omitempty"`
    Location    string        `json:"location,omitempty"`
    Attendees   []string      `json:"attendees,omitempty"` // email addresses
    AllDay      bool          `json:"all_day,omitempty"`
}

// ClarificationNeeded is returned by the LLM when more info is required
type ClarificationNeeded struct {
    Question string `json:"question"` // asked back to the user
}

// ConflictInfo is returned when existing events overlap with the requested time
type ConflictInfo struct {
    Events  []CalendarEvent `json:"events"`
    Message string          `json:"message"` // human-readable description + hint to use force:true
}

// Result wraps the possible outcomes of an EventService call
type Result struct {
    Event               *CalendarEvent       `json:"event,omitempty"`
    EventURL            string               `json:"event_url,omitempty"`
    ClarificationNeeded *ClarificationNeeded `json:"clarification,omitempty"`
    Conflict            *ConflictInfo        `json:"conflict,omitempty"`
}
```

---

## 6. API Contracts

### POST /event

Create a calendar event from natural language.

**Request:**
```json
{
  "text": "Lunch with Sarah next Tuesday at noon for an hour at Nobu",
  "force": false
}
```

`"force"` is optional (default `false`). When `true` and conflicts exist, the event is created anyway.

**Response 201 — event created:**
```json
{
  "event": {
    "title": "Lunch with Sarah",
    "start": "2026-03-17T12:00:00-05:00",
    "end":   "2026-03-17T13:00:00-05:00",
    "location": "Nobu"
  },
  "event_url": "https://calendar.google.com/calendar/event?eid=..."
}
```

**Response 200 — clarification needed:**
```json
{
  "clarification": {
    "question": "Did you mean next Tuesday March 17th or the following Tuesday March 24th?"
  }
}
```

Re-submit `POST /event` with a more specific `text` to resolve.

**Response 200 — conflict detected:**
```json
{
  "conflict": {
    "events": [
      { "title": "Team standup", "start": "2026-03-17T12:00:00-05:00", "end": "2026-03-17T12:30:00-05:00" }
    ],
    "message": "You have 'Team standup' from 12:00–12:30 on Tuesday. Resubmit with \"force\": true to schedule anyway."
  }
}
```

---

### PATCH /event

Update an existing calendar event using natural language.

**Request:**
```json
{
  "text": "Move my lunch with Sarah to 2pm"
}
```

The service searches the user's calendar for a matching event. If exactly one match is found, the update is applied. If multiple matches are found, a `clarification` is returned asking the user to be more specific.

**Response:** same shape as `POST /event` (can return `event`, `clarification`, or `conflict`).

---

### POST /auth/connect

Initiate the Google OAuth flow for the requesting user.

**Response:**
```json
{
  "auth_url": "https://accounts.google.com/o/oauth2/auth?..."
}
```

User visits the URL, authorizes, and is redirected to `GET /auth/callback`.

---

### GET /auth/callback

Google redirects here after the user authorizes. Exchanges the authorization code for a refresh token, writes the user record to the user store, and returns a success page.

---

### GET /health

```json
{ "status": "ok", "time": "2026-03-08T10:00:00Z" }
```

---

## 7. LLM Prompt Strategy

**Design goals:** deterministic output, no hallucinated dates, JSON-only response.

### System prompt skeleton

```
You are a calendar assistant. Your only job is to extract structured event
information from natural-language text.

Rules:
- Always respond with valid JSON. No prose, no markdown fences.
- Determine the intent: CREATE, UPDATE, or DELETE.
- For CREATE: if the input has enough info, return an "event" object. If
  critical information is missing or ambiguous (date, time, duration), return
  a "clarification" object with a single clarifying question.
- For UPDATE: return a "target_description" (natural-language description of
  the event to find) and an "update_fields" object with only the changed fields.
- For DELETE: return a "target_description" only.
- Never guess an ambiguous date — ask instead.
- Duration default: 1 hour if not specified.
- All-day events: set all_day=true and omit time fields.
- Timezone: use the provided user timezone for all times.
- Output times in RFC3339 format.

Current date and time: {{.CurrentTime}}  ({{.UserTZ}})

Response schema:
{
  "intent": "CREATE" | "UPDATE" | "DELETE",
  "event": { ... } | null,
  "target_description": string | null,
  "update_fields": { ... } | null,
  "clarification": { "question": string } | null
}
```

**Parameters:** `temperature=0`, `max_tokens=512`

**Provider abstraction:** `prompt.go` builds the final string. Each provider's `Parse()` wraps it in provider-specific message format (Claude `messages` API vs OpenAI `chat/completions`).

---

## 8. Google Calendar Integration

### OAuth 2.0 flow (per-user)

Each user goes through their own OAuth flow:

1. User sends `!cal connect` (Discord) or `POST /auth/connect` (HTTP)
2. Server generates a Google OAuth consent URL and returns it to the user
3. User visits the URL and authorizes
4. Google redirects to `GET /auth/callback` with an authorization code
5. Server exchanges the code for a refresh token and writes a `User` record to the user store
6. Subsequent requests: `CalendarClientFactory.ForUser(ctx, user)` builds a per-user client using the stored refresh token; the underlying token source auto-refreshes as needed

**Scopes required:** `https://www.googleapis.com/auth/calendar.events`

**Credentials:** `credentials.json` downloaded from Google Cloud Console (gitignored, shared across the deployment).

**User store:** `users.json` (gitignored). Maps userID → `User` record including refresh token, calendar ID, and timezone.

### Conflict detection

`EventService` calls `CalendarClient.ListEvents(ctx, start, end)` to fetch events overlapping the requested window, then checks for overlaps itself. If conflicts are found and `force` is false, it returns a `Conflict` result. If `force` is true, it proceeds to create the event.

### Apple Calendar sync

Google Calendar natively syncs to Apple Calendar on iPhone via CalDAV when the Google account is added in iOS Settings → Calendar → Accounts. No additional work needed.

---

## 9. Input Channels

### Discord Bot

**Trigger prefix:** `!cal ` (case-insensitive strip before sending to service)

**Bot permissions needed:** `Read Messages`, `Send Messages`, `Read Message History`

**Access control:** The bot is added only to a private Discord server. No allowlist in code — membership in the server is the gate.

**Onboarding:** `!cal connect` — bot replies with a Google OAuth URL. After the user authorizes, they can start creating events immediately.

**Session handling:** Stateless. If clarification is needed, the bot replies with the question. The user sends a follow-up `!cal` command with the refined text.

**Response format:**

```
✅ Event created: Lunch with Sarah
   📅 Tuesday March 17, 12:00–1:00 PM EST
   📍 Nobu
   🔗 https://calendar.google.com/...
```

or

```
❓ Did you mean next Tuesday March 17th or the following Tuesday March 24th?
   Reply with: !cal lunch with Sarah on March 17th at noon
```

### HTTP

Thin REST API, not publicly exposed. Intended for Postman testing and potential future front-end use.

---

## 10. Configuration / Env Vars

`.env.example`:

```dotenv
# LLM
LLM_PROVIDER=claude                  # "claude" or "openai"
ANTHROPIC_API_KEY=sk-ant-...
OPENAI_API_KEY=sk-...

# Google Calendar
GOOGLE_CREDENTIALS_FILE=credentials.json
GOOGLE_CALENDAR_ID=primary           # or specific calendar ID

# User store (v1 JSON file persistence)
USER_STORE_FILE=users.json

# OAuth callback
OAUTH_CALLBACK_URL=http://localhost:8080/auth/callback

# HTTP server
HTTP_PORT=8080

# Discord
DISCORD_BOT_TOKEN=...

# Timezone (used when user TZ is unknown)
DEFAULT_TIMEZONE=America/New_York
```

All fields validated at startup; missing required fields cause a fatal error with a clear message.

---

## 11. Edge Cases

| # | Input / Scenario | Expected Behavior |
|---|---|---|
| 1 | No date mentioned ("lunch with Bob") | Ask: "When would you like to schedule this?" |
| 2 | Relative date ("next Tuesday") near week boundary | Inject current date+time in prompt; LLM resolves unambiguously |
| 3 | Ambiguous "next Tuesday" (could be +7 or +14 days) | Ask user to confirm specific date |
| 4 | No time mentioned ("meeting tomorrow") | Ask: "What time?" |
| 5 | No duration mentioned ("call at 3pm") | Default to 1 hour |
| 6 | All-day event ("dentist appointment Friday") | Set `all_day=true` |
| 7 | Multi-day event ("vacation July 4–7") | Set start/end spanning multiple days |
| 8 | Past date ("meeting yesterday") | Ask: "Did you mean to add a past event, or a future date?" |
| 9 | Conflict with existing event | Return `conflict` with details; client re-submits with `force: true` to override |
| 10 | Partial conflict (overlap, not exact) | Same as #9 |
| 11 | Multiple events in one message ("lunch Mon, dinner Tue") | v1: handle only first event; tell user to submit separately |
| 12 | Recurring event ("every Monday at 9am") | v1: tell user recurring events are not yet supported |
| 13 | Very long description input (>2000 chars) | Truncate to 2000 chars before sending to LLM |
| 14 | LLM returns invalid JSON | Retry once; if still invalid, return generic error |
| 15 | LLM timeout | Return 504 to HTTP caller; Discord: "Sorry, timed out. Try again." |
| 16 | LLM rate limit | Exponential backoff ×2 (max 3 attempts); then surface error |
| 17 | Google Calendar API down | Return 503 with "Calendar service unavailable" |
| 18 | Expired Google OAuth token | `oauth2.TokenSource` auto-refreshes; if refresh fails, log and return 500 |
| 19 | Attendee email in text ("invite john@example.com") | Extract email and populate `attendees` field |
| 20 | No attendees mentioned | `attendees` field omitted |
| 21 | Discord user not yet registered (`!cal connect` not done) | Bot replies: "You haven't linked your Google Calendar yet. Run `!cal connect` to get started." |
| 22 | Discord bot loses connection | Auto-reconnect; no session state to recover |
| 23 | UPDATE text matches multiple calendar events | Return `clarification` asking user to be more specific |
| 24 | UPDATE text matches no calendar events | Return `clarification` explaining no match was found |
| 25 | HTTP user not registered | 403 `{"error": "user not registered — visit /auth/connect"}` |
| 26 | OAuth callback receives invalid/expired code | 400 with error; user must restart the connect flow |
| 27 | Empty text field | 400 `{"error": "text is required"}` |
| 28 | Timezone not determinable | Fall back to `DEFAULT_TIMEZONE` env var |

---

## 12. Phased Rollout

### Phase 1 — HTTP MVP
- `EventService` (stateless) + `LLMProvider` (one provider to start) + `GoogleCalendarClient`
- `POST /event` (with `force` support), `PATCH /event`, `GET /health`
- `POST /auth/connect`, `GET /auth/callback` — per-user Google OAuth onboarding
- `UserStore` — `users.json` persistence
- `CalendarClientFactory` — per-user client construction
- Testable end-to-end with Postman

### Phase 2 — Discord Text Bot
- Add `DiscordHandler`
- `!cal connect` onboarding command
- Stateless clarification flow (user re-sends `!cal` with refined text)
- Deploy (e.g., Fly.io, Railway, or a $5 VPS)

### Phase 3 — Voice Input (Discord + Whisper)
- Discord voice channel listener
- Pipe audio to a speech-to-text service → text
- Reuse existing text pipeline from Phase 1

### Phase 4 — Hardening
- Replace `users.json` with SQLite
- Rate limiting per user
- Structured logging and observability

---

## 13. Future Considerations

| Feature | Notes |
|---|---|
| SMS via Twilio | Twilio webhook → HTTP adapter → existing `EventService`; minimal new code |
| Web UI | Small React/HTMX front end calling `POST /event`; auth via session cookie |
| Event deletion | `DELETE` intent already modeled; add `DELETE /event` endpoint and `DeleteEvent` to CalendarClient |
| Multi-calendar support | `GOOGLE_CALENDAR_ID` already per-user; extend to per-user config |
| Reminders / notifications | Add `reminders` field to `CalendarEvent`; pass through to Google API |
| Natural-language event search | New `GET /events/search?q=` endpoint + LLM → date range → Events.List |
| Observability | Structured logging (slog), OpenTelemetry traces, Prometheus metrics |

---

## 14. Open Questions / Known Limitations

1. **LLM provider choice:** Claude is preferred for quality, but OpenAI is a viable fallback. The interface ensures either can be swapped with a single env var change.

2. **Timezone detection:** v1 uses a server-wide default timezone. A more correct approach would detect the Discord user's timezone (e.g., from their locale or a `!cal timezone America/Chicago` setup command), or capture it during OAuth onboarding.

3. **User store concurrency:** v1 uses a JSON file. This is not concurrent-write-safe under load. Acceptable for personal v1 use — move to SQLite for v2.

4. **Natural-language event matching for UPDATE/DELETE:** Matching is inherently fuzzy. A confidence threshold should be defined: above it, auto-select; below it, ask the user to clarify. The exact threshold should be tuned during implementation.

5. **Rate limits:** Google Calendar API free tier allows 1,000,000 queries/day — not a concern for personal use. LLM rate limits are more likely to be hit during testing; handled via retry logic (see Edge Case #16).

6. **Voice API stability (Phase 3):** Evaluate options before committing; a separate voice ingestion service may be more stable than an in-process listener.
