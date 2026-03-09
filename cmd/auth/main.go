// cmd/auth/main.go — one-time OAuth flow to obtain a Google refresh token.
//
// Usage:
//
//	go run cmd/auth/main.go /path/to/credentials.json
//
// Opens a browser for Google consent, then prints the refresh token to stdout.
// Copy the token into your .env as GOOGLE_REFRESH_TOKEN.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	gcal "google.golang.org/api/calendar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	localCallbackAddr = "localhost:9999"
	localCallbackPath = "/oauth/callback"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run cmd/auth/main.go /path/to/credentials.json")
	}
	credsPath := os.Args[1]

	data, err := os.ReadFile(credsPath)
	if err != nil {
		log.Fatalf("read credentials: %v", err)
	}

	cfg, err := google.ConfigFromJSON(data, gcal.CalendarEventsScope)
	if err != nil {
		log.Fatalf("parse credentials: %v", err)
	}

	// Override redirect URI to our local server.
	cfg.RedirectURL = "http://" + localCallbackAddr + localCallbackPath

	// Generate a random CSRF state nonce to protect the callback.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		log.Fatalf("generate state nonce: %v", err)
	}
	csrfState := hex.EncodeToString(stateBytes)

	type result struct {
		code string
		err  error
	}
	resultCh := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(localCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		// Handle OAuth errors (e.g., user denied consent).
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			http.Error(w, "Authorization denied: "+oauthErr, http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("authorization denied: %s", oauthErr)}
			return
		}

		// Validate CSRF state.
		if r.URL.Query().Get("state") != csrfState {
			http.Error(w, "invalid state", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("invalid state parameter — possible CSRF")}
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			resultCh <- result{err: fmt.Errorf("missing authorization code")}
			return
		}
		fmt.Fprintln(w, "Auth complete — you can close this tab.")
		resultCh <- result{code: code}
	})

	srv := &http.Server{Addr: localCallbackAddr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("callback server: %v", err)
		}
	}()

	// ApprovalForce ensures Google always issues a new refresh token,
	// even if the user has previously granted consent.
	authURL := cfg.AuthCodeURL(csrfState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("\nOpen this URL in your browser to authorize:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("Waiting for callback on", "http://"+localCallbackAddr+localCallbackPath, "...")

	res := <-resultCh
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)

	if res.err != nil {
		log.Fatalf("auth failed: %v", res.err)
	}

	token, err := cfg.Exchange(context.Background(), res.code)
	if err != nil {
		log.Fatalf("exchange code: %v", err)
	}

	if token.RefreshToken == "" {
		log.Fatal("no refresh token returned by Google — revoke app access at " +
			"https://myaccount.google.com/permissions and re-run")
	}

	out, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		log.Fatalf("marshal token: %v", err)
	}
	fmt.Println("\n--- Token details ---")
	fmt.Println(string(out))

	fmt.Println("\n--- Copy this into your .env ---")
	fmt.Printf("GOOGLE_REFRESH_TOKEN=%s\n", token.RefreshToken)
}
