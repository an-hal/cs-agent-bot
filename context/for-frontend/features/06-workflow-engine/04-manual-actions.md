# Workflow Engine — Manual Actions Overlay

Full spec for the "GUARD" manual-flow overlay lives in
[../00-shared/04-manual-actions.md](../00-shared/04-manual-actions.md).

## How it ties into the workflow engine

The cron channel dispatcher intercepts trigger IDs listed in
`ManualFlowTriggers` registry BEFORE routing to the WA/email/telegram
channels. When a manual-flow trigger fires:

1. `channelDispatcher.Dispatch()` checks `IsManualFlow(rule.TriggerID)`.
2. If true → calls the enqueuer (`ManualActionEnqueuer.Enqueue`) instead
   of sending via WA.
3. A row appears in `manual_action_queue` with priority + role + assignee
   derived from the trigger ID prefix.
4. Telegram DM (best-effort, when notifier is wired) alerts the assignee.

## Low-intent skip (anti-spam)

Separate mechanism: BD sequence triggers D12/D14/D21 auto-skip when the
client shows low-intent signals — see `custom_fields`:
- `bants_classification = "cold"`
- `buying_intent = "low"`

These take precedence over manual-flow routing. Skip is logged (not
enqueued) — FE shouldn't see the manual-action row for skipped triggers.

## Timing parser (cron engine)

The cron loop resolves each `AutomationRule.Timing` string to a day-offset
against the rule's anchor. Both legacy + Indonesian formats supported:

| Format | Anchor | Offset |
|---|---|---|
| `H-90` | `contract_end` | -90 |
| `D+14` | `contract_start` | +14 |
| `D-3` | `invoice_due` | -3 |
| `90 hari sebelum kontrak berakhir` | `contract_end` | -90 |
| `14 hari setelah aktivasi` | `contract_start` | +14 |
| `3 hari sebelum jatuh tempo` | `invoice_due` | -3 |
| `post-activation D+14` | `contract_start` | +14 |
| `30 hari setelah dibayar` | `invoice_paid` | +30 |

FE can use the same parser format when composing rule UIs — BE will
accept either format.

## Template resolution priority

BE exposes `template.ResolvePriority(candidates[])` for multi-fallback
template selection (renewal → intent → legacy → default). Usage:

```go
body, chosenID, err := template.ResolvePriority(ctx, resolver,
  []string{"REN90_V2", "REN90_INTENT", "REN90_LEGACY", "REN90_DEFAULT"},
  client, invoice, cfg)
```

First candidate that resolves cleanly wins. The chosen ID is audit-logged.

## FE touchpoints

- **Manual action queue UI** — see shared doc.
- **Workflow canvas** — manual-flow nodes should be marked visually
  (e.g. human-icon pill) so admins know the trigger won't auto-send.
- **Rule editor** — timing input accepts both formats; validate via
  regex client-side (fallback: BE returns 400 with format guidance).
- **Template picker** — when editing a rule, allow multi-fallback
  candidate list (priority order).
