# Checker-Maker (Approval) System

## Overview

Checker-maker is a dual-control approval mechanism for high-risk operations. A **maker** initiates the request, and a separate **checker** reviews and approves (or rejects) it before the system executes the actual change. This prevents accidental or unauthorized modifications to financial data, access control, and critical system configurations.

### Why Needed

- **Financial safety** — invoice creation/payment marking requires a second pair of eyes
- **Data integrity** — bulk imports and record deletions are irreversible
- **Access control** — role changes and member management affect security boundaries
- **Business continuity** — automation rules and integration keys impact live operations

### Operations Requiring Approval

| # | Operation | Category | Risk |
|---|-----------|----------|------|
| 1 | Mark Invoice as Paid | Financial | Incorrect payment status affects revenue tracking |
| 2 | Create Invoice | Financial | Invoices sent to clients have legal/financial implications |
| 3 | Bulk Import Master Data | Data Integrity | Mass data changes can corrupt the entire dataset |
| 4 | Delete Client Record | Data Loss | Permanent deletion of client history |
| 5 | Change Role/Permission | Access Control | Privilege escalation or accidental lockout |
| 6 | Invite/Remove Member | Access Control | Unauthorized access or loss of team access |
| 7 | Stage Transition (manual) | Business Process | Moving clients between stages affects pipeline metrics |
| 8 | Activate/Deactivate Automation Rule | Customer Impact | Automations send messages to real clients |
| 9 | Integration API Key Change | System Outage | Wrong key breaks WA, Telegram, Paper.id, etc. |
| 10 | Collection Schema Change | Data Integrity | Adding/removing/renaming fields can misinterpret existing collection records |

---

## Database Schema

### Enum Type

```sql
CREATE TYPE approval_request_type AS ENUM (
  'mark_invoice_paid',
  'create_invoice',
  'bulk_import_master_data',
  'delete_client_record',
  'change_role_permission',
  'invite_remove_member',
  'stage_transition',
  'toggle_automation_rule',
  'integration_api_key_change',
  'collection_schema_change'
);

CREATE TYPE approval_status AS ENUM (
  'pending',
  'approved',
  'rejected',
  'expired'
);
```

### Table: `approval_requests`

```sql
CREATE TABLE approval_requests (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     UUID NOT NULL REFERENCES workspaces(id),

  -- What
  request_type     approval_request_type NOT NULL,
  payload          JSONB NOT NULL,           -- the actual data to apply on approval
  description      TEXT NOT NULL,            -- human-readable summary, e.g. "Mark INV-2026-042 as Paid"

  -- Maker (initiator)
  maker_email      VARCHAR(255) NOT NULL,
  maker_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  -- Checker (approver/rejector)
  checker_email    VARCHAR(255),
  checker_at       TIMESTAMPTZ,
  rejection_reason TEXT,

  -- Status
  status           approval_status NOT NULL DEFAULT 'pending',
  expires_at       TIMESTAMPTZ NOT NULL,     -- maker_at + 72 hours
  applied_at       TIMESTAMPTZ,              -- when the change was actually executed

  -- Meta
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Query patterns: list pending by workspace, find by id, count pending
CREATE INDEX idx_ar_workspace_status ON approval_requests(workspace_id, status);
CREATE INDEX idx_ar_workspace_created ON approval_requests(workspace_id, created_at DESC);
CREATE INDEX idx_ar_expires ON approval_requests(expires_at) WHERE status = 'pending';
CREATE INDEX idx_ar_maker ON approval_requests(maker_email, workspace_id);
CREATE INDEX idx_ar_checker ON approval_requests(checker_email, workspace_id);
```

### Payload Examples (JSONB)

Each `request_type` has a defined payload shape:

```jsonc
// mark_invoice_paid
{
  "invoice_id": "uuid-xxx",
  "invoice_number": "INV-DE-2026-042",
  "amount": 25000000,
  "payment_method": "bank_transfer",
  "paid_at": "2026-04-12T10:00:00Z"
}

// create_invoice
{
  "client_record_id": "uuid-xxx",
  "client_name": "PT Maju Jaya",
  "items": [
    { "description": "Enterprise Plan - Q2 2026", "quantity": 1, "unit_price": 25000000 }
  ],
  "due_date": "2026-05-12",
  "currency": "IDR",
  "notes": "Payment terms: NET 30"
}

// bulk_import_master_data
{
  "file_name": "import-april-2026.csv",
  "file_url": "https://storage.example.com/uploads/xxx.csv",
  "row_count": 1540,
  "columns_mapped": { "Company_Name": "A", "Phone": "B", "Email": "C" },
  "overwrite_existing": false
}

// delete_client_record
{
  "record_id": "uuid-xxx",
  "company_name": "PT Example Corp",
  "stage": "LEAD",
  "reason": "Duplicate record"
}

// change_role_permission
{
  "target_user_email": "john@company.com",
  "current_role": "member",
  "new_role": "admin",
  "permissions_changed": ["approve", "manage_members", "manage_integrations"]
}

// invite_remove_member
{
  "action": "invite",  // or "remove"
  "target_email": "newperson@company.com",
  "role": "member",
  "reason": "New sales hire"
}

// stage_transition
{
  "record_id": "uuid-xxx",
  "company_name": "PT Maju Jaya",
  "from_stage": "PROSPECT",
  "to_stage": "CLIENT",
  "reason": "Contract signed"
}

// toggle_automation_rule
{
  "rule_id": "uuid-xxx",
  "rule_name": "Auto follow-up after 7 days inactive",
  "action": "activate",  // or "deactivate"
  "affected_record_count": 342
}

// integration_api_key_change
{
  "provider": "haloai",  // haloai | telegram | paperid
  "field_changed": "api_key",
  "has_existing_key": true,
  "reason": "Key rotation — old key compromised"
}

// collection_schema_change
{
  "collection_id": "uuid-xxx",
  "collection_slug": "competitor-notes",
  "collection_name": "Competitor Notes",
  "record_count": 42,               // existing records that will be affected
  "changes": [
    { "op": "add",    "field_key": "tier",      "field_label": "Tier", "type": "enum", "options": ["A","B","C"], "required": false },
    { "op": "remove", "field_key": "legacy_id", "field_label": "Legacy ID" },
    { "op": "rename", "field_key": "notes",     "new_key": "internal_notes", "new_label": "Internal Notes" },
    { "op": "retype", "field_key": "rating",    "old_type": "text", "new_type": "number" }
  ],
  "reason": "Add Tier field for competitor ranking; remove obsolete legacy_id."
}
```

---

## API Endpoints

All endpoints scoped to `/{workspace_id}`.

### POST `/approvals`

Maker creates a new approval request. The actual operation is NOT executed — it is stored as a pending request.

```
Headers:
  Authorization: Bearer {token}

Request:
{
  "request_type": "mark_invoice_paid",
  "payload": { ... },
  "description": "Mark INV-DE-2026-042 as Paid (Rp 25.000.000)"
}

Response 201:
{
  "id": "uuid-xxx",
  "request_type": "mark_invoice_paid",
  "status": "pending",
  "description": "Mark INV-DE-2026-042 as Paid (Rp 25.000.000)",
  "maker_email": "alice@company.com",
  "maker_at": "2026-04-12T10:00:00Z",
  "expires_at": "2026-04-15T10:00:00Z",
  "payload": { ... }
}
```

Backend validates the payload shape matches the `request_type` before saving.

### GET `/approvals`

List approval requests for the workspace. Supports filtering and pagination.

```
Query params:
  ?status=pending              // pending | approved | rejected | expired (optional, default: all)
  ?request_type=create_invoice // optional filter by type
  ?page=1&per_page=20          // pagination

Response 200:
{
  "data": [
    {
      "id": "uuid-xxx",
      "request_type": "mark_invoice_paid",
      "status": "pending",
      "description": "Mark INV-DE-2026-042 as Paid (Rp 25.000.000)",
      "maker_email": "alice@company.com",
      "maker_at": "2026-04-12T10:00:00Z",
      "expires_at": "2026-04-15T10:00:00Z"
    }
  ],
  "total": 42,
  "page": 1,
  "per_page": 20
}
```

### GET `/approvals/{id}`

Full detail including payload.

```
Response 200:
{
  "id": "uuid-xxx",
  "request_type": "mark_invoice_paid",
  "status": "pending",
  "description": "Mark INV-DE-2026-042 as Paid (Rp 25.000.000)",
  "payload": { "invoice_id": "uuid-xxx", "amount": 25000000, ... },
  "maker_email": "alice@company.com",
  "maker_at": "2026-04-12T10:00:00Z",
  "checker_email": null,
  "checker_at": null,
  "rejection_reason": null,
  "expires_at": "2026-04-15T10:00:00Z",
  "applied_at": null
}
```

### PUT `/approvals/{id}/approve`

Checker approves the request. Backend executes the actual change atomically within the same transaction.

```
Request:
{}   // no body needed; checker identity comes from auth token

Response 200:
{
  "id": "uuid-xxx",
  "status": "approved",
  "checker_email": "bob@company.com",
  "checker_at": "2026-04-12T14:30:00Z",
  "applied_at": "2026-04-12T14:30:00Z"
}

Error 403: { "error": "Cannot approve your own request" }
Error 403: { "error": "You do not have the 'approve' permission" }
Error 409: { "error": "Request is no longer pending (current status: expired)" }
```

### PUT `/approvals/{id}/reject`

Checker rejects the request with a mandatory reason.

```
Request:
{
  "reason": "Amount does not match the signed contract. Should be Rp 20.000.000."
}

Response 200:
{
  "id": "uuid-xxx",
  "status": "rejected",
  "checker_email": "bob@company.com",
  "checker_at": "2026-04-12T14:30:00Z",
  "rejection_reason": "Amount does not match the signed contract. Should be Rp 20.000.000."
}

Error 400: { "error": "Rejection reason is required" }
Error 403: { "error": "Cannot reject your own request" }
Error 409: { "error": "Request is no longer pending" }
```

### GET `/approvals/pending/count`

Returns the count of pending requests for the workspace. Used for badge/notification count in the UI.

```
Response 200:
{
  "count": 5
}
```

---

## Go Models & Service

### Models

```go
type ApprovalRequestType string

const (
	ApprovalMarkInvoicePaid     ApprovalRequestType = "mark_invoice_paid"
	ApprovalCreateInvoice       ApprovalRequestType = "create_invoice"
	ApprovalBulkImportMaster    ApprovalRequestType = "bulk_import_master_data"
	ApprovalDeleteClientRecord  ApprovalRequestType = "delete_client_record"
	ApprovalChangeRolePermission ApprovalRequestType = "change_role_permission"
	ApprovalInviteRemoveMember  ApprovalRequestType = "invite_remove_member"
	ApprovalStageTransition     ApprovalRequestType = "stage_transition"
	ApprovalToggleAutomation    ApprovalRequestType = "toggle_automation_rule"
	ApprovalIntegrationKeyChange ApprovalRequestType = "integration_api_key_change"
	ApprovalCollectionSchemaChange ApprovalRequestType = "collection_schema_change"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"
)

type ApprovalRequest struct {
	ID              uuid.UUID           `json:"id" db:"id"`
	WorkspaceID     uuid.UUID           `json:"workspace_id" db:"workspace_id"`
	RequestType     ApprovalRequestType `json:"request_type" db:"request_type"`
	Payload         json.RawMessage     `json:"payload" db:"payload"`
	Description     string              `json:"description" db:"description"`
	MakerEmail      string              `json:"maker_email" db:"maker_email"`
	MakerAt         time.Time           `json:"maker_at" db:"maker_at"`
	CheckerEmail    *string             `json:"checker_email" db:"checker_email"`
	CheckerAt       *time.Time          `json:"checker_at" db:"checker_at"`
	RejectionReason *string             `json:"rejection_reason" db:"rejection_reason"`
	Status          ApprovalStatus      `json:"status" db:"status"`
	ExpiresAt       time.Time           `json:"expires_at" db:"expires_at"`
	AppliedAt       *time.Time          `json:"applied_at" db:"applied_at"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at" db:"updated_at"`
}

type CreateApprovalInput struct {
	RequestType ApprovalRequestType `json:"request_type" validate:"required"`
	Payload     json.RawMessage     `json:"payload" validate:"required"`
	Description string              `json:"description" validate:"required,max=500"`
}

type RejectApprovalInput struct {
	Reason string `json:"reason" validate:"required,min=5,max=1000"`
}
```

### Service

```go
type ApprovalService struct {
	db              *sqlx.DB
	invoiceSvc      InvoiceService
	masterDataSvc   MasterDataService
	memberSvc       MemberService
	roleSvc         RoleService
	automationSvc   AutomationService
	integrationSvc  IntegrationService
	pipelineSvc     PipelineService
	collectionSvc   CollectionService
	notificationSvc NotificationService
}

// Create — maker initiates a request. Validates payload shape, stores as pending.
func (s *ApprovalService) Create(ctx context.Context, wsID uuid.UUID, makerEmail string, input CreateApprovalInput) (*ApprovalRequest, error) {
	// 1. Validate payload matches request_type schema
	if err := validatePayload(input.RequestType, input.Payload); err != nil {
		return nil, fmt.Errorf("invalid payload for %s: %w", input.RequestType, err)
	}

	now := time.Now()
	req := &ApprovalRequest{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		RequestType: input.RequestType,
		Payload:     input.Payload,
		Description: input.Description,
		MakerEmail:  makerEmail,
		MakerAt:     now,
		Status:      ApprovalPending,
		ExpiresAt:   now.Add(72 * time.Hour),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	query := `INSERT INTO approval_requests
		(id, workspace_id, request_type, payload, description, maker_email, maker_at, status, expires_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	_, err := s.db.ExecContext(ctx, query,
		req.ID, req.WorkspaceID, req.RequestType, req.Payload, req.Description,
		req.MakerEmail, req.MakerAt, req.Status, req.ExpiresAt, req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert approval_request: %w", err)
	}

	// Notify users with 'approve' permission
	s.notificationSvc.NotifyApprovers(ctx, wsID, req)

	return req, nil
}

// Approve — checker approves and the system applies the change in one transaction.
func (s *ApprovalService) Approve(ctx context.Context, wsID uuid.UUID, requestID uuid.UUID, checkerEmail string) (*ApprovalRequest, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Lock the row
	var req ApprovalRequest
	err = tx.GetContext(ctx, &req,
		"SELECT * FROM approval_requests WHERE id = $1 AND workspace_id = $2 FOR UPDATE", requestID, wsID)
	if err != nil {
		return nil, fmt.Errorf("approval request not found: %w", err)
	}

	// 2. Validate status
	if req.Status != ApprovalPending {
		return nil, fmt.Errorf("request is no longer pending (current status: %s)", req.Status)
	}

	// 3. Check expiry
	if time.Now().After(req.ExpiresAt) {
		tx.ExecContext(ctx, "UPDATE approval_requests SET status = 'expired', updated_at = NOW() WHERE id = $1", requestID)
		tx.Commit()
		return nil, fmt.Errorf("request has expired")
	}

	// 4. Maker != Checker
	if req.MakerEmail == checkerEmail {
		return nil, fmt.Errorf("cannot approve your own request")
	}

	// 5. Apply the actual change (within same transaction)
	if err := applyApproval(ctx, tx, s, &req); err != nil {
		return nil, fmt.Errorf("failed to apply approval: %w", err)
	}

	// 6. Update approval record
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE approval_requests
		 SET status = 'approved', checker_email = $1, checker_at = $2, applied_at = $2, updated_at = $2
		 WHERE id = $3`,
		checkerEmail, now, requestID)
	if err != nil {
		return nil, fmt.Errorf("update approval status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit approval: %w", err)
	}

	req.Status = ApprovalApproved
	req.CheckerEmail = &checkerEmail
	req.CheckerAt = &now
	req.AppliedAt = &now

	// Notify maker that request was approved
	s.notificationSvc.NotifyMaker(ctx, &req, "approved")

	return &req, nil
}

// Reject — checker rejects with a reason.
func (s *ApprovalService) Reject(ctx context.Context, wsID uuid.UUID, requestID uuid.UUID, checkerEmail string, input RejectApprovalInput) (*ApprovalRequest, error) {
	var req ApprovalRequest
	err := s.db.GetContext(ctx, &req,
		"SELECT * FROM approval_requests WHERE id = $1 AND workspace_id = $2", requestID, wsID)
	if err != nil {
		return nil, fmt.Errorf("approval request not found: %w", err)
	}

	if req.Status != ApprovalPending {
		return nil, fmt.Errorf("request is no longer pending (current status: %s)", req.Status)
	}

	if req.MakerEmail == checkerEmail {
		return nil, fmt.Errorf("cannot reject your own request")
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		`UPDATE approval_requests
		 SET status = 'rejected', checker_email = $1, checker_at = $2, rejection_reason = $3, updated_at = $2
		 WHERE id = $4 AND status = 'pending'`,
		checkerEmail, now, input.Reason, requestID)
	if err != nil {
		return nil, fmt.Errorf("update rejection: %w", err)
	}

	req.Status = ApprovalRejected
	req.CheckerEmail = &checkerEmail
	req.CheckerAt = &now
	req.RejectionReason = &input.Reason

	// Notify maker that request was rejected
	s.notificationSvc.NotifyMaker(ctx, &req, "rejected")

	return &req, nil
}
```

### Apply Function

`applyApproval` is the dispatcher that executes the actual change based on `request_type`. It runs within the approval transaction, ensuring atomicity.

```go
func applyApproval(ctx context.Context, tx *sqlx.Tx, s *ApprovalService, req *ApprovalRequest) error {
	switch req.RequestType {

	case ApprovalMarkInvoicePaid:
		var p struct {
			InvoiceID     uuid.UUID `json:"invoice_id"`
			Amount        int64     `json:"amount"`
			PaymentMethod string    `json:"payment_method"`
			PaidAt        time.Time `json:"paid_at"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE invoices SET payment_status = 'Paid', paid_at = $1, payment_method = $2, updated_at = NOW()
			 WHERE id = $3 AND workspace_id = $4`,
			p.PaidAt, p.PaymentMethod, p.InvoiceID, req.WorkspaceID)
		return err

	case ApprovalCreateInvoice:
		var p struct {
			ClientRecordID uuid.UUID       `json:"client_record_id"`
			ClientName     string          `json:"client_name"`
			Items          json.RawMessage `json:"items"`
			DueDate        string          `json:"due_date"`
			Currency       string          `json:"currency"`
			Notes          string          `json:"notes"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		// Insert into invoices table within the transaction
		_, err := tx.ExecContext(ctx,
			`INSERT INTO invoices (id, workspace_id, client_record_id, client_name, items, due_date, currency, notes, payment_status, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'Unpaid', NOW(), NOW())`,
			uuid.New(), req.WorkspaceID, p.ClientRecordID, p.ClientName, p.Items, p.DueDate, p.Currency, p.Notes)
		return err

	case ApprovalBulkImportMaster:
		var p struct {
			FileURL           string          `json:"file_url"`
			RowCount          int             `json:"row_count"`
			ColumnsMapped     json.RawMessage `json:"columns_mapped"`
			OverwriteExisting bool            `json:"overwrite_existing"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		// Enqueue async import job — the actual import runs as a background task
		// because it may take minutes for large files. The approval just unblocks it.
		_, err := tx.ExecContext(ctx,
			`INSERT INTO import_jobs (id, workspace_id, file_url, row_count, columns_mapped, overwrite_existing, status, approved_by_request_id, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7, NOW())`,
			uuid.New(), req.WorkspaceID, p.FileURL, p.RowCount, p.ColumnsMapped, p.OverwriteExisting, req.ID)
		return err

	case ApprovalDeleteClientRecord:
		var p struct {
			RecordID uuid.UUID `json:"record_id"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		// Soft-delete: mark as deleted, retain data for 30 days
		_, err := tx.ExecContext(ctx,
			`UPDATE master_data SET deleted_at = NOW(), updated_at = NOW()
			 WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`,
			p.RecordID, req.WorkspaceID)
		return err

	case ApprovalChangeRolePermission:
		var p struct {
			TargetUserEmail string   `json:"target_user_email"`
			NewRole         string   `json:"new_role"`
			Permissions     []string `json:"permissions_changed"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE workspace_members SET role = $1, updated_at = NOW()
			 WHERE workspace_id = $2 AND email = $3`,
			p.NewRole, req.WorkspaceID, p.TargetUserEmail)
		return err

	case ApprovalInviteRemoveMember:
		var p struct {
			Action      string `json:"action"` // "invite" or "remove"
			TargetEmail string `json:"target_email"`
			Role        string `json:"role"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		if p.Action == "invite" {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO workspace_members (id, workspace_id, email, role, status, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, 'invited', NOW(), NOW())
				 ON CONFLICT (workspace_id, email) DO NOTHING`,
				uuid.New(), req.WorkspaceID, p.TargetEmail, p.Role)
			return err
		}
		// remove
		_, err := tx.ExecContext(ctx,
			`UPDATE workspace_members SET status = 'removed', updated_at = NOW()
			 WHERE workspace_id = $1 AND email = $2`,
			req.WorkspaceID, p.TargetEmail)
		return err

	case ApprovalStageTransition:
		var p struct {
			RecordID uuid.UUID `json:"record_id"`
			ToStage  string    `json:"to_stage"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE master_data SET stage = $1, updated_at = NOW()
			 WHERE id = $2 AND workspace_id = $3`,
			p.ToStage, p.RecordID, req.WorkspaceID)
		return err

	case ApprovalToggleAutomation:
		var p struct {
			RuleID uuid.UUID `json:"rule_id"`
			Action string    `json:"action"` // "activate" or "deactivate"
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		active := p.Action == "activate"
		_, err := tx.ExecContext(ctx,
			`UPDATE automation_rules SET active = $1, updated_at = NOW()
			 WHERE id = $2 AND workspace_id = $3`,
			active, p.RuleID, req.WorkspaceID)
		return err

	case ApprovalIntegrationKeyChange:
		// Integration key changes are applied via the IntegrationService
		// because credentials are encrypted. We store the encrypted value in payload
		// and the service handles decryption/re-encryption.
		var p struct {
			Provider     string `json:"provider"`
			FieldChanged string `json:"field_changed"`
		}
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return err
		}
		// The actual new key value is stored encrypted in payload.new_value
		// IntegrationService.ApplyKeyChange reads it and updates workspace_integrations
		return s.integrationSvc.ApplyKeyChange(ctx, tx, req.WorkspaceID, req.Payload)

	case ApprovalCollectionSchemaChange:
		// Collection schema changes are applied via the CollectionService.
		// It dispatches per-op (add/remove/rename/retype) and handles value migration
		// inside collection_records.data (JSONB) in the same transaction.
		return s.collectionSvc.ApplySchemaChange(ctx, tx, req.WorkspaceID, req.Payload)

	default:
		return fmt.Errorf("unknown approval request type: %s", req.RequestType)
	}
}
```

---

## Rules

### 1. Maker Cannot Approve Own Request

The checker (approver/rejector) **must be a different person** from the maker. The backend enforces this by comparing `maker_email` with the authenticated user's email. This is checked in both `Approve` and `Reject` methods.

### 2. Checker Must Have 'approve' Permission

Only users whose role includes the `approve` permission can act as checkers. The API handler middleware checks this before calling `ApprovalService.Approve` or `ApprovalService.Reject`:

```go
func (h *ApprovalHandler) handleApprove(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	wsID := workspace.IDFromContext(r.Context())

	// Check 'approve' permission
	if !h.roleSvc.HasPermission(r.Context(), wsID, user.Email, "approve") {
		http.Error(w, `{"error":"You do not have the 'approve' permission"}`, http.StatusForbidden)
		return
	}

	// ... proceed with approval
}
```

### 3. Expiry After 72 Hours

Pending requests expire automatically after 72 hours. Two mechanisms enforce this:

**a) On-read check** — when fetching a pending request, the backend checks `expires_at`:
```go
if req.Status == ApprovalPending && time.Now().After(req.ExpiresAt) {
    // Mark as expired
    s.db.ExecContext(ctx, "UPDATE approval_requests SET status = 'expired', updated_at = NOW() WHERE id = $1", req.ID)
    req.Status = ApprovalExpired
}
```

**b) Cron job** — a scheduled job runs every hour to bulk-expire stale requests:
```sql
UPDATE approval_requests
SET status = 'expired', updated_at = NOW()
WHERE status = 'pending' AND expires_at < NOW();
```

### 4. Atomic Application

Approval and the actual data change happen in the **same database transaction**. If the data change fails (e.g., the invoice no longer exists), the approval is rolled back too. The request remains `pending` and the checker can retry.

### 5. Audit Trail

Every approval request is itself an audit record. The `approval_requests` table captures who requested what, who approved/rejected, when, and the exact payload. Additionally, the `action_logs` table should log the applied change with a reference to the approval request ID:

```go
// After successful apply within the transaction
tx.ExecContext(ctx,
    `INSERT INTO action_logs (id, workspace_id, action, entity_type, entity_id, performed_by, metadata, created_at)
     VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
    uuid.New(), req.WorkspaceID,
    "approval_applied",
    string(req.RequestType),
    req.ID,
    checkerEmail,
    fmt.Sprintf(`{"approval_id":"%s","maker":"%s"}`, req.ID, req.MakerEmail),
)
```

---

## Integration Examples

Each of the 10 operations creates an approval request instead of executing directly. Below shows how existing service calls are replaced.

### 1. Mark Invoice as Paid

**Before (direct execution):**
```go
func (h *InvoiceHandler) markAsPaid(w http.ResponseWriter, r *http.Request) {
    // Directly updates invoice to Paid
    h.invoiceSvc.MarkAsPaid(ctx, wsID, invoiceID, paidAt, paymentMethod)
}
```

**After (via approval):**
```go
func (h *InvoiceHandler) markAsPaid(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    wsID := workspace.IDFromContext(r.Context())

    payload, _ := json.Marshal(map[string]interface{}{
        "invoice_id":     invoiceID,
        "invoice_number": invoice.Number,
        "amount":         invoice.TotalAmount,
        "payment_method": input.PaymentMethod,
        "paid_at":        input.PaidAt,
    })

    req, err := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalMarkInvoicePaid,
        Payload:     payload,
        Description: fmt.Sprintf("Mark %s as Paid (Rp %s)", invoice.Number, formatCurrency(invoice.TotalAmount)),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 2. Create Invoice

```go
func (h *InvoiceHandler) create(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    payload, _ := json.Marshal(input) // input is the CreateInvoiceInput struct

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalCreateInvoice,
        Payload:     payload,
        Description: fmt.Sprintf("Create invoice for %s — %s %s", input.ClientName, input.Currency, formatCurrency(total)),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 3. Bulk Import Master Data

```go
func (h *ImportHandler) bulkImport(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    // File already uploaded to storage, we just have the URL and metadata

    payload, _ := json.Marshal(map[string]interface{}{
        "file_name":          uploadedFile.Name,
        "file_url":           uploadedFile.URL,
        "row_count":          rowCount,
        "columns_mapped":     columnMapping,
        "overwrite_existing": input.OverwriteExisting,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalBulkImportMaster,
        Payload:     payload,
        Description: fmt.Sprintf("Bulk import %d records from %s", rowCount, uploadedFile.Name),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 4. Delete Client Record

```go
func (h *MasterDataHandler) deleteRecord(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    record, _ := h.masterDataSvc.GetByID(ctx, wsID, recordID)

    payload, _ := json.Marshal(map[string]interface{}{
        "record_id":    record.ID,
        "company_name": record.CompanyName,
        "stage":        record.Stage,
        "reason":       input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalDeleteClientRecord,
        Payload:     payload,
        Description: fmt.Sprintf("Delete client record: %s (%s)", record.CompanyName, record.Stage),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 5. Change Role/Permission

```go
func (h *MemberHandler) changeRole(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    currentMember, _ := h.memberSvc.Get(ctx, wsID, input.TargetEmail)

    payload, _ := json.Marshal(map[string]interface{}{
        "target_user_email":   input.TargetEmail,
        "current_role":        currentMember.Role,
        "new_role":            input.NewRole,
        "permissions_changed": diffPermissions(currentMember.Role, input.NewRole),
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalChangeRolePermission,
        Payload:     payload,
        Description: fmt.Sprintf("Change %s role: %s → %s", input.TargetEmail, currentMember.Role, input.NewRole),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 6. Invite/Remove Member

```go
func (h *MemberHandler) inviteMember(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())

    payload, _ := json.Marshal(map[string]interface{}{
        "action":       "invite",
        "target_email": input.Email,
        "role":         input.Role,
        "reason":       input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalInviteRemoveMember,
        Payload:     payload,
        Description: fmt.Sprintf("Invite %s as %s", input.Email, input.Role),
    })

    respondJSON(w, http.StatusCreated, req)
}

func (h *MemberHandler) removeMember(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())

    payload, _ := json.Marshal(map[string]interface{}{
        "action":       "remove",
        "target_email": input.Email,
        "role":         "",
        "reason":       input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalInviteRemoveMember,
        Payload:     payload,
        Description: fmt.Sprintf("Remove %s from workspace", input.Email),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 7. Stage Transition (Manual)

```go
func (h *PipelineHandler) transitionStage(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    record, _ := h.masterDataSvc.GetByID(ctx, wsID, input.RecordID)

    payload, _ := json.Marshal(map[string]interface{}{
        "record_id":    record.ID,
        "company_name": record.CompanyName,
        "from_stage":   record.Stage,
        "to_stage":     input.ToStage,
        "reason":       input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalStageTransition,
        Payload:     payload,
        Description: fmt.Sprintf("Move %s: %s → %s", record.CompanyName, record.Stage, input.ToStage),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 8. Activate/Deactivate Automation Rule

```go
func (h *AutomationHandler) toggleRule(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    rule, _ := h.automationSvc.GetByID(ctx, wsID, ruleID)

    action := "activate"
    if rule.Active {
        action = "deactivate"
    }

    affectedCount, _ := h.automationSvc.CountAffectedRecords(ctx, wsID, ruleID)

    payload, _ := json.Marshal(map[string]interface{}{
        "rule_id":                ruleID,
        "rule_name":              rule.Name,
        "action":                 action,
        "affected_record_count":  affectedCount,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalToggleAutomation,
        Payload:     payload,
        Description: fmt.Sprintf("%s automation: %s (%d records affected)", strings.Title(action), rule.Name, affectedCount),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

### 10. Collection Schema Change

```go
func (h *CollectionHandler) updateSchema(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    coll, _ := h.collectionSvc.GetByID(ctx, wsID, collectionID)
    recordCount, _ := h.collectionSvc.CountRecords(ctx, collectionID)

    // input.Changes is the array of {op, field_key, ...} diff entries
    payload, _ := json.Marshal(map[string]interface{}{
        "collection_id":   coll.ID,
        "collection_slug": coll.Slug,
        "collection_name": coll.Name,
        "record_count":    recordCount,
        "changes":         input.Changes,
        "reason":          input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalCollectionSchemaChange,
        Payload:     payload,
        Description: fmt.Sprintf("Modify schema: %s (%d fields changed, %d records affected)",
            coll.Name, len(input.Changes), recordCount),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

Cosmetic-only edits (name, icon, description — no field changes) skip approval and apply directly via `PATCH /collections/{id}`.

### 9. Integration API Key Change

```go
func (h *IntegrationHandler) updateKey(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    existing, _ := h.integrationSvc.Get(ctx, wsID)

    hasExisting := false
    switch input.Provider {
    case "haloai":
        hasExisting = existing.HaloaiAPIKey != ""
    case "telegram":
        hasExisting = existing.TelegramBotToken != ""
    case "paperid":
        hasExisting = existing.PaperidAPIKey != ""
    }

    // Encrypt the new key value before storing in payload
    encryptedValue, _ := h.integrationSvc.EncryptForPayload(input.NewValue)

    payload, _ := json.Marshal(map[string]interface{}{
        "provider":         input.Provider,
        "field_changed":    input.FieldChanged,
        "has_existing_key": hasExisting,
        "new_value":        encryptedValue,  // encrypted — never store plaintext in approval payload
        "reason":           input.Reason,
    })

    req, _ := h.approvalSvc.Create(ctx, wsID, user.Email, CreateApprovalInput{
        RequestType: ApprovalIntegrationKeyChange,
        Payload:     payload,
        Description: fmt.Sprintf("Change %s %s (reason: %s)", input.Provider, input.FieldChanged, input.Reason),
    })

    respondJSON(w, http.StatusCreated, req)
}
```

---

## Frontend Considerations

The frontend should:

1. **Replace direct action buttons** with "Request Approval" for the 9 operations above
2. **Show pending badge** using `GET /approvals/pending/count` on the sidebar/header
3. **Approval inbox page** at `/dashboard/{workspace}/approvals` listing pending requests
4. **Detail modal** showing the payload diff — what will change if approved
5. **After submission** — show a toast: "Approval request submitted. A checker must approve before the change takes effect."
6. **After approval/rejection** — notify the maker via in-app notification and optionally email/Telegram
