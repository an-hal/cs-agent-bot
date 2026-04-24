# Workflow Engine — Backend Implementation Guide

## Context
Workflow Engine adalah inti dari automation system. Setiap workflow mendefinisikan pipeline
(urutan node yang di-execute oleh cron/event) yang membaca dan menulis ke **Master Data**.

Dashboard ini menyediakan:
1. **Workflow Builder** — visual canvas (React Flow) untuk membangun dan mengedit flow
2. **Pipeline View** — tabel data per workflow dengan tabs/stats/columns yang customizable
3. **Step Config** — konfigurasi per step (timing, condition, template assignment)
4. **Automation Rules** — library aturan otomasi yang bisa di-edit, pause, enable per workspace

## Arsitektur: WorkflowItem -> Nodes -> Edges -> Master Data

```
                        ┌──────────────────────────────┐
                        │       WORKFLOW BUILDER        │
                        │  (React Flow visual canvas)   │
                        └──────────┬───────────────────┘
                                   │ save canvas
                                   ▼
                   ┌───────────────────────────────────┐
                   │          workflows table           │
                   │  key, name, icon, status           │
                   │  workspace_id                      │
                   └───────┬──────────┬────────────────┘
                           │          │
              ┌────────────┘          └────────────┐
              ▼                                    ▼
   ┌──────────────────┐                ┌──────────────────┐
   │  workflow_nodes   │                │  workflow_edges   │
   │  id, type, pos    │◄──────────────│  source, target   │
   │  data (JSONB)     │    connects   │  style, label     │
   │  templateId       │               └──────────────────┘
   │  triggerId        │
   └──────┬───────────┘
          │ references
          ▼
   ┌──────────────────┐         ┌──────────────────────┐
   │ automation_rules  │────────│  pipeline_configs     │
   │ trigger_id        │        │  tabs, stats, columns │
   │ condition, timing │        │  per workflow          │
   │ template_id       │        └──────────────────────┘
   │ sent_flag, stopIf │
   └──────┬───────────┘
          │ cron evaluates
          ▼
   ┌──────────────────┐
   │   master_data     │
   │   (PostgreSQL)    │
   │   READ ← → WRITE │
   └──────────────────┘
```

## Entity Hierarchy

```
WorkflowItem (top level)
├── key (UUID)
├── name, icon, status
├── PipelineStep[] ── step configs (timing, condition, template refs)
├── PipelineTab[]  ── filter tabs for the pipeline view
├── PipelineStat[] ── stat cards (metric computation)
├── PipelineColumn[] ── visible columns in table
├── Node[]         ── React Flow nodes on canvas
└── Edge[]         ── React Flow edges connecting nodes
```

### Node Types
Setiap node di canvas memiliki `type` dan `category`:

| Category    | Description                  | Examples                                    |
|-------------|------------------------------|---------------------------------------------|
| `trigger`   | Entry point / event          | Lead Created, BD Meeting Done, Ticket Created |
| `action`    | Send message / update data   | WA Blast, Email, Invoice, Stage Transition  |
| `condition` | Branch / decision            | Reply Check, Qualified?, Resolved?, SLA     |
| `delay`     | Wait / timing control        | Wait SLA (2 jam), Snooze Resume             |
| `zone`      | Visual grouping only         | Phase zones (P0, P1, P2...)                 |

### Node Data Fields (JSONB `data`)
```json
{
  "category": "action",
  "label": "P0.1 Welcome",
  "icon": "emoji",
  "description": "D+0~5 welcome message",
  "templateId": "TPL-OB-WELCOME",
  "triggerId": "Onboarding_Welcome",
  "timing": "D+0 to D+5",
  "condition": "days_since_activation BETWEEN 0 AND 5 AND onboarding_sent = FALSE",
  "stopIf": "",
  "sentFlag": "onboarding_sent"
}
```

> **FE emits two timing formats** since the node editor added Indonesian builders:
> - Legacy (from Excel seeds): `D+0 to D+5`, `H-120`, `D+60`
> - New (from `TimingBuilder` UX): `5 Hari Setelah`, `0-5 Hari Setelah`, `120 Hari Sebelum`
>
> Backend cron parser must handle both. See `05-cron-engine.md` → "Timing Format Parser (dual-format)".
>
> **FE `condition` / `stopIf`** come from the structured `ConditionBuilder` / `StopIfBuilder` widgets. The builders emit the same SQL-like grammar the Excel seeds use — no format change — but constrained to a known catalog of 21 fields + 7 operators (see `05-cron-engine.md` → "DSL Field Catalog").

## Relationship to Master Data

Workflow Engine is the **consumer and writer** of Master Data:

```
READ flow:
  Cron → query master_data WHERE stage = X AND bot_active = TRUE
       → for each record, evaluate all nodes in the relevant workflow
       → node.condition checks core + custom fields

WRITE flow:
  After action executed → WRITE sent_flag, status changes, stage transitions
                        → log to action_logs
```

### Stage Routing
Master Data `stage` determines which workflow pipeline processes a record:

| Stage      | Pipeline           | Phase            |
|------------|--------------------|------------------|
| `LEAD`     | SDR Lead Outreach  | P1-P4 + Seasonal |
| `PROSPECT` | BD Deal Closing    | P1-P3.5          |
| `CLIENT`   | AE Client Lifecycle| P0-P6            |
| `CLIENT`   | CS Customer Support| Event-driven     |

### Stage Transitions (cross-pipeline handoff)
```
LEAD ──────► PROSPECT  (SDR qualifies → SDR_QUALIFY_HANDOFF)
PROSPECT ──► CLIENT    (BD payment confirmed → BD_PAYMENT_HANDOFF)
CLIENT ────► CLIENT    (AE renewed → cycle reset P0)
PROSPECT ──► LEAD      (BD dormant D+90 → BD_DORMANT_TO_SDR, segment=RECYCLED)
LEAD ──────► DORMANT   (SDR nurture_count >= 3 → SDR_NURTURE_TO_DORMANT)
```

## Default Workflows (Seed Data)

The system ships with 4 pre-built workflows:

| UUID                                   | Name                 | Stage Filter        | Nodes | Phases              |
|----------------------------------------|----------------------|---------------------|-------|---------------------|
| `4fc22c98-1e3b-4901-aa86-9f81b33354d2`| SDR Lead Outreach    | LEAD, DORMANT       | ~25   | P1-P4 + Seasonal    |
| `0c85261e-277c-4143-93b3-bb6714eaff08`| BD Deal Closing      | PROSPECT            | ~13   | P1-P3.5             |
| `406e6b25-37f6-4531-aade-aa42df2d52a3`| AE Client Lifecycle  | CLIENT              | ~25   | P0-P6               |
| `01400f6a-cdc9-43a0-8409-b96e316bec91`| CS Customer Support  | CLIENT              | ~8    | CS + ESC            |

## Data Master I/O per Pipeline

### SDR Pipeline
- **READ**: Stage, Company_Name, PIC_WA, PIC_Email, call_date, channel_availability, all wa_hX_sent flags, reply_wa, Bot_Active, lead_segment, HC_Size
- **WRITE**: wa_hX_sent flags, sequence_status, Stage transitions (LEAD->PROSPECT, LEAD->DORMANT)

### BD Pipeline
- **READ**: Stage, Company_Name, PIC_WA, call_date, dX_sent flags, sequence_status, closing_status, Payment_Status, Bot_Active, invoice fields
- **WRITE**: dX_sent flags, sequence_status, closing_status, Stage transitions (PROSPECT->CLIENT, PROSPECT->LEAD recycle)

### AE Pipeline
- **READ**: Stage, Company_Name, Bot_Active, Days_to_Expiry, Contract_End, Payment_Status, all phase sent flags, NPS_Score, cross_sell flags
- **WRITE**: Phase sent flags, Payment_Status, renewed, Bot_Active, invoice_created

### CS Pipeline
- **READ**: Stage, Company_ID, Company_Name, PIC_Name, PIC_WA, ticket fields
- **WRITE**: ticket_ack_sent, assigned_agent, sla_breached, CSAT_Score, Risk_Flag, Last_Interaction_Date
