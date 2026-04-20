package collection

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func TestValidateRecordData(t *testing.T) {
	ctx := context.Background()

	fields := []entity.CollectionField{
		{Key: "title", Label: "Title", Type: entity.ColFieldText, Required: true, Options: map[string]any{"maxLength": float64(10)}},
		{Key: "count", Label: "Count", Type: entity.ColFieldNumber, Options: map[string]any{"min": float64(0), "max": float64(100)}},
		{Key: "active", Label: "Active", Type: entity.ColFieldBoolean},
		{Key: "start", Label: "Start", Type: entity.ColFieldDate},
		{Key: "when", Label: "When", Type: entity.ColFieldDateTime},
		{Key: "stage", Label: "Stage", Type: entity.ColFieldEnum, Options: map[string]any{"choices": []any{"A", "B"}}},
		{Key: "tags", Label: "Tags", Type: entity.ColFieldMultiEnum, Options: map[string]any{"choices": []any{"x", "y"}}},
		{Key: "homepage", Label: "Home", Type: entity.ColFieldURL},
		{Key: "contact", Label: "Contact", Type: entity.ColFieldEmail},
	}

	tests := []struct {
		name    string
		data    map[string]any
		strict  bool
		wantErr bool
		wantKey string
	}{
		{
			name: "happy path",
			data: map[string]any{
				"title":    "hi",
				"count":    float64(5),
				"active":   true,
				"start":    "2026-04-17",
				"when":     "2026-04-17T10:00:00Z",
				"stage":    "A",
				"tags":     []any{"x"},
				"homepage": "https://example.com",
				"contact":  "a@b.com",
			},
			strict:  true,
			wantErr: false,
		},
		{
			name:    "missing required",
			data:    map[string]any{},
			strict:  true,
			wantErr: true,
			wantKey: "title",
		},
		{
			name:    "text exceeds maxLength",
			data:    map[string]any{"title": "this is too long"},
			strict:  true,
			wantErr: true,
			wantKey: "title",
		},
		{
			name:    "number below min",
			data:    map[string]any{"title": "ok", "count": float64(-1)},
			strict:  true,
			wantErr: true,
			wantKey: "count",
		},
		{
			name:    "number above max",
			data:    map[string]any{"title": "ok", "count": float64(101)},
			strict:  true,
			wantErr: true,
			wantKey: "count",
		},
		{
			name:    "bool wrong type",
			data:    map[string]any{"title": "ok", "active": "yes"},
			strict:  true,
			wantErr: true,
			wantKey: "active",
		},
		{
			name:    "date wrong format",
			data:    map[string]any{"title": "ok", "start": "2026/04/17"},
			strict:  true,
			wantErr: true,
			wantKey: "start",
		},
		{
			name:    "datetime wrong format",
			data:    map[string]any{"title": "ok", "when": "not-a-date"},
			strict:  true,
			wantErr: true,
			wantKey: "when",
		},
		{
			name:    "enum not in choices",
			data:    map[string]any{"title": "ok", "stage": "Z"},
			strict:  true,
			wantErr: true,
			wantKey: "stage",
		},
		{
			name:    "multi_enum item not in choices",
			data:    map[string]any{"title": "ok", "tags": []any{"q"}},
			strict:  true,
			wantErr: true,
			wantKey: "tags",
		},
		{
			name:    "multi_enum not array",
			data:    map[string]any{"title": "ok", "tags": "x"},
			strict:  true,
			wantErr: true,
			wantKey: "tags",
		},
		{
			name:    "url invalid",
			data:    map[string]any{"title": "ok", "homepage": "not a url"},
			strict:  true,
			wantErr: true,
			wantKey: "homepage",
		},
		{
			name:    "email invalid",
			data:    map[string]any{"title": "ok", "contact": "nope"},
			strict:  true,
			wantErr: true,
			wantKey: "contact",
		},
		{
			name:    "strict rejects unknown key",
			data:    map[string]any{"title": "ok", "mystery": 1},
			strict:  true,
			wantErr: true,
			wantKey: "mystery",
		},
		{
			name:    "non-strict accepts unknown key",
			data:    map[string]any{"title": "ok", "mystery": 1},
			strict:  false,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs, err := ValidateRecordData(ctx, fields, tc.data, tc.strict, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil (errs=%v)", errs)
				}
				if tc.wantKey != "" {
					if _, ok := errs[tc.wantKey]; !ok {
						t.Fatalf("expected error on key %q, got %v", tc.wantKey, errs)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v (errs=%v)", err, errs)
			}
		})
	}
}

func TestValidateLinkClient(t *testing.T) {
	ctx := context.Background()
	fields := []entity.CollectionField{{Key: "owner", Label: "Owner", Type: entity.ColFieldLinkClient}}

	t.Run("ok when client exists", func(t *testing.T) {
		check := func(ctx context.Context, id string) (bool, error) { return true, nil }
		_, err := ValidateRecordData(ctx, fields, map[string]any{"owner": "uuid-1"}, true, check)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("error when client missing", func(t *testing.T) {
		check := func(ctx context.Context, id string) (bool, error) { return false, nil }
		errs, err := ValidateRecordData(ctx, fields, map[string]any{"owner": "uuid-1"}, true, check)
		if err == nil {
			t.Fatalf("expected error")
		}
		if _, ok := errs["owner"]; !ok {
			t.Fatalf("expected owner error, got %v", errs)
		}
	})
}
