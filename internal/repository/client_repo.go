package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/rs/zerolog"
)

type ClientRepository interface {
	GetAll(ctx context.Context) ([]entity.Client, error)
	GetByID(ctx context.Context, companyID string) (*entity.Client, error)
	GetByWANumber(ctx context.Context, waNumber string) (*entity.Client, error)
	GetByCompanyID(ctx context.Context, companyID string) (*entity.Client, error)
	GetLatestInvoice(ctx context.Context, companyID string) (*entity.Invoice, error)
	UpdateLastInteraction(ctx context.Context, companyID string, t time.Time) error
	CreateClient(ctx context.Context, client entity.Client) error
	UpdateInvoiceReminderFlags(ctx context.Context, companyID string, flags map[string]bool) error
	UpdatePaymentStatus(ctx context.Context, companyID, status string) error
}

type clientRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewClientRepo(sheetsClient *sheets.SheetsClient, cache cache.SheetCache, logger zerolog.Logger) ClientRepository {
	return &clientRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *clientRepo) GetAll(ctx context.Context) ([]entity.Client, error) {
	rows, err := r.cache.Get(ctx, cache.KeyMasterClient, cache.RangeMasterClient, cache.TTLMasterClient)
	if err != nil {
		return nil, err
	}

	var clients []entity.Client
	// Skip first 5 rows (indices 0-4): 4 info rows + 1 column header row
	// Data starts at row 6 (index 5)
	for i, row := range rows {
		if i < 5 {
			continue // skip header rows
		}
		c, err := parseClientRow(row)
		if err != nil {
			r.logger.Warn().Err(err).Int("row", i+1).Msg("Failed to parse client row")
			continue
		}
		clients = append(clients, *c)
	}

	return clients, nil
}

func (r *clientRepo) GetByID(ctx context.Context, companyID string) (*entity.Client, error) {
	clients, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		if c.CompanyID == companyID {
			return &c, nil
		}
	}

	return nil, fmt.Errorf("client not found: %s", companyID)
}

func (r *clientRepo) GetByWANumber(ctx context.Context, waNumber string) (*entity.Client, error) {
	clients, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		if c.PICWA == waNumber {
			return &c, nil
		}
	}

	return nil, fmt.Errorf("client not found for WA number: %s", waNumber)
}

func (r *clientRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.Client, error) {
	return r.GetByID(ctx, companyID)
}

func (r *clientRepo) GetLatestInvoice(ctx context.Context, companyID string) (*entity.Invoice, error) {
	// For POC, return nil. In production, this would fetch from Invoice sheet
	// TODO: Implement actual invoice fetching from Invoice & Billing sheet
	return nil, fmt.Errorf("no invoice found for company: %s", companyID)
}

func (r *clientRepo) UpdatePaymentStatus(ctx context.Context, companyID, status string) error {
	clients, err := r.GetAll(ctx)
	if err != nil {
		return err
	}

	for i, c := range clients {
		if c.CompanyID == companyID {
			// Payment_Status is at column 15 (0-indexed), which is P in 1-indexed
			// Spreadsheet row calculation: 5 header rows + data index + 1 (1-indexed) = i + 6
			cellRange := fmt.Sprintf("1. Master Client!P%d", i+6)
			if err := r.sheetsClient.UpdateCell(ctx, cellRange, status); err != nil {
				return err
			}
			// Also update Last_Interaction_Date (column 19, T)
			now := time.Now().Format("2006-01-02")
			cellRange2 := fmt.Sprintf("1. Master Client!T%d", i+6)
			if err := r.sheetsClient.UpdateCell(ctx, cellRange2, now); err != nil {
				return err
			}
			return r.cache.Invalidate(ctx, cache.KeyMasterClient)
		}
	}

	return fmt.Errorf("client not found: %s", companyID)
}

func (r *clientRepo) UpdateLastInteraction(ctx context.Context, companyID string, t time.Time) error {
	clients, err := r.GetAll(ctx)
	if err != nil {
		return err
	}

	for i, c := range clients {
		if c.CompanyID == companyID {
			// Last_Interaction_Date is at column 19 (0-indexed), which is T in 1-indexed
			// Spreadsheet row calculation: 5 header rows + data index + 1 (1-indexed) = i + 6
			cellRange := fmt.Sprintf("1. Master Client!T%d", i+6)
			if err := r.sheetsClient.UpdateCell(ctx, cellRange, t.Format("2006-01-02")); err != nil {
				return err
			}
			return r.cache.Invalidate(ctx, cache.KeyMasterClient)
		}
	}

	return fmt.Errorf("client not found: %s", companyID)
}

func (r *clientRepo) CreateClient(ctx context.Context, client entity.Client) error {
	row := clientToRow(client)
	if err := r.sheetsClient.AppendRows(ctx, cache.RangeMasterClient, [][]interface{}{row}); err != nil {
		return err
	}
	return r.cache.Invalidate(ctx, cache.KeyMasterClient)
}

// parseClientRow maps a Sheets row to entity.Client.
// Column order based on "POC Project Bumi – AE HRIS Automation.xlsx" Sheet 1: Master Client
// Row 5 (index 4) contains actual column names. Data starts at row 7 (index 6).
func parseClientRow(row []interface{}) (*entity.Client, error) {
	if len(row) < 47 {
		return nil, fmt.Errorf("row too short: %d columns, expected at least 47", len(row))
	}

	c := &entity.Client{
		// Core identity (Cols 0-3)
		CompanyID:   safeString(row, 0), // Company_ID
		CompanyName: safeString(row, 1), // Company_Name
		PICName:     safeString(row, 2), // PIC_Name
		PICWA:       safeString(row, 3), // PIC_WA

		// Segment & contract (Cols 10-12)
		Segment:        safeString(row, 10), // Segment
		ContractMonths: safeInt(row, 8),     // Next_Discount_%_Manual
		ContractStart:  safeDate(row, 11),   // Contract_Start
		ContractEnd:    safeDate(row, 12),   // Contract_End

		// Payment & scores (Cols 15, 17-18)
		PaymentStatus: safeString(row, 15), // Payment_Status
		NPSScore:      safeInt(row, 17),    // NPS_Score
		UsageScore:    safeInt(row, 18),    // Usage_Score (0-100)

		// Owner info (Cols 21-22, 44)
		OwnerName:       safeString(row, 21), // Owner_Name
		OwnerWA:         safeString(row, 22), // Owner_WA
		OwnerTelegramID: safeString(row, 44), // Owner_Telegram_ID

		// Cross-sell state (Cols 24-26)
		SequenceCS:          safeString(row, 24), // sequence_cs
		CrossSellRejected:   safeBool(row, 25),   // cross_sell_rejected
		CrossSellInterested: safeBool(row, 26),   // cross_sell_interested

		// Flags & status (Cols 41, 45-46)
		CheckinReplied: safeBool(row, 41), // checkin_replied
		BotActive:      safeBool(row, 45), // Bot_Active
		Blacklisted:    safeBool(row, 46), // blacklisted

		// Other (Cols 19, 43)
		LastInteractionDate: safeDate(row, 19),   // Last_Interaction_Date
		QuotationLink:       safeString(row, 43), // quotation_link
	}

	// Use Contract_Start as ActivationDate
	c.ActivationDate = c.ContractStart

	// Fields not in Master Client: Renewed, Rejected, ResponseStatus
	// ResponseStatus is in Conversation State sheet (Col 6)

	// Invoice reminder flags (Cols 27-33)
	c.Pre14Sent = safeBool(row, 27)
	c.Pre7Sent = safeBool(row, 28)
	c.Pre3Sent = safeBool(row, 29)
	c.Post1Sent = safeBool(row, 30)
	c.Post4Sent = safeBool(row, 31)
	c.Post8Sent = safeBool(row, 32)
	c.Post15Sent = safeBool(row, 33)

	return c, nil
}

// clientToRow converts entity.Client to a Sheets row.
// Must match parseClientRow column order (47 columns).
func clientToRow(c entity.Client) []interface{} {
	// Create a 47-column row (0-46)
	row := make([]interface{}, 47)

	// Core identity (Cols 0-3)
	row[0] = c.CompanyID
	row[1] = c.CompanyName
	row[2] = c.PICName
	row[3] = c.PICWA
	// Col 4: PIC_Email

	// Segment & contract
	row[8] = c.ContractMonths // Next_Discount_%_Manual
	row[10] = c.Segment       // Segment
	row[11] = c.ContractStart.Format("2006-01-02")
	row[12] = c.ContractEnd.Format("2006-01-02")

	// Payment & scores
	row[15] = c.PaymentStatus
	row[17] = c.NPSScore
	row[18] = c.UsageScore
	row[19] = c.LastInteractionDate.Format("2006-01-02")

	// Owner info
	row[21] = c.OwnerName
	row[22] = c.OwnerWA
	row[44] = c.OwnerTelegramID

	// Cross-sell state
	row[24] = c.SequenceCS
	row[25] = c.CrossSellRejected
	row[26] = c.CrossSellInterested

	// Flags
	row[41] = c.CheckinReplied
	row[43] = c.QuotationLink
	row[45] = c.BotActive
	row[46] = c.Blacklisted

	// Invoice reminder flags (Cols 27-33)
	row[27] = c.Pre14Sent
	row[28] = c.Pre7Sent
	row[29] = c.Pre3Sent
	row[30] = c.Post1Sent
	row[31] = c.Post4Sent
	row[32] = c.Post8Sent
	row[33] = c.Post15Sent

	return row
}

func safeString(row []interface{}, idx int) string {
	if idx >= len(row) || row[idx] == nil {
		return ""
	}
	return fmt.Sprintf("%v", row[idx])
}

func safeInt(row []interface{}, idx int) int {
	s := safeString(row, idx)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func safeBool(row []interface{}, idx int) bool {
	s := safeString(row, idx)
	if s == "" {
		return false
	}
	v, _ := strconv.ParseBool(s)
	return v
}

func safeDate(row []interface{}, idx int) time.Time {
	s := safeString(row, idx)
	if s == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func safeFloat(row []interface{}, idx int) float64 {
	s := safeString(row, idx)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// UpdateInvoiceReminderFlags updates the Pre*/Post* flags for a client
func (r *clientRepo) UpdateInvoiceReminderFlags(ctx context.Context, companyID string, flags map[string]bool) error {
	clients, err := r.GetAll(ctx)
	if err != nil {
		return err
	}

	for i, c := range clients {
		if c.CompanyID == companyID {
			rowIdx := i + 6 // 5 header rows + 1 for 1-indexed

			flagCols := map[string]int{
				"pre14_sent":  27, // AB
				"pre7_sent":   28, // AC
				"pre3_sent":   29, // AD
				"post1_sent":  30, // AE
				"post4_sent":  31, // AF
				"post8_sent":  32, // AG
				"post15_sent": 33, // AH
			}

			for flagName, value := range flags {
				if colIdx, ok := flagCols[flagName]; ok {
					colLetter := columnLetter(colIdx)
					cellRange := fmt.Sprintf("'%1. Master Client'!%s%d", colLetter, rowIdx)
					if err := r.sheetsClient.UpdateCell(ctx, cellRange, value); err != nil {
						return fmt.Errorf("failed to update flag %s: %w", flagName, err)
					}
				}
			}
			return r.cache.Invalidate(ctx, cache.KeyMasterClient)
		}
	}

	return fmt.Errorf("client not found: %s", companyID)
}
