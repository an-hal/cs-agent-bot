package messaging

import (
	"reflect"
	"testing"
)

func TestExtractVariables(t *testing.T) {
	cases := []struct {
		name     string
		inputs   []string
		expected []string
	}{
		{
			name:     "basic placeholders",
			inputs:   []string{"Halo [PIC_Name] dari [Company_Name]!"},
			expected: []string{"Company_Name", "PIC_Name"},
		},
		{
			name:     "deduplicates across inputs",
			inputs:   []string{"[Company_Name]", "selamat datang [Company_Name]"},
			expected: []string{"Company_Name"},
		},
		{
			name:     "ignores empty brackets and numeric-leading names",
			inputs:   []string{"[] [123abc] [_valid] [Also_Valid1]"},
			expected: []string{"Also_Valid1", "_valid"},
		},
		{
			name:     "no placeholders",
			inputs:   []string{"plain text"},
			expected: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractVariables(tc.inputs...)
			if len(got) == 0 && len(tc.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("ExtractVariables(%v) = %v, want %v", tc.inputs, got, tc.expected)
			}
		})
	}
}

func TestRender(t *testing.T) {
	cases := []struct {
		name            string
		text            string
		sample          map[string]string
		wantRendered    string
		wantMissing     []string
	}{
		{
			name:         "all variables resolved",
			text:         "Halo [PIC_Name] dari [Company_Name]!",
			sample:       map[string]string{"PIC_Name": "Budi", "Company_Name": "PT Maju"},
			wantRendered: "Halo Budi dari PT Maju!",
			wantMissing:  []string{},
		},
		{
			name:         "some variables missing are preserved",
			text:         "Halo [PIC_Name], due [Due_Date]",
			sample:       map[string]string{"PIC_Name": "Budi"},
			wantRendered: "Halo Budi, due [Due_Date]",
			wantMissing:  []string{"Due_Date"},
		},
		{
			name:         "sorted unique missing vars",
			text:         "[A] [B] [A] [C]",
			sample:       map[string]string{},
			wantRendered: "[A] [B] [A] [C]",
			wantMissing:  []string{"A", "B", "C"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered, missing := Render(tc.text, tc.sample)
			if rendered != tc.wantRendered {
				t.Errorf("rendered = %q, want %q", rendered, tc.wantRendered)
			}
			if len(missing) == 0 && len(tc.wantMissing) == 0 {
				return
			}
			if !reflect.DeepEqual(missing, tc.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tc.wantMissing)
			}
		})
	}
}
