# cs-agent-bot — Project Overview

## Mission

`cs-agent-bot` is a Go microservice implementing an AI-powered customer service automation platform for **Kantorku.id** HRIS SaaS. It acts as a virtual Account Executive (AE) managing client lifecycle through automated WhatsApp outreach via the **HaloAI** API.

**Critical principle:** The bot never marks a client as *Paid*, *Renewed*, or *Churned* — only the AE does that via the Dashboard. The bot's role is engagement and audit; the AE retains final control.

## Core Capabilities

- Scheduled batch outreach (GCP Cloud Scheduler at 09:00 WIB) across six sequential client-lifecycle phases (P0–P5).
- Inbound webhook handling for WhatsApp replies (HaloAI).
- Intent classification (9 categories) driving automated responses and escalations.
- Multi-tenant dashboard (workspaces) for AEs: client data master, invoices, activity log, analytics, workflow engine.
- Invoice lifecycle management with Paper.id payment webhook integration.
- Escalation orchestration with Telegram alerts to AE leads.

## Tech Stack

| Layer | Choice |
|---|---|
| Language | Go 1.25 |
| HTTP | Custom `net/http` router (no Gin/Echo/Fiber) |
| Database | PostgreSQL 13+ via `pgx/v5` + `Masterminds/squirrel` |
| Cache/Sessions | Redis 6.2+ via `go-redis/v9` |
| Logging | `zerolog` (structured JSON) |
| Tracing | OpenTelemetry (GCP exporters, Zipkin fallback) |
| API Docs | Swagger via `swaggo/swag` |
| Validation | `go-playground/validator/v10` |
| Excel | `xuri/excelize/v2` (import/export) |
| HTML sanitization | `microcosm-cc/bluemonday` |
| Deployment | Docker (multi-stage), Docker Compose for local dev |

## External Integrations

- **HaloAI** — WhatsApp Business API provider (send + receive webhooks).
- **Telegram Bot API** — AE alerts and escalation notifications.
- **GCP Cloud Scheduler** — daily cron trigger (OIDC-authenticated).
- **Paper.id** — payment notification webhooks.
- **JWT validator endpoint** — external service validates dashboard JWTs (`JWT_VALIDATE_URL`).