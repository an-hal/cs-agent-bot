# Working Day & Timezone

## Overview

Backend bot send window, Indonesian public-holiday handling, and timezone semantics. ALL outbound bot communications (WA, email, Telegram-to-prospect) MUST go through the `isWorkingDay` + `nextSendableTime` checker before enqueuing. Internal Telegram alerts to BD users are exempt.

Reference timezone: **Asia/Jakarta (WIB, UTC+7)**. Indonesia does NOT observe Daylight Saving Time — backend MUST NOT add DST logic.

## Bot Send Window

```
Morning slot:    09:00 – 11:30 WIB
Afternoon slot:  13:30 – 16:30 WIB
Total:           5 hours/day on working days
```

Outside these slots → defer to next sendable slot (same day if morning miss, next day if afternoon miss).

`isWithinSendWindow(t time.Time) bool` is the only function that knows these bounds. Hardcoded in Go — NOT in `system_config` (changing send hours is a controlled deploy, not an admin tweak).

## Holiday Calendar

Backend reads holidays from `system_config[INDONESIAN_HOLIDAYS_2026]` per workspace. Allows per-workspace overrides (e.g., a workspace serving banking sector may add `2026-12-31` for a customer-side bank holiday).

```
Key:        INDONESIAN_HOLIDAYS_2026
Data type:  json
Value:      ["2026-01-01", "2026-03-31", "2026-04-01", "2026-06-07", "2026-08-17", "2026-12-25"]
```

### 2026 Baseline Holidays (Seed)

These MUST be inserted via migration when a workspace is created:

| Date | Holiday |
|---|---|
| 2026-01-01 | Tahun Baru Masehi |
| 2026-03-31 | Idul Fitri 1447 H (Day 1) |
| 2026-04-01 | Idul Fitri 1447 H (Day 2) |
| 2026-06-07 | Idul Adha 1447 H |
| 2026-08-17 | Hari Kemerdekaan RI |
| 2026-12-25 | Hari Raya Natal |

Workspace Admin/Lead may add more via Settings → System Config (e.g., regional holidays, company off-days). Backend treats the JSON array as the full source of truth — no implicit baseline appended at runtime.

### Holiday Handling

```
isWorkingDay(date, workspace_id) → false  if:
  - date weekday ∈ {Saturday, Sunday}
  - YYYY-MM-DD(date) ∈ INDONESIAN_HOLIDAYS_{year}
```

When `isWorkingDay` is false:
- Bot send is skipped entirely for that day
- Queued sends defer to **next working day at 09:00 WIB**
- BD owner receives a Telegram digest at 08:00 WIB on the next working day listing all deferred sends

### Tahun Baru Exception (and other seasonal blasts)

Templates with `template_type = 'seasonal_blast'` (e.g., New Year greeting, Idul Fitri greeting) are MEANT to fire AT the holiday. Backend MUST bypass the working-day rule for these. Identification:

```
SELECT * FROM message_queue
WHERE template_type = 'seasonal_blast'
  AND scheduled_at BETWEEN holiday_start AND holiday_end;
```

These still respect the send window (09:00–11:30 / 13:30–16:30) — Tahun Baru greeting goes out at 09:00 WIB on Jan 1, not at midnight.

## DST

**Indonesia does NOT observe Daylight Saving Time.** UTC offset is constant at +07:00 year-round. Backend MUST NOT:
- Use `time.LoadLocation("Asia/Jakarta")` and rely on Go's tzdata for DST adjustments — there are none, so it's a wasted complexity
- Apply any seasonal +1h/-1h shift to send windows
- Account for DST when scheduling cross-region cron (no fellow-tz peer of Jakarta observes DST either)

This is documented explicitly so a future engineer doesn't introduce DST bugs after seeing other tz codebases.

## Cron Checker Functions

```go
// isWorkingDay returns true if the given date in Asia/Jakarta is a weekday AND not a holiday
// for the given workspace. workspaceID determines the holiday set (per-workspace override).
func isWorkingDay(t time.Time, workspaceID uuid.UUID) (bool, error) {
    jkt := t.In(time.FixedZone("WIB", 7*3600))
    weekday := jkt.Weekday()
    if weekday == time.Saturday || weekday == time.Sunday {
        return false, nil
    }
    holidays, err := getHolidays(workspaceID, jkt.Year())
    if err != nil {
        return false, err
    }
    dateStr := jkt.Format("2006-01-02")
    for _, h := range holidays {
        if h == dateStr {
            return false, nil
        }
    }
    return true, nil
}

// nextSendableTime returns the next time within the send window when bot can send.
// If `from` is already in window AND working day → returns `from`.
// Otherwise rolls forward to next working-day slot.
func nextSendableTime(from time.Time, workspaceID uuid.UUID) (time.Time, error) {
    jkt := from.In(time.FixedZone("WIB", 7*3600))
    for i := 0; i < 14; i++ {                              // safety cap: 2 weeks
        candidate := jkt.AddDate(0, 0, i)
        if i > 0 {
            candidate = time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 9, 0, 0, 0, candidate.Location())
        }
        ok, err := isWorkingDay(candidate, workspaceID)
        if err != nil {
            return time.Time{}, err
        }
        if !ok {
            continue
        }
        // candidate day is a working day; find slot
        h, m := candidate.Hour(), candidate.Minute()
        switch {
        case h < 9:
            return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 9, 0, 0, 0, candidate.Location()), nil
        case h == 9 || h == 10 || (h == 11 && m <= 30):
            return candidate, nil
        case (h == 11 && m > 30) || h == 12 || (h == 13 && m < 30):
            return time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 13, 30, 0, 0, candidate.Location()), nil
        case h == 13 || h == 14 || h == 15 || (h == 16 && m <= 30):
            return candidate, nil
        default:
            // past 16:30 → roll to next day morning
            continue
        }
    }
    return time.Time{}, fmt.Errorf("no sendable time within 14 days")
}
```

`getHolidays(workspaceID, year)` reads `system_config[INDONESIAN_HOLIDAYS_{year}]`, parses the JSON array, caches in memory for 60s.

## API

### GET `/api/_health/timezone`

Health endpoint exposing timezone state. Used by FE to detect server-side misconfiguration (e.g., a deploy on a server with wrong tz).

```
Response 200:
{
  "tz": "Asia/Jakarta",
  "offset": "+07:00",
  "current_server_time": "2026-04-22T15:30:00+07:00",
  "is_working_day": true,
  "is_within_send_window": true,
  "holidays_count": 6,
  "next_send_window_starts": "2026-04-22T13:30:00+07:00",   // current slot if active, else next slot
  "next_holiday": "2026-06-07"
}
```

If `tz` does NOT equal `"Asia/Jakarta"` — FE displays a banner and the deploy pipeline alerts. The server config is wrong; bots will misfire.

## Backend MUST

- Apply `isWorkingDay` + `nextSendableTime` to EVERY outbound bot send before enqueuing
- Use FixedZone(+07:00) — do NOT depend on host system tz settings
- Read holidays from `system_config[INDONESIAN_HOLIDAYS_{year}]` per workspace; cache 60s
- Bypass working-day rule for `template_type='seasonal_blast'` only
- Log every deferred send to `bot_send_deferrals` (prospect_id, original_scheduled_at, deferred_to, reason)

## Backend MAY

- Pre-compute next 30 days' working-day map per workspace into Redis at 00:00 WIB cron
- Allow per-template override `respect_working_hours=false` for ad-hoc Lead-triggered campaigns (audit-logged)
- Push a `bot_send_deferred` Telegram digest at 08:00 WIB summarising what was deferred yesterday
