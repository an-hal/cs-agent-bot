package messaging

import (
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/htmlsanitize"
)

func TestSanitizeEmailHTML_StripsDangerousContent(t *testing.T) {
	cases := []struct {
		name           string
		dirty          string
		mustNotContain []string
		mustContain    []string
	}{
		{
			name:           "strips script tag",
			dirty:          `<p>Hello</p><script>alert('xss')</script>`,
			mustNotContain: []string{"<script", "alert"},
			mustContain:    []string{"<p>Hello</p>"},
		},
		{
			name:           "strips inline event handlers",
			dirty:          `<a href="https://example.com" onclick="alert(1)">x</a>`,
			mustNotContain: []string{"onclick"},
			mustContain:    []string{`href="https://example.com"`},
		},
		{
			name:           "preserves variable placeholders verbatim",
			dirty:          `<p>Halo [PIC_Name] dari [Company_Name]!</p>`,
			mustNotContain: []string{"<script"},
			mustContain:    []string{"[PIC_Name]", "[Company_Name]"},
		},
		{
			name:           "strips javascript: URLs",
			dirty:          `<a href="javascript:alert(1)">click</a>`,
			mustNotContain: []string{"javascript:"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := htmlsanitize.SanitizeEmailHTML(tc.dirty)
			for _, banned := range tc.mustNotContain {
				if strings.Contains(got, banned) {
					t.Errorf("sanitized output %q should not contain %q", got, banned)
				}
			}
			for _, req := range tc.mustContain {
				if !strings.Contains(got, req) {
					t.Errorf("sanitized output %q should contain %q", got, req)
				}
			}
		})
	}
}
