// Package htmlsanitize wraps bluemonday to enforce the project-wide email
// HTML allowlist defined in context/for-backend/features/00-shared/03-html-sanitization.md.
package htmlsanitize

import (
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	emailPolicyOnce sync.Once
	emailPolicy     *bluemonday.Policy
)

// EmailPolicy returns a singleton bluemonday policy that allows the tags,
// attributes, and styles required by the TipTap-powered email editor.
func EmailPolicy() *bluemonday.Policy {
	emailPolicyOnce.Do(func() {
		p := bluemonday.UGCPolicy()

		// Headings, block, inline, list, table, link, media are all in UGCPolicy.
		// Explicitly allow style on block and inline elements the editor produces.
		p.AllowAttrs("style", "class").OnElements("span", "div", "p", "h1", "h2", "h3", "h4", "h5", "h6")
		p.AllowAttrs("colspan", "rowspan", "style").OnElements("td", "th")
		p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
		p.AllowAttrs("href", "title", "target", "rel").OnElements("a")

		// Data-* attributes on all elements.
		p.AllowDataAttributes()

		// Allowed CSS properties inside style=""
		p.AllowStyles(
			"color",
			"background-color",
			"text-align",
			"font-size",
			"font-weight",
			"font-style",
			"text-decoration",
			"margin", "margin-top", "margin-bottom",
			"padding", "padding-top", "padding-bottom",
			"border", "border-collapse",
			"width", "height",
			"max-width",
		).Globally()

		// Allow images over http(s) and data: (inline base64 previews).
		p.AllowImages()
		p.AllowStandardURLs()

		emailPolicy = p
	})
	return emailPolicy
}

// SanitizeEmailHTML strips dangerous tags/attributes from email body HTML.
// Variable placeholders like [Variable_Name] are preserved verbatim since
// they are plain text, not HTML.
func SanitizeEmailHTML(dirty string) string {
	return EmailPolicy().Sanitize(dirty)
}
