package entity

import (
	"time"

	"github.com/google/uuid"
)

// Example represents a sample entity for demonstrating Clean Architecture patterns.
// This can be used as a template for creating new entities in the payment gateway service.
type Example struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"` // e.g., "active", "inactive"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// ExampleStatus constants
const (
	ExampleStatusActive   = "active"
	ExampleStatusInactive = "inactive"
)
