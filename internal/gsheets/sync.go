package gsheets

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/sheets/v4"

	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

// ErrNoSplitMetadata means the tab exists but has not been tagged for assignment sync.
var ErrNoSplitMetadata = errors.New("sheet has no splitbot assignment metadata")

// TabURL is the browser link for a tab in the configured spreadsheet.
func (c *Client) TabURL(gid int64) string {
	return TabURL(c.spreadsheetID, strconv.FormatInt(gid, 10))
}

// TabURL builds a Google Sheets URL for a workbook tab.
func TabURL(spreadsheetID, gid string) string {
	spreadsheetID = strings.TrimSpace(spreadsheetID)
	gid = strings.TrimSpace(gid)
	if spreadsheetID == "" || gid == "" {
		return ""
	}
	return fmt.Sprintf("https://docs.google.com/spreadsheets/d/%s/edit#gid=%s", spreadsheetID, gid)
}

// ReadAssignments loads item→person assignments from checkbox cells using developer metadata.
func (c *Client) ReadAssignments(ctx context.Context, gidStr string) (map[string][]string, error) {
	gid, ok := parseGID(gidStr)
	if !ok {
		return nil, fmt.Errorf("invalid sheet gid %q", gidStr)
	}
	ss, err := c.sheets.Spreadsheets.Get(c.spreadsheetID).Context(ctx).Fields("spreadsheetId,sheets.properties").Do()
	if err != nil {
		return nil, fmt.Errorf("load spreadsheet: %w", err)
	}
	props := tabByGID(ss, gid)
	if props == nil {
		return nil, fmt.Errorf("sheet tab gid %d not found", gid)
	}

	entries, err := c.listSplitMetadata(ctx, gid)
	if err != nil {
		return nil, err
	}
	meta := metaFromEntries(entries)
	if len(meta.ItemIDs) == 0 || len(meta.PersonKeys) == 0 {
		return nil, ErrNoSplitMetadata
	}

	quoted := quoteTab(props.Title)
	vr, err := c.sheets.Spreadsheets.Values.Get(c.spreadsheetID, quoted).
		ValueRenderOption("UNFORMATTED_VALUE").
		Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("read sheet values: %w", err)
	}
	values := valueRows(vr)
	return totals.ParseSheetAssignments(meta, values), nil
}

func valueRows(vr *sheets.ValueRange) [][]any {
	if vr == nil {
		return nil
	}
	out := make([][]any, len(vr.Values))
	for i, row := range vr.Values {
		out[i] = append([]any(nil), row...)
	}
	return out
}
