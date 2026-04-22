// Package workday provides working-day helpers backed by Google Calendar
// (Indonesian public holidays) with an in-memory cache and static fallback.
package workday

import (
	"context"
	"log"
	"sync"
	"time"
)

// Provider resolves whether a date is a holiday (non-working) in Indonesia.
type Provider struct {
	cal    calendarClient
	cache  map[int]map[string]bool // year → "YYYY-MM-DD" → true
	mu     sync.RWMutex
	ttl    time.Duration
	expiry map[int]time.Time // year → when cache expires
}

// NewProvider creates a workday provider. apiKey is the Google Calendar API key.
// If apiKey is empty, the provider falls back to static Indonesian holidays only.
func NewProvider(apiKey string) *Provider {
	var cal calendarClient
	if apiKey != "" {
		cal = &googleCalendar{apiKey: apiKey}
	}
	return &Provider{
		cal:    cal,
		cache:  make(map[int]map[string]bool),
		ttl:    24 * time.Hour,
		expiry: make(map[int]time.Time),
	}
}

// IsWorkingDay reports whether the given date falls on a working day
// (Monday–Friday and not an Indonesian public holiday).
func (p *Provider) IsWorkingDay(ctx context.Context, date time.Time) bool {
	wd := date.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	holidays, err := p.holidays(ctx, date.Year())
	if err != nil {
		log.Printf("workday: falling back to static holidays for %d: %v", date.Year(), err)
		holidays = staticHolidays(date.Year())
	}
	_, isHoliday := holidays[date.Format("2006-01-02")]
	return !isHoliday
}

// WorkingDaysSince counts working days between from (inclusive) and to (exclusive).
func (p *Provider) WorkingDaysSince(ctx context.Context, from, to time.Time) int {
	if from.After(to) {
		from, to = to, from
	}
	count := 0
	d := from
	for d.Before(to) {
		if p.IsWorkingDay(ctx, d) {
			count++
		}
		d = d.AddDate(0, 0, 1)
	}
	return count
}

// DaysSince returns calendar days between from (inclusive) and to (exclusive).
func DaysSince(from, to time.Time) int {
	if from.After(to) {
		from, to = to, from
	}
	return int(to.Sub(from).Hours() / 24)
}

// holidays returns the holiday set for a given year, using cache or fetching.
func (p *Provider) holidays(ctx context.Context, year int) (map[string]bool, error) {
	p.mu.RLock()
	if m, ok := p.cache[year]; ok {
		if exp, hasExp := p.expiry[year]; hasExp && time.Now().Before(exp) {
			p.mu.RUnlock()
			return m, nil
		}
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock.
	if m, ok := p.cache[year]; ok {
		if exp, hasExp := p.expiry[year]; hasExp && time.Now().Before(exp) {
			return m, nil
		}
	}

	// Fetch from Google Calendar, fall back to static.
	var holidays map[string]bool
	if p.cal != nil {
		fetched, err := p.cal.fetchHolidays(ctx, year)
		if err != nil {
			return nil, err
		}
		holidays = fetched
	} else {
		holidays = staticHolidays(year)
	}

	p.cache[year] = holidays
	p.expiry[year] = time.Now().Add(p.ttl)
	return holidays, nil
}
