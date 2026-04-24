# HTML Sanitization Rules

## Context

Email templates use TipTap rich text editor yang menghasilkan HTML.
Backend harus sanitize HTML sebelum simpan ke DB untuk mencegah XSS.

Frontend sudah pakai `isomorphic-dompurify` — backend harus punya aturan yang sama.

## Allowed Tags

```go
var allowedTags = []string{
    // Headings
    "h1", "h2", "h3", "h4", "h5", "h6",
    // Block
    "p", "div", "br", "hr", "blockquote", "pre",
    // Inline
    "b", "strong", "i", "em", "u", "s", "del", "sub", "sup", "code", "span",
    // Lists
    "ul", "ol", "li",
    // Table
    "table", "thead", "tbody", "tfoot", "tr", "th", "td",
    // Links & Media
    "a", "img",
}
```

## Allowed Attributes

```go
var allowedAttributes = map[string][]string{
    "a":     {"href", "title", "target", "rel"},
    "img":   {"src", "alt", "title", "width", "height"},
    "td":    {"colspan", "rowspan", "style"},
    "th":    {"colspan", "rowspan", "style"},
    "span":  {"style", "class"},
    "div":   {"style", "class"},
    "p":     {"style", "class"},
    "*":     {"data-*"},  // data attributes allowed on all
}
```

## Allowed Styles (within style attribute)

```go
var allowedStyles = []string{
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
}
```

## Variable Placeholder Format

Email templates use `[Variable_Name]` placeholders:

```
Allowed pattern: /\[[A-Za-z_][A-Za-z0-9_]*\]/
Examples: [Company_Name], [PIC_Name], [link_wiki], [Due_Date]
```

These are NOT HTML — they're rendered as plain text by the template engine.
Backend should preserve them as-is during sanitization.

## Go Library Recommendation

Use `github.com/microcosm-cc/bluemonday`:

```go
import "github.com/microcosm-cc/bluemonday"

func sanitizeEmailHTML(dirty string) string {
    p := bluemonday.NewPolicy()
    p.AllowStandardURLs()
    p.AllowElements(allowedTags...)
    p.AllowAttrs("href", "title", "target", "rel").OnElements("a")
    p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
    p.AllowAttrs("colspan", "rowspan", "style").OnElements("td", "th")
    p.AllowAttrs("style", "class").OnElements("span", "div", "p")
    p.AllowStyles("color", "background-color", "text-align", "font-size",
        "font-weight", "font-style", "text-decoration").Globally()
    return p.Sanitize(dirty)
}
```
