# Workflow Integration — How Nodes Use Master Data

## Setiap Workflow Node Harus:

```
1. READ dari Master Data  → get record + check conditions
2. Evaluate gates         → blacklisted? Bot_Active? isWorkingDay?
3. Check conditions       → custom_fields.nps_score >= 8? sent_flag = FALSE?
4. Execute action         → Send WA, Send Email, Update DB, Create Invoice
5. WRITE ke Master Data   → set sent_flag = TRUE, update status, log action
```

## Contoh: Node "Onboarding_Welcome" (AE P0.1)

```go
func processNode(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    // 1. READ — fields sudah di-load dari DB
    // dataRead: Stage, Company_Name, Bot_Active, onboarding_sent

    // 2. Gates
    if record.Blacklisted {
        return nil // EXIT
    }
    if !record.BotActive {
        return nil // EXIT
    }
    if !isWorkingDay(time.Now()) {
        return nil // skip, cron will retry tomorrow
    }

    // 3. Conditions
    daysSinceActivation := daysSince(record.ContractStart)
    if daysSinceActivation < 0 || daysSinceActivation > 5 {
        return nil // not in window
    }
    if record.GetCustomBool("onboarding_sent") {
        return nil // already sent
    }

    // 4. Execute — send WA via HaloAI
    err := haloai.SendTemplate(ctx, haloai.SendRequest{
        PhoneNumber: *record.PICWA,
        TemplateID:  "TPL-OB-WELCOME",
        Variables: map[string]string{
            "Company_Name": record.CompanyName,
            "link_wiki":    config.WikiURL,
        },
    })
    if err != nil {
        logAction(ctx, record, "Onboarding_Welcome", "failed", nil)
        return err
    }

    // 5. WRITE — update Master Data
    // dataWrite: onboarding_sent = TRUE
    err = repo.MergeCustomFields(ctx, record.ID, map[string]interface{}{
        "onboarding_sent": true,
    })
    if err != nil {
        return err
    }

    // Update last interaction
    repo.UpdateLastInteraction(ctx, record.ID)

    // Log action
    logAction(ctx, record, "Onboarding_Welcome", "delivered", map[string]interface{}{
        "onboarding_sent": true,
    })

    return nil
}
```

## Contoh: Stage Transition "SDR → BD Handoff"

```go
func handleSDRQualifyHandoff(ctx context.Context, repo *DataMasterRepo, record *DataMaster) error {
    // Preconditions
    if record.Stage != StageLead {
        return fmt.Errorf("expected LEAD, got %s", record.Stage)
    }

    hcSize := record.GetCustomFloat("hc_size")
    if hcSize < float64(config.MinHCSize) {
        return nil // not qualified
    }

    // Atomic stage transition
    err := repo.TransitionStage(ctx, record.ID, StageProspect, map[string]interface{}{
        "qualified_at":    time.Now().Format(time.RFC3339),
        "qualified_by":    record.OwnerName,
        "sequence_status": "ACTIVE",
    })
    if err != nil {
        return err
    }

    // Schedule BD meeting
    bookBDCalendar(ctx, record)

    // Notify BD via Telegram
    notifyTelegram(ctx, fmt.Sprintf(
        "🤝 New prospect from SDR: %s (%s) — HC: %.0f",
        record.CompanyName, record.CompanyID, hcSize,
    ))

    // Log
    logAction(ctx, record, "SDR_QUALIFY_HANDOFF", "delivered", map[string]interface{}{
        "previous_stage": "LEAD",
        "new_stage":      "PROSPECT",
    })

    return nil
}
```

## Cron Job Pattern

Workflow nodes dijalankan via cron (Google Cloud Scheduler → HTTP endpoint):

```go
// POST /api/v1/cron/evaluate
// Called every hour on working days (09:00-17:00 WIB)

func EvaluateCron(ctx context.Context, repo *DataMasterRepo) error {
    // Get all active records
    records, _, err := repo.GetByWorkspace(ctx, wsID, nil, 10000, 0)
    if err != nil {
        return err
    }

    for _, record := range records {
        // Skip gates
        if record.Blacklisted || !record.BotActive {
            continue
        }

        // Route to correct pipeline based on Stage
        switch record.Stage {
        case StageLead:
            evaluateSDR(ctx, repo, &record)
        case StageProspect:
            evaluateBD(ctx, repo, &record)
        case StageClient:
            evaluateAE(ctx, repo, &record)
            // CS doesn't need cron — ticket-driven
        }
    }

    return nil
}

func evaluateAE(ctx context.Context, repo *DataMasterRepo, record *DataMaster) {
    // Try each phase in order
    evaluateOnboarding(ctx, repo, record)      // P0
    evaluateAssessment(ctx, repo, record)       // P1
    evaluateCheckIn(ctx, repo, record)          // P2
    evaluatePromoSelling(ctx, repo, record)     // P3
    evaluateNegotiation(ctx, repo, record)      // P4
    evaluateRenewalOps(ctx, repo, record)       // P5
    evaluateOverdue(ctx, repo, record)          // P6
}
```

## Custom Fields dalam Workflow Conditions

Workflow conditions yang reference custom fields:

```go
// Core field — langsung akses struct field
if record.BotActive { ... }
if record.Stage == StageClient { ... }
if *record.DaysToExpiry <= 35 { ... }

// Custom field — akses via helper
if record.GetCustomBool("onboarding_sent") { ... }
if record.GetCustomFloat("nps_score") >= 8 { ... }
if record.GetCustomString("plan_type") == "Enterprise" { ... }

// Bulk query custom field via SQL
records, _ := repo.QueryByCustomField(ctx, wsID, "nps_score", ">=", 8)
```

## Migration: Menambah Field Baru

Kalau client ingin tambah custom field baru (misal `loyalty_tier`):

1. **POST `/master-data/field-definitions`** — define field di backend
2. Dashboard auto-render kolom baru di tabel + form
3. Workflow nodes bisa reference `custom_fields.loyalty_tier` di conditions
4. **Tidak perlu ALTER TABLE** — JSONB handles it

Kalau perlu tambah **core field** baru (jarang):

1. `ALTER TABLE master_data ADD COLUMN new_field TYPE;`
2. Update Go model struct
3. Update API response
4. Update dashboard types
