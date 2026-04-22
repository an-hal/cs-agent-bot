package workday

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// calendarClient is the interface for fetching holiday data.
type calendarClient interface {
	fetchHolidays(ctx context.Context, year int) (map[string]bool, error)
}

// googleCalendar fetches Indonesian public holidays from Google Calendar API.
type googleCalendar struct {
	apiKey string
}

// Indonesian public holiday calendar ID.
const calendarID = "en.indonesian#holiday@group.v.calendar.google.com"

type calEvents struct {
	Items []struct {
		Start struct {
			Date string `json:"date"`
		} `json:"start"`
	} `json:"items"`
}

func (g *googleCalendar) fetchHolidays(ctx context.Context, year int) (map[string]bool, error) {
	timeMin := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	timeMax := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC).Format(time.RFC3339)

	url := fmt.Sprintf(
		"https://www.googleapis.com/calendar/v3/calendars/%s/events?key=%s&timeMin=%s&timeMax=%s&singleEvents=true&maxResults=250",
		calendarID, g.apiKey, timeMin, timeMax,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("workday: build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("workday: fetch calendar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("workday: calendar API returned %d: %s", resp.StatusCode, string(body))
	}

	var events calEvents
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, fmt.Errorf("workday: decode response: %w", err)
	}

	holidays := make(map[string]bool, len(events.Items))
	for _, item := range events.Items {
		if item.Start.Date != "" {
			holidays[item.Start.Date] = true
		}
	}
	return holidays, nil
}

// staticHolidays returns a hardcoded set of Indonesian public holidays for a given year.
// Used as fallback when Google Calendar API is unavailable.
// This is not exhaustive — only covers major national holidays.
func staticHolidays(year int) map[string]bool {
	y := fmt.Sprintf("%d", year)
	dates := []string{
		// New Year
		y + "-01-01",
		// Independence Day
		y + "-08-17",
		// Christmas
		y + "-12-25",
	}

	// Pancasila Day (June 1)
	dates = append(dates, y+"-06-01")

	holidays := make(map[string]bool, len(dates))
	for _, d := range dates {
		if _, err := time.Parse("2006-01-02", d); err == nil {
			holidays[d] = true
		}
	}
	return holidays
}

