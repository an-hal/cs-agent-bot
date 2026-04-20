package collection

import (
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func TestParseSort(t *testing.T) {
	fields := []entity.CollectionField{
		{Key: "title", Type: entity.ColFieldText},
		{Key: "count", Type: entity.ColFieldNumber},
	}

	tests := []struct {
		name       string
		in         string
		wantKey    string
		wantType   string
		wantDesc   bool
		wantErrSub string
	}{
		{name: "default empty → created_at desc", in: "", wantKey: "created_at", wantDesc: true},
		{name: "created_at asc", in: "created_at:asc", wantKey: "created_at", wantDesc: false},
		{name: "created_at desc", in: "created_at:desc", wantKey: "created_at", wantDesc: true},
		{name: "updated_at asc", in: "updated_at:asc", wantKey: "updated_at", wantDesc: false},
		{name: "declared field with data. prefix", in: "data.title:asc", wantKey: "title", wantType: entity.ColFieldText, wantDesc: false},
		{name: "declared numeric field", in: "data.count:desc", wantKey: "count", wantType: entity.ColFieldNumber, wantDesc: true},
		{name: "no direction defaults to asc", in: "data.title", wantKey: "title", wantType: entity.ColFieldText, wantDesc: false},
		{name: "unknown field rejected", in: "data.ghost:asc", wantErrSub: "unknown field"},
		{name: "invalid direction rejected", in: "data.title:backward", wantErrSub: "direction"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, fieldType, desc, err := parseSort(tc.in, fields)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("got %q, want error containing %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if key != tc.wantKey {
				t.Fatalf("key: got %q, want %q", key, tc.wantKey)
			}
			if fieldType != tc.wantType {
				t.Fatalf("type: got %q, want %q", fieldType, tc.wantType)
			}
			if desc != tc.wantDesc {
				t.Fatalf("desc: got %v, want %v", desc, tc.wantDesc)
			}
		})
	}
}
