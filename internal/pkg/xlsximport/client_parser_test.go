package xlsximport

import (
	"bytes"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/xuri/excelize/v2"
)

// buildXLSX builds a minimal xlsx with header + data rows for the standard
// import sheet. extraHeaders/extraVals are appended after the 19 core columns.
func buildXLSX(t *testing.T, extraHeaders []string, extraVals []string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if _, err := f.NewSheet("Template Import"); err != nil {
		t.Fatal(err)
	}
	f.DeleteSheet("Sheet1")
	core := []string{
		"Company ID", "Company Name", "Stage",
		"PIC Name", "PIC Role", "PIC WA", "PIC Email",
		"Owner Name", "Owner WA",
		"Bot Active", "Blacklisted", "Risk Flag",
		"Contract Start", "Contract End", "Contract Months",
		"Payment Status", "Payment Terms", "Final Price",
		"Notes",
	}
	headers := append(core, extraHeaders...)
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue("Template Import", cell, h)
	}
	dataCore := []any{
		"COMP-001", "Acme Corp", "CLIENT",
		"Alice", "CTO", "62811000001", "alice@acme.com",
		"AE Team", "62811000002",
		false, false, "Low",
		"2026-01-01", "2026-12-31", 12,
		"Lunas", "Net30", 50000000,
		"Imported test row",
	}
	dataAll := append(dataCore, anyifyStrings(extraVals)...)
	for i, v := range dataAll {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue("Template Import", cell, v)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func anyifyStrings(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func TestParseClientSheet_BackwardCompat_NoCustomFields(t *testing.T) {
	data := buildXLSX(t, nil, nil)
	rows, errs, err := ParseClientSheet(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected parse errs: %v", errs)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].CompanyID != "COMP-001" || rows[0].CompanyName != "Acme Corp" {
		t.Errorf("standard fields wrong: %+v", rows[0])
	}
	if rows[0].CustomFields != nil {
		t.Errorf("expected nil CustomFields, got %v", rows[0].CustomFields)
	}
}

func TestParseClientSheetWithDefs_MatchByLabel(t *testing.T) {
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "subscription_type", FieldLabel: "Subscription Type", FieldType: entity.FieldTypeSelect},
		{FieldKey: "headcount", FieldLabel: "Headcount", FieldType: entity.FieldTypeNumber},
	}
	data := buildXLSX(t,
		[]string{"Subscription Type", "Headcount"},
		[]string{"paid", "150"},
	)
	rows, _, err := ParseClientSheetWithDefs(bytes.NewReader(data), defs)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows", len(rows))
	}
	cf := rows[0].CustomFields
	if cf["subscription_type"] != "paid" {
		t.Errorf("subscription_type: %v", cf["subscription_type"])
	}
	if hc, ok := cf["headcount"].(float64); !ok || hc != 150 {
		t.Errorf("headcount: %v (%T)", cf["headcount"], cf["headcount"])
	}
}

func TestParseClientSheetWithDefs_MatchByFieldKey_Fallback(t *testing.T) {
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "subscription_type", FieldLabel: "Sub Type Label", FieldType: entity.FieldTypeText},
	}
	// xlsx header uses field_key, not field_label
	data := buildXLSX(t, []string{"subscription_type"}, []string{"trial"})
	rows, _, err := ParseClientSheetWithDefs(bytes.NewReader(data), defs)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CustomFields["subscription_type"] != "trial" {
		t.Errorf("expected fallback match by field_key, got %v", rows[0].CustomFields)
	}
}

func TestParseClientSheetWithDefs_EmptyCellSkipped(t *testing.T) {
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "headcount", FieldLabel: "Headcount", FieldType: entity.FieldTypeNumber},
	}
	data := buildXLSX(t, []string{"Headcount"}, []string{""})
	rows, _, err := ParseClientSheetWithDefs(bytes.NewReader(data), defs)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CustomFields != nil {
		t.Errorf("expected nil CustomFields when only column is empty, got %v", rows[0].CustomFields)
	}
}

func TestParseClientSheetWithDefs_BooleanAndDate(t *testing.T) {
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "is_vip", FieldLabel: "Is VIP", FieldType: entity.FieldTypeBoolean},
		{FieldKey: "renewal_at", FieldLabel: "Renewal At", FieldType: entity.FieldTypeDate},
	}
	data := buildXLSX(t,
		[]string{"Is VIP", "Renewal At"},
		[]string{"yes", "2027-03-15"},
	)
	rows, _, err := ParseClientSheetWithDefs(bytes.NewReader(data), defs)
	if err != nil {
		t.Fatal(err)
	}
	cf := rows[0].CustomFields
	if cf["is_vip"] != true {
		t.Errorf("is_vip: %v (%T)", cf["is_vip"], cf["is_vip"])
	}
	if cf["renewal_at"] != "2027-03-15" {
		t.Errorf("renewal_at: %v", cf["renewal_at"])
	}
}

func TestParseClientSheetWithDefs_UnmatchedDefIgnored(t *testing.T) {
	defs := []entity.CustomFieldDefinition{
		{FieldKey: "missing_field", FieldLabel: "Not In Sheet", FieldType: entity.FieldTypeText},
	}
	data := buildXLSX(t, nil, nil)
	rows, _, err := ParseClientSheetWithDefs(bytes.NewReader(data), defs)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].CustomFields != nil {
		t.Errorf("expected nil CustomFields when no def matches, got %v", rows[0].CustomFields)
	}
}
