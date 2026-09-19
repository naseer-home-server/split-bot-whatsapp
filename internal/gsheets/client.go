package gsheets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/naseer2426/split-bot-whatsapp/internal/config"
	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

const spreadsheetMIME = "application/vnd.google-apps.spreadsheet"

// Client talks to Google Drive + Sheets using a service account.
type Client struct {
	folderID string
	sheets   *sheets.Service
	drive    *drive.Service
}

// NewClient builds a Google client from env config. Returns an error if credentials or folder id are missing.
func NewClient(ctx context.Context) (*Client, error) {
	cfg := config.Get().Google
	if cfg.DriveFolderID == "" {
		return nil, fmt.Errorf("GOOGLE_DRIVE_FOLDER_ID is required")
	}
	creds, err := loadCredentials(cfg)
	if err != nil {
		return nil, err
	}
	opts := []option.ClientOption{
		option.WithCredentialsJSON(creds),
		option.WithScopes(sheets.SpreadsheetsScope, drive.DriveScope),
	}
	sheetSvc, err := sheets.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("sheets client: %w", err)
	}
	driveSvc, err := drive.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("drive client: %w", err)
	}
	return &Client{folderID: cfg.DriveFolderID, sheets: sheetSvc, drive: driveSvc}, nil
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

// ExportResult is the created or updated spreadsheet.
type ExportResult struct {
	SpreadsheetID string
	URL           string
}

// Export writes layout into an existing spreadsheet (if id is set and still exists) or creates a new one in the Drive folder.
func (c *Client) Export(ctx context.Context, layout totals.SheetLayout, existingID string) (*ExportResult, error) {
	id := strings.TrimSpace(existingID)
	if id != "" {
		_, err := c.sheets.Spreadsheets.Get(id).Context(ctx).Fields("spreadsheetId").Do()
		if err != nil {
			id = ""
		}
	}
	if id == "" {
		created, err := c.createSpreadsheet(ctx, layout.Title)
		if err != nil {
			return nil, err
		}
		id = created
	} else {
		_, _ = c.drive.Files.Update(id, &drive.File{Name: layout.Title}).Context(ctx).SupportsAllDrives(true).Do()
	}

	if err := c.writeLayout(ctx, id, layout); err != nil {
		return nil, err
	}
	_ = c.shareAnyoneWithLink(ctx, id)

	url := fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit", id)
	file, err := c.drive.Files.Get(id).Context(ctx).SupportsAllDrives(true).Fields("webViewLink").Do()
	if err == nil && file.WebViewLink != "" {
		url = file.WebViewLink
	}
	return &ExportResult{SpreadsheetID: id, URL: url}, nil
}

func (c *Client) createSpreadsheet(ctx context.Context, title string) (string, error) {
	file := &drive.File{
		Name:     title,
		MimeType: spreadsheetMIME,
		Parents:  []string{c.folderID},
	}
	created, err := c.drive.Files.Create(file).Context(ctx).SupportsAllDrives(true).Fields("id").Do()
	if err != nil {
		return "", fmt.Errorf("create spreadsheet in folder: %w", err)
	}
	if created.Id == "" {
		return "", fmt.Errorf("create spreadsheet: empty id")
	}
	return created.Id, nil
}

func (c *Client) writeLayout(ctx context.Context, spreadsheetID string, layout totals.SheetLayout) error {
	ss, err := c.sheets.Spreadsheets.Get(spreadsheetID).Context(ctx).Fields("sheets.properties").Do()
	if err != nil {
		return fmt.Errorf("load spreadsheet: %w", err)
	}
	if len(ss.Sheets) == 0 {
		return fmt.Errorf("spreadsheet has no sheets")
	}
	props := ss.Sheets[0].Properties
	sheetID := props.SheetId
	tabName := props.Title
	if tabName == "" {
		tabName = "Sheet1"
	}

	_, err = c.sheets.Spreadsheets.Values.Clear(spreadsheetID, tabName, &sheets.ClearValuesRequest{}).Context(ctx).Do()
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
	_, err = c.sheets.Spreadsheets.Values.Update(spreadsheetID, tabName+"!A1", &sheets.ValueRange{
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
		{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{SheetId: sheetID, Title: "Split"},
				Fields:     "title",
			},
		},
		repeatNumberFormat(sheetID, 1, int64(layout.TaxTotalRow), 1, 2, money),
		repeatNumberFormat(sheetID, 1, int64(layout.DiscountRow), 3, 4, money),
	}
	if layout.PersonCount > 0 && layout.CheckboxRange.EndColumn > layout.CheckboxRange.StartColumn {
		r := layout.CheckboxRange
		reqs = append(reqs, &sheets.Request{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: grid(sheetID, int64(r.StartRow), int64(r.EndRow), int64(r.StartColumn), int64(r.EndColumn)),
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
			sheetID,
			int64(layout.TotalRow-1),
			int64(layout.TaxTotalRow),
			int64(r.StartColumn),
			int64(r.EndColumn),
			money,
		))
	}

	_, err = c.sheets.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("format sheet: %w", err)
	}
	return nil
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

func (c *Client) shareAnyoneWithLink(ctx context.Context, fileID string) error {
	_, err := c.drive.Permissions.Create(fileID, &drive.Permission{
		Type: "anyone",
		Role: "writer",
	}).Context(ctx).SupportsAllDrives(true).Do()
	if err != nil {
		return fmt.Errorf("share spreadsheet: %w", err)
	}
	return nil
}
