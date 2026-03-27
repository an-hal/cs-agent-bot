package sheets

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/config"
	"github.com/rs/zerolog"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type SheetsClient struct {
	service       *sheets.Service
	spreadsheetID string
	logger        zerolog.Logger
}

func NewSheetsClient(cfg *config.AppConfig, logger zerolog.Logger) (*SheetsClient, error) {
	ctx := context.Background()

	srv, err := sheets.NewService(ctx, option.WithCredentialsFile(cfg.GoogleSAKeyFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	logger.Info().Msg("Google Sheets client initialized")

	return &SheetsClient{
		service:       srv,
		spreadsheetID: cfg.SpreadsheetID,
		logger:        logger,
	}, nil
}

// ReadRange reads a range from the spreadsheet and returns raw row data.
func (s *SheetsClient) ReadRange(ctx context.Context, sheetRange string) ([][]interface{}, error) {
	resp, err := s.service.Spreadsheets.Values.Get(s.spreadsheetID, sheetRange).
		Context(ctx).
		ValueRenderOption("UNFORMATTED_VALUE").
		DateTimeRenderOption("FORMATTED_STRING").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to read range %s: %w", sheetRange, err)
	}

	return resp.Values, nil
}

// WriteRange overwrites a range with the given values.
func (s *SheetsClient) WriteRange(ctx context.Context, sheetRange string, values [][]interface{}) error {
	vr := &sheets.ValueRange{
		Values: values,
	}

	_, err := s.service.Spreadsheets.Values.Update(s.spreadsheetID, sheetRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to write range %s: %w", sheetRange, err)
	}

	return nil
}

// AppendRows appends rows to the end of a sheet range.
func (s *SheetsClient) AppendRows(ctx context.Context, sheetRange string, values [][]interface{}) error {
	vr := &sheets.ValueRange{
		Values: values,
	}

	_, err := s.service.Spreadsheets.Values.Append(s.spreadsheetID, sheetRange, vr).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to append to range %s: %w", sheetRange, err)
	}

	return nil
}

// UpdateCell updates a single cell value.
func (s *SheetsClient) UpdateCell(ctx context.Context, cellRange string, value interface{}) error {
	vr := &sheets.ValueRange{
		Values: [][]interface{}{{value}},
	}

	_, err := s.service.Spreadsheets.Values.Update(s.spreadsheetID, cellRange, vr).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to update cell %s: %w", cellRange, err)
	}

	return nil
}
