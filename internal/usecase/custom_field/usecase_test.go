package custom_field

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type stubCFD struct {
	defs       []entity.CustomFieldDefinition
	createErr  error
	createOut  *entity.CustomFieldDefinition
	getOut     *entity.CustomFieldDefinition
	updateOut  *entity.CustomFieldDefinition
	reorderErr error
}

func (s *stubCFD) List(ctx context.Context, ws string) ([]entity.CustomFieldDefinition, error) {
	return s.defs, nil
}
func (s *stubCFD) GetByID(ctx context.Context, ws, id string) (*entity.CustomFieldDefinition, error) {
	return s.getOut, nil
}
func (s *stubCFD) Create(ctx context.Context, ws string, d *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createOut != nil {
		return s.createOut, nil
	}
	d.ID = "cfd-1"
	return d, nil
}
func (s *stubCFD) Update(ctx context.Context, ws, id string, d *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	if s.updateOut != nil {
		return s.updateOut, nil
	}
	return d, nil
}
func (s *stubCFD) Delete(ctx context.Context, ws, id string) error { return nil }
func (s *stubCFD) Reorder(ctx context.Context, ws string, items []repository.ReorderItem) error {
	return s.reorderErr
}

func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name string
		req  CreateRequest
		want string
	}{
		{"bad field_key", CreateRequest{FieldKey: "BadKey", FieldLabel: "x", FieldType: "text"}, "field_key"},
		{"empty label", CreateRequest{FieldKey: "x", FieldLabel: " ", FieldType: "text"}, "field_label"},
		{"bad type", CreateRequest{FieldKey: "x", FieldLabel: "X", FieldType: "wrong"}, "field_type"},
		{"select needs options", CreateRequest{FieldKey: "x", FieldLabel: "X", FieldType: "select"}, "option"},
	}
	uc := New(&stubCFD{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), "ws", tc.req)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestCreate_DuplicateKey(t *testing.T) {
	uc := New(&stubCFD{createErr: errors.New("duplicate key value violates unique constraint")})
	_, err := uc.Create(context.Background(), "ws", CreateRequest{
		FieldKey: "hc_size", FieldLabel: "HC", FieldType: "number",
	})
	if err == nil || !strings.Contains(err.Error(), "field_key") {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestUpdate_FieldKeyImmutable(t *testing.T) {
	stub := &stubCFD{
		getOut: &entity.CustomFieldDefinition{ID: "cfd-1", FieldKey: "original", FieldType: "text"},
	}
	uc := New(stub)
	out, err := uc.Update(context.Background(), "ws", "cfd-1", UpdateRequest{
		FieldLabel: "New Label", FieldType: "text",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out.FieldKey != "original" {
		t.Fatalf("field_key changed to %q", out.FieldKey)
	}
}

func TestReorder_RequiresItems(t *testing.T) {
	uc := New(&stubCFD{})
	if err := uc.Reorder(context.Background(), "ws", nil); err == nil {
		t.Fatalf("expected error for empty order")
	}
}

func TestList_RequiresWorkspace(t *testing.T) {
	uc := New(&stubCFD{})
	if _, err := uc.List(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}
