package workday

import (
	"context"
	"testing"
	"time"
)

// mockCalendar is a test double for calendarClient.
type mockCalendar struct {
	holidays map[string]bool
	err      error
}

func (m *mockCalendar) fetchHolidays(_ context.Context, year int) (map[string]bool, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.holidays, nil
}

func newTestProvider(holidays map[string]bool) *Provider {
	p := NewProvider("")
	p.cal = &mockCalendar{holidays: holidays}
	return p
}

func TestIsWorkingDay_Weekend(t *testing.T) {
	p := newTestProvider(nil)

	saturday := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC) // Saturday
	sunday := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)   // Sunday

	if p.IsWorkingDay(context.Background(), saturday) {
		t.Error("Saturday should not be a working day")
	}
	if p.IsWorkingDay(context.Background(), sunday) {
		t.Error("Sunday should not be a working day")
	}
}

func TestIsWorkingDay_NormalWeekday(t *testing.T) {
	p := newTestProvider(nil)

	monday := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC) // Monday
	if !p.IsWorkingDay(context.Background(), monday) {
		t.Error("Monday with no holidays should be a working day")
	}
}

func TestIsWorkingDay_Holiday(t *testing.T) {
	p := newTestProvider(map[string]bool{
		"2026-01-01": true, // New Year
	})

	newYear := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if p.IsWorkingDay(context.Background(), newYear) {
		t.Error("New Year should not be a working day")
	}
}

func TestIsWorkingDay_IndependenceDay(t *testing.T) {
	p := newTestProvider(map[string]bool{
		"2026-08-17": true,
	})

	independence := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if p.IsWorkingDay(context.Background(), independence) {
		t.Error("Indonesian Independence Day should not be a working day")
	}
}

func TestIsWorkingDay_APIErrorFallsBackToStatic(t *testing.T) {
	p := NewProvider("") // no API key → static fallback
	p.cal = &mockCalendar{err: context.DeadlineExceeded}

	// Static fallback includes Jan 1 and Dec 25
	newYear := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if p.IsWorkingDay(context.Background(), newYear) {
		t.Error("New Year should not be a working day even with API error")
	}

	christmas := time.Date(2026, 12, 25, 0, 0, 0, 0, time.UTC)
	if p.IsWorkingDay(context.Background(), christmas) {
		t.Error("Christmas should not be a working day even with API error")
	}
}

func TestWorkingDaysSince_Basic(t *testing.T) {
	p := newTestProvider(nil)

	// Mon Apr 20 → Fri Apr 24 = 5 working days
	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)

	count := p.WorkingDaysSince(context.Background(), from, to)
	if count != 5 {
		t.Errorf("expected 5 working days, got %d", count)
	}
}

func TestWorkingDaysSince_WithHoliday(t *testing.T) {
	p := newTestProvider(map[string]bool{
		"2026-04-21": true, // Tuesday holiday
	})

	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC) // Monday
	to := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)   // Saturday

	count := p.WorkingDaysSince(context.Background(), from, to)
	// Mon(20), Tue(21 holiday), Wed(22), Thu(23), Fri(24) = 4 working days
	if count != 4 {
		t.Errorf("expected 4 working days (1 holiday), got %d", count)
	}
}

func TestWorkingDaysSince_Reversed(t *testing.T) {
	p := newTestProvider(nil)

	from := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

	count := p.WorkingDaysSince(context.Background(), from, to)
	if count != 5 {
		t.Errorf("expected 5 working days (reversed), got %d", count)
	}
}

func TestDaysSince(t *testing.T) {
	from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)

	if got := DaysSince(from, to); got != 5 {
		t.Errorf("DaysSince = %d, want 5", got)
	}
}

func TestDaysSince_Reversed(t *testing.T) {
	from := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

	if got := DaysSince(from, to); got != 5 {
		t.Errorf("DaysSince (reversed) = %d, want 5", got)
	}
}

func TestCacheRefresh(t *testing.T) {
	p := newTestProvider(map[string]bool{})

	ctx := context.Background()
	april20 := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

	// First call populates cache.
	if !p.IsWorkingDay(ctx, april20) {
		t.Error("should be working day")
	}

	// Update mock to return a holiday for that date.
	p.cal.(*mockCalendar).holidays = map[string]bool{
		"2026-04-20": true,
	}

	// Cache should still say it's a working day (not expired yet).
	if !p.IsWorkingDay(ctx, april20) {
		t.Error("cache should still return working day before expiry")
	}

	// Expire the cache manually.
	p.mu.Lock()
	p.expiry[2026] = time.Time{}
	p.mu.Unlock()

	// Now it should fetch fresh data and see the holiday.
	if p.IsWorkingDay(ctx, april20) {
		t.Error("after cache expiry, should see holiday")
	}
}

func TestNewProvider_NoAPIKey(t *testing.T) {
	p := NewProvider("")
	if p.cal != nil {
		t.Error("without API key, calendar client should be nil (static fallback)")
	}
}

func TestNewProvider_WithAPIKey(t *testing.T) {
	p := NewProvider("test-key")
	if p.cal == nil {
		t.Error("with API key, calendar client should be initialized")
	}
}

func TestStaticHolidays_KnownDates(t *testing.T) {
	holidays := staticHolidays(2026)

	expected := []string{
		"2026-01-01", // New Year
		"2026-06-01", // Pancasila Day
		"2026-08-17", // Independence Day
		"2026-12-25", // Christmas
	}

	for _, date := range expected {
		if !holidays[date] {
			t.Errorf("static holidays should include %s", date)
		}
	}
}
