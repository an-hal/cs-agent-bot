package entity

import "time"

// Collection field type constants (feat/10 §Field Types Supported).
// Prefix "Col" disambiguates from master_data's FieldType* constants.
const (
	ColFieldText       = "text"
	ColFieldTextarea   = "textarea"
	ColFieldNumber     = "number"
	ColFieldBoolean    = "boolean"
	ColFieldDate       = "date"
	ColFieldDateTime   = "datetime"
	ColFieldEnum       = "enum"
	ColFieldMultiEnum  = "multi_enum"
	ColFieldURL        = "url"
	ColFieldEmail      = "email"
	ColFieldLinkClient = "link_client"
	ColFieldFile       = "file"
)

// CollectionSchemaChangeType is the request_type stored on approval_requests for
// schema mutations. Non-enum, stable string per spec §Checker-Maker.
const CollectionSchemaChangeType = "collection_schema_change"

// Collection approval payload.op values dispatched by ApplyCollectionSchemaChange.
const (
	OpCreateCollection = "create_collection"
	OpDeleteCollection = "delete_collection"
	OpUpdateCollection = "update_collection"
	OpAddField         = "add_field"
	OpDeleteField      = "delete_field"
)

// Hard limits per spec §Scale Assumptions.
const (
	MaxCollectionsPerWorkspace = 50
	MaxFieldsPerCollection     = 30
	MaxRecordsPerCollection    = 10000
	MaxDistinctValues          = 500
)

// IsValidColFieldType reports whether s is one of the supported field type constants.
func IsValidColFieldType(s string) bool {
	switch s {
	case ColFieldText, ColFieldTextarea, ColFieldNumber, ColFieldBoolean,
		ColFieldDate, ColFieldDateTime, ColFieldEnum, ColFieldMultiEnum,
		ColFieldURL, ColFieldEmail, ColFieldLinkClient, ColFieldFile:
		return true
	}
	return false
}

// Collection is the meta row for a user-defined table.
type Collection struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Permissions map[string]any    `json:"permissions"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeletedAt   *time.Time        `json:"deleted_at,omitempty"`
	RecordCount int               `json:"record_count,omitempty"`
	FieldCount  int               `json:"field_count,omitempty"`
	Fields      []CollectionField `json:"fields,omitempty"`
}

// CollectionField is one declared column on a collection.
type CollectionField struct {
	ID           string         `json:"id"`
	CollectionID string         `json:"collection_id"`
	Key          string         `json:"key"`
	Label        string         `json:"label"`
	Type         string         `json:"type"`
	Required     bool           `json:"required"`
	Options      map[string]any `json:"options"`
	DefaultValue any            `json:"default_value,omitempty"`
	Order        int            `json:"order"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// CollectionRecord is one row of user data.
type CollectionRecord struct {
	ID           string         `json:"id"`
	CollectionID string         `json:"collection_id"`
	Data         map[string]any `json:"data"`
	CreatedBy    string         `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    *time.Time     `json:"deleted_at,omitempty"`
}

// CollectionRecordListOptions captures list-query parameters evaluated by the
// repository. Filter/Sort/Distinct keys have already been validated against the
// collection's field schema by the usecase before this struct is constructed.
type CollectionRecordListOptions struct {
	CollectionID string
	Limit        int
	Offset       int
	SortKey      string // validated field key, "created_at", or "updated_at"
	SortType     string // field type to pick the correct cast
	SortDesc     bool
	Search       string
	// FilterSQL is the pre-built AND-joined fragment (without leading AND),
	// e.g. "(data->>'category' IN ($N,$M))". Empty for no filter.
	FilterSQL  string
	FilterArgs []any
}

// DistinctOptions requests the set of distinct non-empty values on a single field.
type DistinctOptions struct {
	CollectionID string
	FieldKey     string // validated
	FieldType    string
	Limit        int
	FilterSQL    string
	FilterArgs   []any
}

// DistinctResult is the response body for GET /collections/{id}/records/distinct.
type DistinctResult struct {
	Field     string   `json:"field"`
	Values    []string `json:"values"`
	Truncated bool     `json:"truncated"`
}
