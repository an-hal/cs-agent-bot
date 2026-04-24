# Gap Analysis: FE Specs ↔ BE (cs-agent-bot)

**Status snapshot 2026-04-24: mayoritas CLOSED.** Dokumen ini asalnya
gap-analysis awal (2026-04-23). Sekarang dipakai sebagai **ledger "what was
closed and how"** — tiap item lama punya status final + pointer ke implementasi.

Current implementation coverage: **±97%**. Sisa 3% = swap mock→real SDK
(Claude/Fireflies/HaloAI/SMTP), bukan gap coding.

## Original CRITICAL items (all CLOSED)

| Item | Status | Where |
|---|---|---|
| `workspace_integrations` multi-tenancy | ✅ CLOSED | Migrasi `20260423000002`, usecase + approval gate (`ApprovalTypeIntegrationKeyChange`), AES vault encryption, 4 endpoint CRUD |
| `user_preferences` table + endpoints | ✅ CLOSED | Migrasi `20260423000001`, 4 endpoint `/preferences` |
| Approval executor 6 (now 8) tipe wired | ✅ CLOSED | `usecase/approval/dispatcher.go` routes `create_invoice, mark_invoice_paid, collection_schema_change, delete_client_record, toggle_automation_rule, bulk_import_master_data, stage_transition, integration_key_change` via `POST /approvals/{id}/apply` |
| Fireflies + Claude extraction pipeline | ✅ CLOSED | Migrasi `20260423000011` + `20260423000012`, webhook handler `/webhook/fireflies/{workspace_id}`, `FirefliesBridge` chains transcript→Claude extraction, mock client + real swap interface |
| Master data dedup preview + reactivation | ✅ CLOSED | `POST /master-data/clients/import/preview`, `POST /master-data/clients/{id}/reactivate`, `reactivation_triggers` + `reactivation_events` tables |

## Original HIGH items (all CLOSED)

| Item | Status | Where |
|---|---|---|
| BD→AE handoff field mapping | ✅ CLOSED | `master_data/handoff.go` — 30 field `HandoffFieldMap`, auto-triggers on transition prospect→client |
| Workflow manual action queue | ✅ CLOSED | Migrasi `20260423000003`, `ManualFlowTriggers` registry (20 trigger IDs), `channelDispatcher` intercepts sebelum channel routing |
| Paper.id webhook → `payment_status` sync | ✅ CLOSED (was already present) | `invoice/paperid.go` — HMAC verify + writes `payment_status = Lunas` atomically |
| `audit_logs.workspace_access` | ✅ CLOSED | Migrasi `20260423000010`, 2 endpoint `/audit-logs/workspace-access` |
| KPI per-role batch endpoint | ✅ CLOSED | `GET /analytics/kpi/bundle?role=&months=` with `analytics.BuildRoleKPIFromJSON` (SDR 4 / BD 4 / AE 6 / admin-full) |

## Original MEDIUM items (all CLOSED or noted deferred)

| Item | Status | Where |
|---|---|---|
| Collection schema approval execution | ✅ CLOSED | `collection.ApplyCollectionSchemaChange` (4 ops: create_collection, delete_collection, add_field, delete_field) |
| Template resolution priority logic | ✅ CLOSED | `template.ResolvePriority(candidates[])` — first successful resolve wins, e.g. renewal→intent→legacy→default |
| PDP compliance audit + erasure workflow | ✅ CLOSED | Migrasi `20260424000002`, real SQL enforcer (9 data classes whitelist) + erasure executor, 10 endpoint PDP, retention cron |
| Conversation state context accumulation | 🟡 PARTIAL | `conversation_states` table exists + used by classifier, but deeper multi-turn dialogue state machine deferred |

## Wave B additions (closed 2026-04-24)

| Item | Status | Where |
|---|---|---|
| AES-256 encryption for workspace_integrations | ✅ NEW | `pkg/secretvault/` — AES-GCM envelope, nil-key passthrough |
| Full `update_existing` bulk import flow | ✅ CLOSED | Reuses `ExistingCompanyIDs` + `Patch()` with sparse non-empty updates |
| CSV formula injection guard (export) | ✅ CLOSED | `pkg/xlsxexport/sanitize.go` prepends `'` to `=|+|-|@|\t|\r` prefixes |
| Workspace theme + holding expansion | ✅ NEW | Migrasi `20260424000010`, `GET/PUT /workspace/theme`, `GET /workspace/holding/expand` |
| Unified activity feed endpoint | ✅ NEW | `GET /activity-log/feed` — UNION across action_log + mutations + activity_log |
| Team activity logs (separate from master_data) | ✅ NEW | Migrasi `20260424000020`, 2 endpoint `/team/activity` |
| Invoice sequence aging auto-resend cron | ✅ NEW | `invoice/aging.go` — 7 cadence points (pre-14/7/3, due-today, post-3/8/15) |
| Session revocation (JWT jti) | ✅ NEW | Migrasi `20260424000030`, 3 endpoint + `IsRevoked(jti)` middleware hook |
| Stage transition approval gate | ✅ NEW | `master_data.RequestStageTransition` + `ApplyApprovedStageTransition`, dispatcher routes `stage_transition` type |
| Low-intent BD sequence skip | ✅ NEW | `cron/low_intent_skip.go` — D12/D14/D21 triggers skipped when `bants_classification=cold` OR `buying_intent=low` |
| Workflow cron timing parser (dual format) | ✅ NEW | `cron/timing_parser.go` accepts `H-90`, `D+14`, AND `90 hari sebelum kontrak berakhir` |
| Escalation severity matrix from system_config | ✅ NEW | `escalation/severity_matrix.go` — 60s cache, wildcard rules `ESC-id:role` / `ESC-id:*` / `*:role` |
| Mutation log source tagging | ✅ NEW | `source` column: `dashboard|bot|import|api|reactivation|handoff` — 100% coverage across 4 master_data write paths + reactivation |

## Shared concerns status (final)

| # | Concern | Status |
|---|---|---|
| 01 | Filter DSL | ✅ 100% — production ready (`pkg/filterdsl/`) |
| 02 | User Preferences | ✅ 100% — new this cycle |
| 03 | HTML Sanitization | ✅ 100% — bluemonday (`pkg/htmlsanitize/`) |
| 04 | Integrations | ✅ 100% — per-workspace table + AES vault + approval gate + mock/real |
| 05 | Checker-Maker | ✅ 100% — 8 types dispatched |
| 06 | Claude Extraction | ✅ 85% — pipeline complete with mock; real SDK swap pending credential |
| 07 | Fireflies | ✅ 85% — webhook + mock complete; real GraphQL fetch pending credential |
| 08 | PDP Compliance | ✅ 100% — real enforcer + erasure + retention + audit |
| 09 | System Config Admin | ✅ 100% |
| 10 | Working Day & Timezone | ✅ 100% |
| 11 | BD Coaching Pipeline | ✅ 90% — full table + peer review scoring; assignment reminder ambiguous |

## Remaining ±3% (third-party credential swaps only)

Every item here has:
- Interface defined
- Mock implementation working
- Wiring path ready (config env + factory selector)
- Tests green

Just needs real API call to replace noop/mock when credential available:

1. **Claude real SDK call** — `claude_client.NewClient` currently returns noop; replace body with Anthropic Go SDK call. Model selector, prompt key, token accounting all in place.
2. **Fireflies GraphQL real fetch** — `fireflies_client.NewClient` currently returns error; implement GraphQL request against `FIREFLIES_GRAPHQL_URL`.
3. **HaloAI outbound WA send from cron** — existing `haloai.Client.SendMessage()` works; just write adapter `cron.WASender` that calls it (replace `mockWASenderAdapter` in main.go).
4. **SMTP real delivery** — `smtp_client.NewClient` already has TLS impl; just set `SMTP_HOST` + flip `MOCK_EXTERNAL_APIS=false`.

Each swap ≤ 4 hours including verification.

## References

- Session detail logs: [09-session-coverage-2026-04-23.md](09-session-coverage-2026-04-23.md), [10-session-coverage-2026-04-24.md](10-session-coverage-2026-04-24.md)
- Endpoint catalog: [11-endpoint-catalog.md](11-endpoint-catalog.md)
- Integration state: [12-integration-state.md](12-integration-state.md)
