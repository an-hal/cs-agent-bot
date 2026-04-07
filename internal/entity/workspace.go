package entity

import "time"

type Workspace struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Logo      string    `json:"logo"`
	Color     string    `json:"color"`
	Plan      string    `json:"plan"`
	IsHolding bool      `json:"is_holding"`
	MemberIDs []string  `json:"member_ids"`
	CreatedAt time.Time `json:"created_at"`
}
