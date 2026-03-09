package calendar

import (
	"context"
	"fmt"
	"os"

	gcal "google.golang.org/api/calendar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"github.com/kushturner/calendar-no-mi/internal/models"
)

// GoogleCalendarClientFactory implements CalendarClientFactory using OAuth2 credentials.
type GoogleCalendarClientFactory struct {
	oauthConfig *oauth2.Config
}

// NewGoogleCalendarClientFactory loads OAuth2 credentials from credsPath and returns a factory.
func NewGoogleCalendarClientFactory(credsPath string) (*GoogleCalendarClientFactory, error) {
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("calendar: read credentials file %q: %w", credsPath, err)
	}

	cfg, err := google.ConfigFromJSON(data, gcal.CalendarEventsScope)
	if err != nil {
		return nil, fmt.Errorf("calendar: parse credentials: %w", err)
	}

	return &GoogleCalendarClientFactory{oauthConfig: cfg}, nil
}

// ForUser builds a CalendarClient scoped to the given user's refresh token and calendar ID.
func (f *GoogleCalendarClientFactory) ForUser(ctx context.Context, user models.User) (CalendarClient, error) {
	if user.RefreshToken == "" {
		return nil, fmt.Errorf("calendar: user %s has no refresh token", user.ID)
	}
	token := &oauth2.Token{RefreshToken: user.RefreshToken}
	ts := f.oauthConfig.TokenSource(ctx, token)

	svc, err := gcal.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("calendar: create service for user %s: %w", user.ID, err)
	}

	return &googleCalendarClient{svc: svc, calendarID: user.CalendarID}, nil
}
