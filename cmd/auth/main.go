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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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

	codeCh := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(localCallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Auth complete — you can close this tab.")
		codeCh <- code
	})

	srv := &http.Server{Addr: localCallbackAddr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("callback server: %v", err)
		}
	}()

	authURL := cfg.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("\nOpen this URL in your browser to authorize:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("Waiting for callback on", "http://"+localCallbackAddr+localCallbackPath, "...")

	code := <-codeCh
	_ = srv.Shutdown(context.Background())

	token, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("exchange code: %v", err)
	}

	fmt.Println("\n--- Token details ---")
	out, _ := json.MarshalIndent(token, "", "  ")
	fmt.Println(string(out))

	fmt.Println("\n--- Copy this into your .env ---")
	fmt.Printf("GOOGLE_REFRESH_TOKEN=%s\n", token.RefreshToken)
}
