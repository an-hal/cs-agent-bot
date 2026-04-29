// Package xlsxexport generates client XLSX exports matching the standard import template layout.
package xlsxexport

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/xuri/excelize/v2"
)

const sheetName = "Clients"

var exportHeaders = []string{
	"Company ID", "Company Name", "Stage",
	"PIC Name", "PIC Role", "PIC WA", "PIC Email",
	"Contract Start", "Contract End", "Contract Months",
	"Payment Terms", "Final Price", "Payment Status",
	"Last Payment Date",
	"Last Interaction",
	"Checkin Replied",
	"Bot Active", "Blacklisted", "Sequence CS",
	"Owner Name", "Owner WA", "Owner Telegram ID",
	"Notes",
}

// WriteClientSheet writes clients to a new XLSX workbook and streams it to w.
func WriteClientSheet(w io.Writer, clients []entity.Client) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := f.GetSheetName(0)
	f.SetSheetName(sheet, sheetName)

	for col, h := range exportHeaders {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheetName, cell, SanitizeCell(h))
	}

	for i, c := range clients {
		rowNum := i + 2
		values := clientRowValues(c)
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowNum)
			f.SetCellValue(sheetName, cell, SanitizeCell(v))
		}
	}

	if err := f.Write(w); err != nil {
		return fmt.Errorf("failed to write xlsx: %w", err)
	}
	return nil
}

func clientRowValues(c entity.Client) []interface{} {
	return []interface{}{
		c.CompanyID,
		c.CompanyName,
		c.SequenceCS,
		c.PICName,
		c.PICRole,
		c.PICWA,
		c.PICEmail,
		fmtDate(c.ContractStart),
		fmtDate(c.ContractEnd),
		strconv.Itoa(c.ContractMonths),
		c.PaymentTerms,
		c.FinalPrice,
		c.PaymentStatus,
		fmtNullableDate(c.LastPaymentDate),
		"",
		"",
		boolToYesNo(c.BotActive),
		boolToYesNo(c.Blacklisted),
		c.SequenceCS,
		c.OwnerName,
		c.GetOwnerWA(),
		c.OwnerTelegramID,
		c.Notes,
	}
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func fmtNullableDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
