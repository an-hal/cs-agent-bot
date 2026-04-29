package entity

import "time"

// Contact kinds — see context/new/multi-stage-pic-spec.md.
const (
	ContactKindInternal   = "internal"
	ContactKindClientSide = "client_side"
)

// ClientContact is one PIC at a specific (client, stage, kind) — i.e. either
// the internal team owner or the client-side counterpart for that lifecycle
// stage. Lets us model SDR Icha → Owner Baba (LEAD), BD Shafira → HR Fina
// (PROSPECT), AE Anggi → Finance Caca (CLIENT) for the same client without
// overwriting history on stage transition.
type ClientContact struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	MasterDataID string `json:"master_data_id"`

	Stage string `json:"stage"` // LEAD | PROSPECT | CLIENT | DORMANT
	Kind  string `json:"kind"`  // internal | client_side
	Role  string `json:"role"`  // SDR | BD | AE | Owner | HR | Finance | …

	Name       string `json:"name"`
	WA         string `json:"wa,omitempty"`
	Email      string `json:"email,omitempty"`
	TelegramID string `json:"telegram_id,omitempty"`

	IsPrimary bool   `json:"is_primary"`
	Notes     string `json:"notes,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsValidContactKind reports whether s is one of the allowed kind values.
func IsValidContactKind(s string) bool {
	return s == ContactKindInternal || s == ContactKindClientSide
}
