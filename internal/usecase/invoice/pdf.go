package invoice

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// PDFGenerator renders an invoice to a PDF-like document. Minimal pure-Go
// implementation: produces a structured HTML document that FE/PDF tooling
// can render into a real PDF client-side (or via headless chromium server-
// side). Keeps external dependencies zero.
//
// Call GeneratePDF for writer-based output, or GeneratePDFBytes for in-memory.
type PDFGenerator interface {
	GeneratePDF(ctx context.Context, workspaceID, invoiceID string, out io.Writer) error
	GeneratePDFBytes(ctx context.Context, workspaceID, invoiceID string) ([]byte, error)
}

type pdfGen struct {
	uc *invoiceUsecase
}

// NewPDFGenerator builds a PDFGenerator over an existing invoice usecase.
// Takes the concrete *invoiceUsecase so it can reach the repos without a
// circular dep through Usecase.
func NewPDFGenerator(uc Usecase) PDFGenerator {
	u, ok := uc.(*invoiceUsecase)
	if !ok {
		panic("invoice.NewPDFGenerator: expected *invoiceUsecase")
	}
	return &pdfGen{uc: u}
}

const invoiceHTMLTemplate = `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Invoice {{.Invoice.InvoiceID}}</title>
  <style>
    body { font-family: Helvetica, Arial, sans-serif; color: #222; margin: 40px; }
    h1 { font-size: 24px; margin: 0; }
    .muted { color: #666; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin: 24px 0; }
    table { width: 100%; border-collapse: collapse; margin-top: 12px; }
    th, td { padding: 8px 10px; border-bottom: 1px solid #ddd; text-align: left; }
    th { background: #f4f4f4; font-size: 12px; text-transform: uppercase; }
    .right { text-align: right; }
    .totals { margin-top: 16px; font-size: 14px; }
    .badge { display: inline-block; padding: 4px 10px; border-radius: 12px; font-size: 12px; }
    .badge.lunas { background: #d4edda; color: #155724; }
    .badge.pending { background: #fff3cd; color: #856404; }
    .badge.overdue { background: #f8d7da; color: #721c24; }
  </style>
</head>
<body>
  <h1>INVOICE</h1>
  <div class="muted">{{.Invoice.InvoiceID}}</div>

  <div class="grid">
    <div>
      <div class="muted">Bill to</div>
      <div><strong>{{.Invoice.CompanyName}}</strong></div>
      <div>{{.Invoice.CompanyID}}</div>
    </div>
    <div class="right">
      <div class="muted">Issue date</div>
      <div>{{fmtDate .Invoice.IssueDate}}</div>
      <div class="muted" style="margin-top:8px">Due date</div>
      <div>{{fmtDate .Invoice.DueDate}}</div>
      <div style="margin-top:8px"><span class="badge {{statusClass .Invoice.PaymentStatus}}">{{.Invoice.PaymentStatus}}</span></div>
    </div>
  </div>

  <table>
    <thead>
      <tr><th>Description</th><th class="right">Qty</th><th class="right">Unit</th><th class="right">Amount</th></tr>
    </thead>
    <tbody>
    {{range .LineItems}}
      <tr>
        <td>{{.Description}}</td>
        <td class="right">{{.Qty}}</td>
        <td class="right">{{fmtCurrency .UnitPrice}}</td>
        <td class="right">{{fmtCurrency .Subtotal}}</td>
      </tr>
    {{else}}
      <tr><td colspan="4" class="muted">No line items</td></tr>
    {{end}}
    </tbody>
  </table>

  <div class="totals right">
    <div>Subtotal: {{fmtCurrency .Subtotal}}</div>
    <div>Tax: {{fmtCurrency .Tax}}</div>
    <div><strong>Total: {{fmtCurrency .Total}}</strong></div>
  </div>

  {{if .Invoice.PaperIDURL}}
  <p style="margin-top:40px">Pay online: <a href="{{.Invoice.PaperIDURL}}">{{.Invoice.PaperIDURL}}</a></p>
  {{end}}
</body>
</html>`

type pdfContext struct {
	Invoice   entity.Invoice
	LineItems []entity.InvoiceLineItem
	Subtotal  float64
	Tax       float64
	Total     float64
}

var tplFuncs = template.FuncMap{
	"fmtDate": func(t interface{}) string {
		switch v := t.(type) {
		case time.Time:
			if v.IsZero() {
				return "-"
			}
			return v.Format("2 Jan 2006")
		case *time.Time:
			if v == nil || v.IsZero() {
				return "-"
			}
			return v.Format("2 Jan 2006")
		}
		return "-"
	},
	"fmtCurrency": func(v interface{}) string {
		n := float64(0)
		switch x := v.(type) {
		case float64:
			n = x
		case int64:
			n = float64(x)
		case int:
			n = float64(x)
		}
		return "Rp " + addCommas(strconv.FormatFloat(n, 'f', 0, 64))
	},
	"statusClass": func(s string) string {
		switch s {
		case entity.PaymentStatusLunas:
			return "lunas"
		case entity.PaymentStatusBelumBayar:
			return "pending"
		}
		return "overdue"
	},
}

func addCommas(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	// Simple thousand-separator, right-to-left.
	out := make([]byte, 0, n+n/3)
	pre := n % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
	}
	for i := pre; i < n; i += 3 {
		if len(out) > 0 {
			out = append(out, ',')
		}
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}

func (p *pdfGen) render(ctx context.Context, workspaceID, invoiceID string) (*pdfContext, error) {
	inv, err := p.uc.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || inv.WorkspaceID != workspaceID {
		return nil, apperror.NotFound("invoice", invoiceID)
	}
	items, err := p.uc.lineItemRepo.GetByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	var subtotal float64
	for _, it := range items {
		subtotal += float64(it.Subtotal)
	}
	tax := subtotal * 0.11 // PPN 11% baseline; adjustable per workspace in a future iter.
	total := subtotal + tax

	return &pdfContext{
		Invoice:   *inv,
		LineItems: items,
		Subtotal:  subtotal,
		Tax:       tax,
		Total:     total,
	}, nil
}

func (p *pdfGen) GeneratePDF(ctx context.Context, workspaceID, invoiceID string, out io.Writer) error {
	data, err := p.render(ctx, workspaceID, invoiceID)
	if err != nil {
		return err
	}
	t, err := template.New("invoice").Funcs(tplFuncs).Parse(invoiceHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	return t.Execute(out, data)
}

func (p *pdfGen) GeneratePDFBytes(ctx context.Context, workspaceID, invoiceID string) ([]byte, error) {
	var buf bytes.Buffer
	if err := p.GeneratePDF(ctx, workspaceID, invoiceID, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
