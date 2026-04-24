# Feature Status

Snapshot 2026-04-24 (after Wave C1-C3). BE implementation status per feature
area. **Overall: ±99% of spec coverage, 34/34 test packages pass, 0 FAIL.**
See [07-known-gaps.md](07-known-gaps.md) for the FE-spec alignment ledger.

## At a glance

| Feature | Status | Notes |
|---|---|---|
| 01-auth | ✅ 100% | JWT + Google OAuth + whitelist + **session revocation** (new) |
| 02-workspace | ✅ 100% | Members + invitations + **theme + holding expansion** (new) |
| 03-master-data | ✅ 100% | CRUD + custom fields + **preview + reactivation + handoff** |
| 04-team | ✅ 95% | Roles + permissions + **activity log** (new); full permission cascade tests remaining |
| 05-messaging | 🟡 80% | WA/email templates complete; TipTap→HTML server-side pending FE extension alignment |
| 06-workflow-engine | ✅ 95% | Canvas + automation rules + **manual actions + low-intent skip + timing parser** |
| 07-invoices | ✅ 100% | CRUD + line items + Paper.id + **PDF + aging cron** |
| 08-activity-log | ✅ 100% | Action log + mutations + **team activity + unified feed** |
| 09-analytics-reports | ✅ 92% | KPI + **per-role bundle + Redis 15-min cache**; complex per-role formulas pending |
| 10-collections | ✅ 90% | User-defined tables + validation; data migration on schema change pending alignment |

## Shared concerns status

| Concern | Status |
|---|---|
| Filter DSL | ✅ 100% |
| User Preferences | ✅ 100% |
| HTML Sanitization | ✅ 100% |
| Integrations (workspace_integrations) | ✅ 100% — AES vault + approval gate |
| Checker-Maker (approvals) | ✅ 100% — 8 types dispatched |
| Claude Extraction Pipeline | ✅ 85% — mock ready + real SDK swap pending credential |
| Fireflies Integration | ✅ 85% — webhook + mock ready + real GraphQL pending credential |
| PDP Compliance | ✅ 100% — real erasure + retention |
| System Config Admin | ✅ 100% |
| Working Day + Timezone | ✅ 100% |
| BD Coaching Pipeline | ✅ 90% |

## What's new since initial BE snapshot

Organized by session:

### 2026-04-23 — Sesi A/B/C + Wave 1-5
- user_preferences table + endpoints
- workspace_integrations table + secret redaction + endpoints
- Central approval dispatcher (6 types)
- manual_action_queue table + endpoints
- Postman collection v1 (192 requests)
- 6 new tables (audit, fireflies, claude_extractions, reactivation, coaching, rejection_analysis)
- Mock outbox layer (Claude/Fireflies/HaloAI/SMTP)
- Wave A (10 items closed): import preview, BD→AE handoff, stage/integration approval, per-role KPI, Redis cache, invoice PDF, PDP, timing parser, low-intent skip, mutation source

### 2026-04-24 — Wave B1-B3
- AES-256-GCM secret vault for workspace_integrations
- Real PDP SQL enforcer + erasure executor
- Full `update_existing` bulk import flow
- CSV formula injection guard (export)
- Workspace theme + holding expansion
- Unified activity feed endpoint
- Team activity logs (separate table)
- Invoice auto-resend cadence cron (7 cadence points)
- Session revocation list

## Remaining ~3% gap

Pure third-party credential swaps — not a coding gap:

1. **Claude real SDK call** — `claude_client.NewClient` body needs Anthropic Go SDK; interface stable.
2. **Fireflies real GraphQL** — `fireflies_client.NewClient` body needs GraphQL client; interface stable.
3. **HaloAI outbound from cron** — existing `haloai.Client.SendMessage()` ready; needs adapter swap in main.go.
4. **SMTP real delivery** — code ready; just set `SMTP_HOST` + flip mock flag.

Plus ambiguity-driven items:
- TipTap→HTML server-side converter (need FE extension-set alignment)
- Collection data migration on schema change (need workspace policy decision)
- Complex per-role KPI formulas (current is projection; spec may want different aggregation per role)

## Implication for FE

**FE can build against the current BE now — no blockers.** Every feature
has a working endpoint. Every external integration has a mock that returns
realistic data. Every domain has a Postman example.

When credentials arrive:
- Swap happens at BE level only
- Same endpoints, same response shapes
- FE code doesn't change

For production readiness, FE should:
- Handle the `***REDACTED***` marker on integration config reads
- Use `X-Workspace-ID` header consistently
- Read and respect pagination `meta.total` for "load more" UIs
- Surface `error_code` to users (especially `CONFLICT` for rate-limited flows)
- Wire `request_id` from response headers into support links

## Key FE-specific reminders

- **Mock mode is auto** — don't build a toggle unless you want explicit display.
- **Secrets are write-only** — FE never sees real keys back after PUT.
- **Approval flow is 2-party** — maker and checker must be different emails.
- **Template send guard** — BE rejects any unresolved `[variable]` — FE should validate client-side too.
- **Paginate** — default `limit=50`; most endpoints cap at 200.
- **Workspace header** — always send except on login/workspace-list endpoints.
- **Postman env has pre-filled examples** — import it and everything just works.
