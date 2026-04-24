# Golang Models & Repository

## Models

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════════════════════════════════════════════
// DataMaster — core entity for all pipeline data
// ══════════════════════════════════════════════════════════════

type Stage string

const (
	StageLead     Stage = "LEAD"
	StageProspect Stage = "PROSPECT"
	StageClient   Stage = "CLIENT"
	StageDormant  Stage = "DORMANT"
)

type SequenceStatus string

const (
	SeqActive      SequenceStatus = "ACTIVE"
	SeqPaused      SequenceStatus = "PAUSED"
	SeqNurture     SequenceStatus = "NURTURE"
	SeqNurturePool SequenceStatus = "NURTURE_POOL"
	SeqSnoozed     SequenceStatus = "SNOOZED"
	SeqDormant     SequenceStatus = "DORMANT"
)

type RiskFlag string

const (
	RiskHigh RiskFlag = "High"
	RiskMid  RiskFlag = "Mid"
	RiskLow  RiskFlag = "Low"
	RiskNone RiskFlag = "None"
)

type DataMaster struct {
	ID          uuid.UUID `db:"id" json:"id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`

	// Core identification
	CompanyID   string `db:"company_id" json:"company_id"`
	CompanyName string `db:"company_name" json:"company_name"`
	Stage       Stage  `db:"stage" json:"stage"`

	// Contact
	PICName     *string `db:"pic_name" json:"pic_name"`
	PICNickname *string `db:"pic_nickname" json:"pic_nickname"`
	PICRole     *string `db:"pic_role" json:"pic_role"`
	PICWA       *string `db:"pic_wa" json:"pic_wa"`
	PICEmail    *string `db:"pic_email" json:"pic_email"`

	// Owner
	OwnerName       *string `db:"owner_name" json:"owner_name"`
	OwnerWA         *string `db:"owner_wa" json:"owner_wa"`
	OwnerTelegramID *string `db:"owner_telegram_id" json:"owner_telegram_id"`

	// Automation
	BotActive      bool           `db:"bot_active" json:"bot_active"`
	Blacklisted    bool           `db:"blacklisted" json:"blacklisted"`
	SequenceStatus SequenceStatus `db:"sequence_status" json:"sequence_status"`
	SnoozeUntil    *time.Time     `db:"snooze_until" json:"snooze_until"`
	SnoozeReason   *string        `db:"snooze_reason" json:"snooze_reason"`

	// Risk
	RiskFlag RiskFlag `db:"risk_flag" json:"risk_flag"`

	// Contract & Payment
	ContractStart   *time.Time `db:"contract_start" json:"contract_start"`
	ContractEnd     *time.Time `db:"contract_end" json:"contract_end"`
	ContractMonths  *int       `db:"contract_months" json:"contract_months"`
	DaysToExpiry    *int       `db:"days_to_expiry" json:"days_to_expiry"`
	PaymentStatus   string     `db:"payment_status" json:"payment_status"`
	PaymentTerms    *string    `db:"payment_terms" json:"payment_terms"`
	FinalPrice      int64      `db:"final_price" json:"final_price"`
	LastPaymentDate *time.Time `db:"last_payment_date" json:"last_payment_date"`
	Renewed         bool       `db:"renewed" json:"renewed"`

	// Interaction
	LastInteractionDate *time.Time `db:"last_interaction_date" json:"last_interaction_date"`

	// Notes
	Notes *string `db:"notes" json:"notes"`

	// ══ Custom fields — JSONB ══
	// Isi tergantung workspace. Akses via helper methods di bawah.
	CustomFields json.RawMessage `db:"custom_fields" json:"custom_fields"`

	// Meta
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// GetCustomString reads a string custom field
func (d *DataMaster) GetCustomString(key string) string {
	var m map[string]interface{}
	if err := json.Unmarshal(d.CustomFields, &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// GetCustomFloat reads a numeric custom field
func (d *DataMaster) GetCustomFloat(key string) float64 {
	var m map[string]interface{}
	if err := json.Unmarshal(d.CustomFields, &m); err != nil {
		return 0
	}
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// GetCustomBool reads a boolean custom field
func (d *DataMaster) GetCustomBool(key string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(d.CustomFields, &m); err != nil {
		return false
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// SetCustomField sets a custom field value
func (d *DataMaster) SetCustomField(key string, value interface{}) error {
	var m map[string]interface{}
	if err := json.Unmarshal(d.CustomFields, &m); err != nil {
		m = make(map[string]interface{})
	}
	m[key] = value
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	d.CustomFields = b
	return nil
}

// ══════════════════════════════════════════════════════════════
// CustomFieldDefinition — schema definisi per workspace
// ══════════════════════════════════════════════════════════════

type FieldType string

const (
	FieldText    FieldType = "text"
	FieldNumber  FieldType = "number"
	FieldDate    FieldType = "date"
	FieldBoolean FieldType = "boolean"
	FieldSelect  FieldType = "select"
	FieldURL     FieldType = "url"
	FieldEmail   FieldType = "email"
)

type CustomFieldDefinition struct {
	ID          uuid.UUID `db:"id" json:"id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`

	FieldKey    string    `db:"field_key" json:"field_key"`       // snake_case
	FieldLabel  string    `db:"field_label" json:"field_label"`   // display name
	FieldType   FieldType `db:"field_type" json:"field_type"`

	IsRequired   bool            `db:"is_required" json:"is_required"`
	DefaultValue *string         `db:"default_value" json:"default_value"`
	Placeholder  *string         `db:"placeholder" json:"placeholder"`
	Description  *string         `db:"description" json:"description"`
	Options      json.RawMessage `db:"options" json:"options"`       // for select type

	MinValue *float64 `db:"min_value" json:"min_value"`
	MaxValue *float64 `db:"max_value" json:"max_value"`
	Regex    *string  `db:"regex_pattern" json:"regex_pattern"`

	SortOrder      int  `db:"sort_order" json:"sort_order"`
	VisibleInTable bool `db:"visible_in_table" json:"visible_in_table"`
	ColumnWidth    int  `db:"column_width" json:"column_width"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ══════════════════════════════════════════════════════════════
// ActionLog — log setiap action dari workflow node
// ══════════════════════════════════════════════════════════════

type ActionLog struct {
	ID           uuid.UUID `db:"id" json:"id"`
	WorkspaceID  uuid.UUID `db:"workspace_id" json:"workspace_id"`
	DataMasterID uuid.UUID `db:"master_data_id" json:"master_data_id"`

	TriggerID  string  `db:"trigger_id" json:"trigger_id"`
	TemplateID *string `db:"template_id" json:"template_id"`

	Status  string  `db:"status" json:"status"`   // delivered, failed, escalated, manual
	Channel *string `db:"channel" json:"channel"` // whatsapp, email, telegram
	Phase   *string `db:"phase" json:"phase"`     // P0-P6, ESC

	FieldsRead    json.RawMessage `db:"fields_read" json:"fields_read"`
	FieldsWritten json.RawMessage `db:"fields_written" json:"fields_written"`

	Replied        bool    `db:"replied" json:"replied"`
	ConversationID *string `db:"conversation_id" json:"conversation_id"`

	Timestamp time.Time `db:"timestamp" json:"timestamp"`
}
```

## Repository Pattern

```go
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type DataMasterRepo struct {
	db *sqlx.DB
}

func NewDataMasterRepo(db *sqlx.DB) *DataMasterRepo {
	return &DataMasterRepo{db: db}
}

// ── CRUD ─────────────────────────────────────────────────────

func (r *DataMasterRepo) GetByWorkspace(ctx context.Context, wsID uuid.UUID, stage *string, limit, offset int) ([]DataMaster, int, error) {
	query := `SELECT * FROM master_data WHERE workspace_id = $1`
	countQuery := `SELECT COUNT(*) FROM master_data WHERE workspace_id = $1`
	args := []interface{}{wsID}

	if stage != nil {
		query += ` AND stage = $2`
		countQuery += ` AND stage = $2`
		args = append(args, *stage)
	}

	var total int
	r.db.GetContext(ctx, &total, countQuery, args...)

	query += fmt.Sprintf(` ORDER BY updated_at DESC LIMIT %d OFFSET %d`, limit, offset)

	var results []DataMaster
	err := r.db.SelectContext(ctx, &results, query, args...)
	return results, total, err
}

func (r *DataMasterRepo) GetByID(ctx context.Context, id uuid.UUID) (*DataMaster, error) {
	var dm DataMaster
	err := r.db.GetContext(ctx, &dm, `SELECT * FROM master_data WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &dm, nil
}

func (r *DataMasterRepo) Create(ctx context.Context, dm *DataMaster) error {
	query := `INSERT INTO master_data (
		workspace_id, company_id, company_name, stage,
		pic_name, pic_nickname, pic_role, pic_wa, pic_email,
		owner_name, owner_wa, owner_telegram_id,
		bot_active, blacklisted, sequence_status,
		risk_flag, contract_start, contract_end, payment_status,
		final_price, custom_fields
	) VALUES (
		:workspace_id, :company_id, :company_name, :stage,
		:pic_name, :pic_nickname, :pic_role, :pic_wa, :pic_email,
		:owner_name, :owner_wa, :owner_telegram_id,
		:bot_active, :blacklisted, :sequence_status,
		:risk_flag, :contract_start, :contract_end, :payment_status,
		:final_price, :custom_fields
	) RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, query, dm)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&dm.ID, &dm.CreatedAt, &dm.UpdatedAt)
	}
	return nil
}

// ── Custom Field Update (partial JSONB merge) ────────────────

func (r *DataMasterRepo) UpdateCustomField(ctx context.Context, id uuid.UUID, key string, value interface{}) error {
	valJSON, _ := json.Marshal(value)
	_, err := r.db.ExecContext(ctx,
		`UPDATE master_data 
		 SET custom_fields = jsonb_set(custom_fields, $1, $2)
		 WHERE id = $3`,
		fmt.Sprintf("{%s}", key), valJSON, id,
	)
	return err
}

// ── Bulk Update Custom Fields ────────────────────────────────

func (r *DataMasterRepo) MergeCustomFields(ctx context.Context, id uuid.UUID, fields map[string]interface{}) error {
	fieldsJSON, _ := json.Marshal(fields)
	_, err := r.db.ExecContext(ctx,
		`UPDATE master_data 
		 SET custom_fields = custom_fields || $1::jsonb
		 WHERE id = $2`,
		fieldsJSON, id,
	)
	return err
}

// ── Stage Transition ─────────────────────────────────────────

func (r *DataMasterRepo) TransitionStage(ctx context.Context, id uuid.UUID, newStage Stage, extraUpdates map[string]interface{}) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update stage
	_, err = tx.ExecContext(ctx,
		`UPDATE master_data SET stage = $1 WHERE id = $2`,
		newStage, id,
	)
	if err != nil {
		return err
	}

	// Merge extra custom field updates
	if len(extraUpdates) > 0 {
		fieldsJSON, _ := json.Marshal(extraUpdates)
		_, err = tx.ExecContext(ctx,
			`UPDATE master_data SET custom_fields = custom_fields || $1::jsonb WHERE id = $2`,
			fieldsJSON, id,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ── Query by Custom Field ────────────────────────────────────
// Contoh: "WHERE custom_fields->>'nps_score' >= '8'"

func (r *DataMasterRepo) QueryByCustomField(ctx context.Context, wsID uuid.UUID, fieldKey string, operator string, value interface{}) ([]DataMaster, error) {
	// Type-safe query builder
	query := fmt.Sprintf(
		`SELECT * FROM master_data 
		 WHERE workspace_id = $1 
		 AND (custom_fields->>$2)::%s %s $3
		 ORDER BY updated_at DESC`,
		inferPGType(value), operator,
	)

	var results []DataMaster
	err := r.db.SelectContext(ctx, &results, query, wsID, fieldKey, value)
	return results, err
}

func inferPGType(v interface{}) string {
	switch v.(type) {
	case int, int64, float64:
		return "numeric"
	case bool:
		return "boolean"
	default:
		return "text"
	}
}
```
