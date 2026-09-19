package gsheets

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/naseer2426/split-bot-whatsapp/internal/config"
	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

// Client writes tabs into a user-owned spreadsheet using a service account.
type Client struct {
	spreadsheetID string
	sheets        *sheets.Service
}

// NewClient builds a Google Sheets client from env config.
func NewClient(ctx context.Context) (*Client, error) {
	cfg := config.Get().Google
	if cfg.SpreadsheetID == "" {
		return nil, fmt.Errorf("GOOGLE_SPREADSHEET_ID is required")
	}
	creds, err := loadCredentials(cfg)
	if err != nil {
		return nil, err
	}
	sheetSvc, err := sheets.NewService(ctx,
		option.WithCredentialsJSON(creds),
		option.WithScopes(sheets.SpreadsheetsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("sheets client: %w", err)
	}
	return &Client{spreadsheetID: cfg.SpreadsheetID, sheets: sheetSvc}, nil
}

func loadCredentials(cfg config.GoogleConfig) ([]byte, error) {
	raw := strings.TrimSpace(cfg.ServiceAccountJSON)
	if raw != "" {
		if strings.HasPrefix(raw, "{") {
			return []byte(raw), nil
		}
		b, err := os.ReadFile(raw)
		if err != nil {
			return nil, fmt.Errorf("read GOOGLE_SERVICE_ACCOUNT_JSON path: %w", err)
		}
		return b, nil
	}
	if cfg.CredentialsFile != "" {
		b, err := os.ReadFile(cfg.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("read GOOGLE_APPLICATION_CREDENTIALS: %w", err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("GOOGLE_SERVICE_ACCOUNT_JSON or GOOGLE_APPLICATION_CREDENTIALS is required")
}

// ExportResult identifies the workbook tab that was written.
type ExportResult struct {
	SpreadsheetID string
	SheetGID      int64
	URL           string
}

// Export writes layout into an existing tab (by gid) or adds a new tab on the configured spreadsheet.
func (c *Client) Export(ctx context.Context, layout totals.SheetLayout, existingGID string) (*ExportResult, error) {
	ss, err := c.sheets.Spreadsheets.Get(c.spreadsheetID).Context(ctx).Fields("spreadsheetId,sheets.properties").Do()
	if err != nil {
		return nil, fmt.Errorf("load spreadsheet: %w", err)
	}

	title := sanitizeTabTitle(layout.Title)
	var gid int64
	var tabName string

	if parsed, ok := parseGID(existingGID); ok {
		if props := tabByGID(ss, parsed); props != nil {
			gid = props.SheetId
			tabName = props.Title
			if tabName != title && !tabTitleTaken(ss, title, gid) {
				if err := c.renameTab(ctx, gid, title); err == nil {
					tabName = title
				}
			}
		}
	}
	if tabName == "" {
		title = uniqueTabTitle(ss, title)
		gid, tabName, err = c.addTab(ctx, title)
		if err != nil {
			return nil, err
		}
	}

	if err := c.writeLayout(ctx, gid, tabName, layout); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit#gid=%d", c.spreadsheetID, gid)
	return &ExportResult{SpreadsheetID: c.spreadsheetID, SheetGID: gid, URL: url}, nil
}

func (c *Client) addTab(ctx context.Context, title string) (int64, string, error) {
	resp, err := c.sheets.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			AddSheet: &sheets.AddSheetRequest{
				Properties: &sheets.SheetProperties{Title: title},
			},
		}},
	}).Context(ctx).Do()
	if err != nil {
		return 0, "", fmt.Errorf("add sheet tab: %w", err)
	}
	if len(resp.Replies) == 0 || resp.Replies[0].AddSheet == nil || resp.Replies[0].AddSheet.Properties == nil {
		return 0, "", fmt.Errorf("add sheet tab: empty reply")
	}
	props := resp.Replies[0].AddSheet.Properties
	return props.SheetId, props.Title, nil
}

func (c *Client) renameTab(ctx context.Context, gid int64, title string) error {
	_, err := c.sheets.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{SheetId: gid, Title: title},
				Fields:     "title",
			},
		}},
	}).Context(ctx).Do()
	return err
}

func (c *Client) writeLayout(ctx context.Context, gid int64, tabName string, layout totals.SheetLayout) error {
	quoted := quoteTab(tabName)
	_, err := c.sheets.Spreadsheets.Values.Clear(c.spreadsheetID, quoted, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("clear sheet: %w", err)
	}

	values := make([][]interface{}, len(layout.Values))
	for i, row := range layout.Values {
		if row == nil {
			values[i] = []interface{}{}
			continue
		}
		values[i] = append([]interface{}(nil), row...)
	}
	_, err = c.sheets.Spreadsheets.Values.Update(c.spreadsheetID, quoted+"!A1", &sheets.ValueRange{
		Values: values,
	}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write values: %w", err)
	}

	money := &sheets.CellData{
		UserEnteredFormat: &sheets.CellFormat{
			NumberFormat: &sheets.NumberFormat{Type: "NUMBER", Pattern: "#,##0.00"},
		},
	}
	reqs := []*sheets.Request{
		repeatNumberFormat(gid, 1, int64(layout.TaxTotalRow), 1, 2, money),
		repeatNumberFormat(gid, 1, int64(layout.DiscountRow), 3, 4, money),
	}
	if layout.PersonCount > 0 && layout.CheckboxRange.EndColumn > layout.CheckboxRange.StartColumn {
		r := layout.CheckboxRange
		reqs = append(reqs, &sheets.Request{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: grid(gid, int64(r.StartRow), int64(r.EndRow), int64(r.StartColumn), int64(r.EndColumn)),
				Cell: &sheets.CellData{
					DataValidation: &sheets.DataValidationRule{
						Condition:    &sheets.BooleanCondition{Type: "BOOLEAN"},
						ShowCustomUi: true,
						Strict:       true,
					},
				},
				Fields: "dataValidation",
			},
		})
		reqs = append(reqs, repeatNumberFormat(
			gid,
			int64(layout.TotalRow-1),
			int64(layout.TaxTotalRow),
			int64(r.StartColumn),
			int64(r.EndColumn),
			money,
		))
	}

	_, err = c.sheets.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("format sheet: %w", err)
	}
	return nil
}

func parseGID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func tabByGID(ss *sheets.Spreadsheet, gid int64) *sheets.SheetProperties {
	for _, s := range ss.Sheets {
		if s.Properties != nil && s.Properties.SheetId == gid {
			return s.Properties
		}
	}
	return nil
}

func tabTitleTaken(ss *sheets.Spreadsheet, title string, exceptGID int64) bool {
	for _, s := range ss.Sheets {
		if s.Properties != nil && s.Properties.Title == title && s.Properties.SheetId != exceptGID {
			return true
		}
	}
	return false
}

func uniqueTabTitle(ss *sheets.Spreadsheet, base string) string {
	if !tabTitleTaken(ss, base, -1) {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)", base, i)
		if !tabTitleTaken(ss, candidate, -1) {
			return candidate
		}
	}
}

func sanitizeTabTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Bill split"
	}
	replacer := strings.NewReplacer(":", " ", "\\", " ", "/", " ", "?", " ", "*", " ", "[", " ", "]", " ")
	title = strings.Join(strings.Fields(replacer.Replace(title)), " ")
	if len(title) > 100 {
		title = title[:100]
	}
	return title
}

func quoteTab(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

func grid(sheetID, startRow, endRow, startCol, endCol int64) *sheets.GridRange {
	return &sheets.GridRange{
		SheetId:          sheetID,
		StartRowIndex:    startRow,
		EndRowIndex:      endRow,
		StartColumnIndex: startCol,
		EndColumnIndex:   endCol,
	}
}

func repeatNumberFormat(sheetID, startRow, endRow, startCol, endCol int64, cell *sheets.CellData) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range:  grid(sheetID, startRow, endRow, startCol, endCol),
			Cell:   cell,
			Fields: "userEnteredFormat.numberFormat",
		},
	}
}
