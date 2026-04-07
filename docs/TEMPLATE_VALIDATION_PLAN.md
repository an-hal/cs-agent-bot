# Template Validation System -- Implementation Plan

## Overview

This document describes a full template validation system for the CS Agent Bot.
The system ensures that every Go-template variable (`{{.VarName}}`) used inside
stored template content is known, correctly spelled, and -- where required --
present before the template is saved or sent.

---

## Architecture Diagram

```
                    ┌─────────────────────────────┐
                    │  entity/template_variable.go │  Canonical variable registry
                    │  (37 variables total)        │
                    └──────────┬──────────────────┘
                               │
              ┌────────────────┼────────────────┐
              v                v                v
   ┌──────────────────┐ ┌──────────────┐ ┌────────────────┐
   │  template/       │ │  template/   │ │  webhook/      │
   │  validator.go    │ │  resolver.go │ │  template_     │
   │  (save-time      │ │  (runtime    │ │  handler.go    │
   │   validation)    │ │   guard)     │ │  (HTTP API)    │
   └──────────────────┘ └──────────────┘ └────────────────┘
```

---

## Step 1 -- `internal/entity/template_variable.go` (NEW FILE)

### Types

| Type / Constant | Purpose |
|---|---|
| `VariableSource` | String type alias: `"client"`, `"invoice"`, `"config"`, `"computed"` |
| `VarSourceClient`, `VarSourceInvoice`, `VarSourceConfig`, `VarSourceComputed` | Typed constants for source filtering |
| `TemplateVariable` struct | Holds `Name`, `Source`, `Required`, `Description` |

### Registry -- 37 variables

**Client (22)**
`CompanyID`, `CompanyName`, `PICName`, `PICWA`, `PICEmail`, `PICRole`,
`HCSize`, `OwnerName`, `OwnerWA`, `OwnerTelegramID`, `Segment`, `PlanType`,
`PaymentTerms`, `ContractStart`, `ContractEnd`, `ContractMonths`, `FinalPrice`,
`QuotationLink`, `NPSScore`, `UsageScore`, `ActivationDate`, `DaysRemaining`

**Invoice (6)**
`InvoiceID`, `InvoiceIssueDate`, `InvoiceDueDate`, `InvoiceAmount`,
`InvoiceAmountFormatted`, `InvoiceDaysOverdue`

**Config (4)**
`PromoDeadline`, `SurveyURL`, `CheckinFormURL`, `ReferralBenefit`

**Computed (5)**
`FinalPriceFormatted`, `CurrentDate`, `RenewalLink`, `BotWhatsApp`,
`SupportEmail`

### Helper Functions

| Function | Signature | Behaviour |
|---|---|---|
| `GetVarByName` | `(name string) (TemplateVariable, bool)` | O(1) lookup via init-built map |
| `GetVarsBySource` | `(source VariableSource) []TemplateVariable` | Returns all variables for a given source |
| `GetRequiredVars` | `() []TemplateVariable` | Returns every variable with `Required == true` |

The package-level `varByNameMap` is built in an `init()` function.

---

## Step 2 -- `internal/usecase/template/validator.go` (NEW FILE)

### Key Decisions

- Lives in the existing `template` package alongside `resolver.go`.
- Pure functions -- no struct, no interface -- so it can be called from
  anywhere (HTTP handler, unit tests, future CLI tool).

### Regex

```go
var templateVarRegex = regexp.MustCompile(`\{\{\.(\w+)\}\}`)
```

Matches `{{.CompanyName}}`, `{{.InvoiceAmount}}`, etc.

### Structs

```go
type ValidationWarning struct {
    Variable string // e.g. "CompanyName"
    Message  string // human-readable description
}

type ValidationResult struct {
    Valid           bool
    UnknownVars     []string            // vars in content not in registry
    MissingRequired []string            // required vars absent from content
    Warnings        []ValidationWarning // non-fatal advisories
}
```

### Functions

#### `ValidateContent(content string) *ValidationResult`

1. Run `templateVarRegex.FindAllStringSubmatch` to extract all `{{.X}}` names.
2. For each extracted name, check `entity.GetVarByName(name)`:
   - If not found, append to `UnknownVars`.
3. Return the result. Sets `Valid = true` only when `UnknownVars` is empty.

#### `ValidateAtSave(content, category string) *ValidationResult`

1. Call `ValidateContent(content)` for the base result.
2. Look up the category-to-required-vars mapping:

| Category | Required Variables |
|---|---|
| `renewal` | `CompanyName`, `PICName`, `ContractEnd`, `DaysRemaining` |
| `invoice` | `CompanyName`, `PICName`, `InvoiceID`, `InvoiceDueDate`, `InvoiceAmountFormatted` |
| `overdue` | `CompanyName`, `PICName`, `InvoiceID`, `InvoiceDaysOverdue` |
| `checkin` | `CompanyName`, `PICName`, `OwnerName`, `CheckinFormURL` |
| `nps` | `CompanyName`, `PICName`, `SurveyURL` |
| `escalation` | `CompanyName`, `PICName`, `OwnerName` |
| `referral` | `CompanyName`, `PICName`, `ReferralBenefit` |
| `health` | `CompanyName`, `PICName`, `OwnerName` |
| `cross_sell` | `CompanyName`, `PICName` |
| `expansion` | `CompanyName`, `PICName` |

3. For each required variable, if it does NOT appear in the extracted set,
   append it to `MissingRequired`.
4. Set `Valid = false` if either `UnknownVars` or `MissingRequired` is non-empty.
5. Add a `ValidationWarning` for any non-required variable that is in the
   registry but not in the content (advisory only, does not fail validation).

---

## Step 3 -- Modifications to `internal/usecase/template/resolver.go`

### What already exists (DO NOT REMOVE)

- `TemplateResolver` interface with `ResolveTemplate`, `ResolveTemplateWithData`,
  `FormatMessage`.
- `templateUseCase` struct with `templateRepo`, `logger`, `tracer`.
- `buildTemplateData(client *entity.Client) map[string]interface{}` -- produces
  16 client-derived keys.

### What to ADD

#### 3a. `TemplateData` struct

A structured container that the caller assembles before calling resolve:

```go
type TemplateData struct {
    Client    *entity.Client
    Flags     *entity.ClientFlags
    Invoice   *entity.Invoice
    ConfigMap map[string]string  // key-value pairs from system_config table
}
```

#### 3b. `buildDataMapFromTemplateData(td *TemplateData) map[string]interface{}`

Method on `templateUseCase` that builds the full data map from ALL sources:

| Source | Variables |
|---|---|
| `td.Client` | All 22 client variables (including formatted dates, `DaysRemaining` computed from `ContractEnd - now`) |
| `td.Invoice` | All 6 invoice variables (including `InvoiceAmountFormatted` via `FormatPrice`, `InvoiceDaysOverdue` via `dueDate - now`) |
| `td.ConfigMap` | Maps config keys to variable names: `promo_deadline` -> `PromoDeadline`, etc. |
| Computed | `FinalPriceFormatted` (via `FormatPrice`), `CurrentDate` (`time.Now().Format(...)`) |

Every value is a string or a number -- never a nil pointer. Missing optional
sources (nil Invoice, empty ConfigMap) are handled gracefully by omitting or
zero-filling.

#### 3c. Runtime guard in `ResolveTemplateWithData`

After `tmplParsed.Execute(&buf, data)` succeeds, scan the output for any
remaining literal `{{.X}}` tokens. If found, log a warning:

```go
remaining := templateVarRegex.FindAllString(buf.String(), -1)
if len(remaining) > 0 {
    logger.Warn().
        Strs("unresolved_vars", remaining).
        Msg("Template contains unresolved variables after execution")
}
```

This does NOT fail the request -- it is an observability guard.

#### 3d. `FormatPrice` helper function

```go
func FormatPrice(amount float64) string
```

Formats a float64 into Indonesian Rupiah format, e.g.
`12_500_000.0` -> `"Rp 12.500.000"`. Uses `shopspring/decimal` (already in
go.mod) or a simple `fmt.Sprintf` with thousand-separator logic.

---

## Step 4 -- Migration Files

### `migration/20250330000010_fix_template_syntax.up.sql`

Converts legacy `[Bracket_Name]` syntax to Go-template `{{.PascalCase}}` syntax.

Key transformations (longest patterns first to avoid partial replacement):

```sql
-- Client variables
UPDATE templates SET template_content = REPLACE(template_content, '[Company_Name]', '{{.CompanyName}}') WHERE template_content LIKE '%[Company_Name]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Company_ID]', '{{.CompanyID}}') WHERE template_content LIKE '%[Company_ID]%';
UPDATE templates SET template_content = REPLACE(template_content, '[PIC_Name]', '{{.PICName}}') WHERE template_content LIKE '%[PIC_Name]%';
UPDATE templates SET template_content = REPLACE(template_content, '[PIC_WA]', '{{.PICWA}}') WHERE template_content LIKE '%[PIC_WA]%';
UPDATE templates SET template_content = REPLACE(template_content, '[PIC_Email]', '{{.PICEmail}}') WHERE template_content LIKE '%[PIC_Email]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Owner_Name]', '{{.OwnerName}}') WHERE template_content LIKE '%[Owner_Name]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Contract_Start]', '{{.ContractStart}}') WHERE template_content LIKE '%[Contract_Start]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Contract_End]', '{{.ContractEnd}}') WHERE template_content LIKE '%[Contract_End]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Final_Price]', '{{.FinalPriceFormatted}}') WHERE template_content LIKE '%[Final_Price]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Quotation_Link]', '{{.QuotationLink}}') WHERE template_content LIKE '%[Quotation_Link]%';

-- Invoice variables
UPDATE templates SET template_content = REPLACE(template_content, '[Invoice_ID]', '{{.InvoiceID}}') WHERE template_content LIKE '%[Invoice_ID]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Invoice_Amount]', '{{.InvoiceAmountFormatted}}') WHERE template_content LIKE '%[Invoice_Amount]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Due_Date]', '{{.InvoiceDueDate}}') WHERE template_content LIKE '%[Due_Date]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Days_Overdue]', '{{.InvoiceDaysOverdue}}') WHERE template_content LIKE '%[Days_Overdue]%';

-- Config variables
UPDATE templates SET template_content = REPLACE(template_content, '[Promo_Deadline]', '{{.PromoDeadline}}') WHERE template_content LIKE '%[Promo_Deadline]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Survey_URL]', '{{.SurveyURL}}') WHERE template_content LIKE '%[Survey_URL]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Checkin_Form_URL]', '{{.CheckinFormURL}}') WHERE template_content LIKE '%[Checkin_Form_URL]%';
UPDATE templates SET template_content = REPLACE(template_content, '[Referral_Benefit]', '{{.ReferralBenefit}}') WHERE template_content LIKE '%[Referral_Benefit]%';
```

### `migration/20250330000010_fix_template_syntax.down.sql`

Exact reverse of the above -- every `REPLACE` call swaps `{{.X}}` back to
`[Bracket_X]`.

---

## Step 5 -- `internal/delivery/http/webhook/template_handler.go` (NEW FILE)

### Handler struct

```go
type TemplateValidationHandler struct {
    Logger zerolog.Logger
    Tracer tracer.Tracer
}
```

No repository dependency -- the handler delegates to the pure validation
functions in `template/validator.go`.

### Constructor

```go
func NewTemplateValidationHandler(logger zerolog.Logger, tracer tracer.Tracer) *TemplateValidationHandler
```

### Request / Response types

```go
type ValidateTemplateRequest struct {
    Content  string `json:"content" validate:"required"`
    Category string `json:"category"` // optional; if present, runs category checks
}

type ValidateTemplateResponse struct {
    Valid           bool                    `json:"valid"`
    UnknownVars     []string                `json:"unknown_vars,omitempty"`
    MissingRequired []string                `json:"missing_required,omitempty"`
    Warnings        []template.ValidationWarning `json:"warnings,omitempty"`
}
```

### HTTP method: `Validate`

```go
func (h *TemplateValidationHandler) Validate(w http.ResponseWriter, r *http.Request) error
```

Flow:
1. Decode JSON body into `ValidateTemplateRequest`.
2. If `Category` is non-empty, call `template.ValidateAtSave(content, category)`.
3. Otherwise call `template.ValidateContent(content)`.
4. Return `response.StandardSuccess(w, r, 200, "Validation complete", result)`.

Follows the same pattern as `ExampleHandler.Create` -- returns `error`,
delegates error rendering to the `middleware.ErrorHandler` wrapper.

---

## Step 6 -- Route Registration in `internal/delivery/http/route.go`

### Changes to `SetupHandler`

After the existing `handoffHandler` block, add:

```go
// Template validation handler (internal tool, no auth required for MVP)
templateValidationHandler := webhook.NewTemplateValidationHandler(
    deps.Logger,
    deps.Tracer,
)
```

Then register the route under the `api` group:

```go
// Template validation
api.Handle(http.MethodPost, "/template/validate",
    middleware.ErrorHandlingMiddleware(deps.ExceptionHandler)(templateValidationHandler.Validate))
```

No new middleware is needed. The error handler wrapper follows the same pattern
used by `handoffHandler.HandleNewClient`.

No changes to `deps.go` are required because the handler only needs
`deps.Logger` and `deps.Tracer`, which are already in the Deps struct.

---

## Step 7 -- Build Verification

After all files are created:

```bash
cd /home/anhalim/dealls/cs-agent-bot
go build ./cmd/server
```

This must compile cleanly with zero errors.

---

## File Summary

| # | File | Action | Description |
|---|---|---|---|
| 1 | `internal/entity/template_variable.go` | CREATE | Variable source types, 37-entry registry, helper functions |
| 2 | `internal/usecase/template/validator.go` | CREATE | Regex, ValidationResult, ValidateContent, ValidateAtSave |
| 3 | `internal/usecase/template/resolver.go` | MODIFY | Add TemplateData struct, buildDataMapFromTemplateData, runtime guard, FormatPrice |
| 4 | `migration/20250330000010_fix_template_syntax.up.sql` | CREATE | Bracket-to-Go-template conversions |
| 5 | `migration/20250330000010_fix_template_syntax.down.sql` | CREATE | Reverse conversions |
| 6 | `internal/delivery/http/webhook/template_handler.go` | CREATE | HTTP handler for /template/validate |
| 7 | `internal/delivery/http/route.go` | MODIFY | Wire template validation route |

---

## Dependency Graph

```
entity/template_variable.go
  ^
  |--- usecase/template/validator.go
  |      ^
  |      |--- webhook/template_handler.go
  |      |      ^
  |      |      |--- route.go (registration)
  |      |
  |--- usecase/template/resolver.go (runtime guard uses same regex)
```

No external dependencies are introduced. `shopspring/decimal` is already in
`go.mod`. No new `deps.go` fields are needed.

---

## Testing Strategy (for future implementation)

| Test | Location | What it covers |
|---|---|---|
| Unit | `entity/template_variable_test.go` | Registry completeness, helper lookups |
| Unit | `usecase/template/validator_test.go` | ValidateContent with known/unknown vars, ValidateAtSave per category |
| Unit | `usecase/template/resolver_test.go` | buildDataMapFromTemplateData, FormatPrice, runtime guard logging |
| Integration | `webhook/template_handler_test.go` | HTTP round-trip for /template/validate |

---

## Variable Counts

| Source | Count | Variables |
|---|---|---|
| Client | 22 | CompanyID, CompanyName, PICName, PICWA, PICEmail, PICRole, HCSize, OwnerName, OwnerWA, OwnerTelegramID, Segment, PlanType, PaymentTerms, ContractStart, ContractEnd, ContractMonths, FinalPrice, QuotationLink, NPSScore, UsageScore, ActivationDate, DaysRemaining |
| Invoice | 6 | InvoiceID, InvoiceIssueDate, InvoiceDueDate, InvoiceAmount, InvoiceAmountFormatted, InvoiceDaysOverdue |
| Config | 4 | PromoDeadline, SurveyURL, CheckinFormURL, ReferralBenefit |
| Computed | 5 | FinalPriceFormatted, CurrentDate, RenewalLink, BotWhatsApp, SupportEmail |
| **Total** | **37** | |
