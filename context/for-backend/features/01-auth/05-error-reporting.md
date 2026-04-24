# Error Reporting

## Overview
Frontend mengirim client-side errors ke backend untuk monitoring.
File: `lib/error-reporter.ts` → POST `/api/error-report`

## API Endpoint

### POST `/error-report`
Menerima error dari frontend (ErrorBoundary, unhandled exceptions).

```json
// Request body:
{
  "message": "Cannot read property 'map' of undefined",
  "stack": "TypeError: Cannot read property...\n    at ClientLifecycleTable...",
  "componentName": "ClientLifecycleTable",
  "timestamp": "2026-04-12T10:30:00.000Z",
  "userAgent": "Mozilla/5.0...",
  "pathname": "/dashboard/ws-dealls-001/ae",
  "extra": {
    "workspaceId": "ws-dealls-001",
    "userId": "user-uuid"
  }
}

// Response 200:
{ "received": true }
```

## Backend Logic
1. Validate payload (message required, max 10KB body)
2. Log as structured JSON to stdout (for log aggregator)
3. Optional: forward to Sentry/Datadog/PagerDuty
4. Rate limit: 30 req/min per IP (prevent spam)

## Database (optional)
```sql
CREATE TABLE client_errors (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  message     TEXT NOT NULL,
  stack       TEXT,
  component   VARCHAR(100),
  pathname    VARCHAR(255),
  user_id     UUID,
  workspace_id UUID,
  user_agent  TEXT,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Cleanup: delete errors older than 30 days
-- Cron: DELETE FROM client_errors WHERE created_at < NOW() - INTERVAL '30 days';
```

## Note
Ini endpoint low-priority — bisa di-skip di awal development.
Cukup console.log structured JSON di backend, nanti switch ke Sentry saat production.
