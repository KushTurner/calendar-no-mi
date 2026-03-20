# calendar-no-mi

Natural-language calendar event creation. Send a plain-English description of an event and it gets added to your Google Calendar.

```
POST /event  →  "Team standup tomorrow at 9am for 30 minutes"  →  Google Calendar
```

See [DESIGN.md](DESIGN.md) for architecture details.

---

## Prerequisites

- Go 1.21+
- A Google account with Google Calendar
- An [OpenAI API key](https://platform.openai.com/api-keys)

---

## Setup

### 1. Clone and build

```bash
git clone https://github.com/kushturner/calendar-no-mi
cd calendar-no-mi
go build ./...
```

### 2. Get an OpenAI API key

Go to [platform.openai.com/api-keys](https://platform.openai.com/api-keys), create a key, and keep it handy for the `.env` step.

### 3. Set up Google Calendar API credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/) and create a project.
2. Enable the **Google Calendar API**: APIs & Services → Library → search "Google Calendar API" → Enable.
3. Create OAuth credentials: APIs & Services → Credentials → Create Credentials → **OAuth client ID**.
   - Application type: **Desktop app** (required — the helper binds `localhost:9999` for the OAuth callback)
   - Name it anything (e.g. "calendar-no-mi")
4. Download the JSON file and save it as `credentials.json` in the project root.

The app reads this file at startup via `GOOGLE_CREDENTIALS_FILE`.

Reference: [Google Calendar API Go Quickstart](https://developers.google.com/calendar/api/quickstart/go)

### 4. Generate the Google refresh token (one-time)

The server needs a refresh token to access your calendar without prompting you on each restart. Run the included OAuth helper:

```bash
go run cmd/auth/main.go credentials.json
```

It will:
1. Print an authorization URL — open it in your browser
2. Grant calendar access in the browser; the callback is handled automatically
3. Print the refresh token to your terminal once the flow completes

Copy the `GOOGLE_REFRESH_TOKEN=...` line it prints into your `.env`.

> The refresh token is long-lived but can be revoked. Treat it like a password and keep it out of version control.
>
> If you ever need a new one, revoke the app's access at [myaccount.google.com/permissions](https://myaccount.google.com/permissions) and re-run the helper.

### 5. Configure `.env`

```bash
cp .env.example .env
```

Edit `.env` with your values:

```bash
# Port the HTTP server listens on (default: 8080)
HTTP_PORT=8080

# Path to the OAuth2 credentials JSON downloaded from Google Cloud Console
GOOGLE_CREDENTIALS_FILE=credentials.json

# Refresh token from step 4
GOOGLE_REFRESH_TOKEN=your_refresh_token_here

# Which calendar to write to ("primary" works for most accounts)
GOOGLE_CALENDAR_ID=primary

# OpenAI API key from step 2
OPENAI_API_KEY=sk-...

# Bearer token for the HTTP API — pick any random string
HTTP_BEARER_TOKEN=your_secret_token_here

# Your local timezone (used when the LLM interprets event times)
DEFAULT_TIMEZONE=Europe/London

# Logging verbosity: debug, info, warn, error (default: info)
# LOG_LEVEL=debug

# Environment: "production" uses JSON logs to stdout; anything else uses text to stderr
# APP_ENV=development
```

`HTTP_BEARER_TOKEN` is used to authenticate requests to `POST /event`. Set it to any secret string you choose.

### 6. Run

```bash
go run cmd/server/main.go
```

---

## Verify

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Create an event
curl -X POST http://localhost:8080/event \
  -H "Authorization: Bearer your_secret_token_here" \
  -H "Content-Type: application/json" \
  -d '{"text": "Team standup tomorrow at 9am for 30 minutes"}'
```

A successful response returns a link to the created event:

```json
{"event_url": "https://www.google.com/calendar/event?eid=..."}
```

