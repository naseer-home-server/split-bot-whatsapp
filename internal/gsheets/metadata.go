package gsheets

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/api/sheets/v4"

	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

func (c *Client) replaceSplitMetadata(ctx context.Context, gid int64, layout totals.SheetLayout) error {
	existing, err := c.listSplitMetadata(ctx, gid)
	if err != nil {
		return err
	}
	reqs := deleteMetadataRequests(existing)
	reqs = append(reqs, createMetadataRequests(gid, layout)...)
	if len(reqs) == 0 {
		return nil
	}
	_, err = c.sheets.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: reqs,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("write sheet metadata: %w", err)
	}
	return nil
}

func (c *Client) listSplitMetadata(ctx context.Context, gid int64) ([]*sheets.DeveloperMetadata, error) {
	out := make([]*sheets.DeveloperMetadata, 0)
	for _, key := range []string{totals.SheetMetaTotalsID, totals.SheetMetaItemID, totals.SheetMetaPersonKey} {
		resp, err := c.sheets.Spreadsheets.DeveloperMetadata.Search(c.spreadsheetID, &sheets.SearchDeveloperMetadataRequest{
			DataFilters: []*sheets.DataFilter{{
				DeveloperMetadataLookup: &sheets.DeveloperMetadataLookup{
					MetadataKey: key,
				},
			}},
		}).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("search metadata %s: %w", key, err)
		}
		if resp == nil {
			continue
		}
		for _, matched := range resp.MatchedDeveloperMetadata {
			if matched == nil || matched.DeveloperMetadata == nil {
				continue
			}
			md := matched.DeveloperMetadata
			if metadataOnSheet(md, gid) {
				out = append(out, md)
			}
		}
	}
	return out, nil
}

func deleteMetadataRequests(existing []*sheets.DeveloperMetadata) []*sheets.Request {
	reqs := make([]*sheets.Request, 0, len(existing))
	seen := make(map[int64]struct{}, len(existing))
	for _, md := range existing {
		if md == nil || md.MetadataId == 0 {
			continue
		}
		if _, ok := seen[md.MetadataId]; ok {
			continue
		}
		seen[md.MetadataId] = struct{}{}
		id := md.MetadataId
		reqs = append(reqs, &sheets.Request{
			DeleteDeveloperMetadata: &sheets.DeleteDeveloperMetadataRequest{
				DataFilter: &sheets.DataFilter{
					DeveloperMetadataLookup: &sheets.DeveloperMetadataLookup{
						MetadataId: id,
					},
				},
			},
		})
	}
	return reqs
}

func createMetadataRequests(gid int64, layout totals.SheetLayout) []*sheets.Request {
	reqs := make([]*sheets.Request, 0, 1+len(layout.ItemIDs)+len(layout.People))
	if layout.TotalsID > 0 {
		reqs = append(reqs, createMeta(sheetLocation(gid), totals.SheetMetaTotalsID, strconv.Itoa(layout.TotalsID)))
	}
	for i, id := range layout.ItemIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		row := int64(1 + i) // 0-based; row 0 is the header
		reqs = append(reqs, createMeta(dimensionLocation(gid, "ROWS", row), totals.SheetMetaItemID, id))
	}
	for i, p := range layout.People {
		key := strings.TrimSpace(p.Key)
		if key == "" {
			continue
		}
		col := int64(totals.SheetFirstPersonCol() + i)
		reqs = append(reqs, createMeta(dimensionLocation(gid, "COLUMNS", col), totals.SheetMetaPersonKey, key))
	}
	return reqs
}

func createMeta(loc *sheets.DeveloperMetadataLocation, key, value string) *sheets.Request {
	return &sheets.Request{
		CreateDeveloperMetadata: &sheets.CreateDeveloperMetadataRequest{
			DeveloperMetadata: &sheets.DeveloperMetadata{
				MetadataKey:   key,
				MetadataValue: value,
				Visibility:    "DOCUMENT",
				Location:      loc,
			},
		},
	}
}

func sheetLocation(gid int64) *sheets.DeveloperMetadataLocation {
	loc := &sheets.DeveloperMetadataLocation{SheetId: gid}
	if gid == 0 {
		loc.ForceSendFields = []string{"SheetId"}
	}
	return loc
}

func dimensionLocation(gid int64, dimension string, index int64) *sheets.DeveloperMetadataLocation {
	r := &sheets.DimensionRange{
		SheetId:    gid,
		Dimension:  dimension,
		StartIndex: index,
		EndIndex:   index + 1,
	}
	if gid == 0 {
		r.ForceSendFields = []string{"SheetId"}
	}
	return &sheets.DeveloperMetadataLocation{DimensionRange: r}
}

func metadataOnSheet(md *sheets.DeveloperMetadata, gid int64) bool {
	if md == nil || md.Location == nil {
		return false
	}
	loc := md.Location
	if loc.DimensionRange != nil {
		return loc.DimensionRange.SheetId == gid
	}
	return loc.SheetId == gid
}

func metaFromEntries(entries []*sheets.DeveloperMetadata) totals.SheetDimensionMeta {
	meta := totals.SheetDimensionMeta{
		ItemIDs:    make(map[int]string),
		PersonKeys: make(map[int]string),
	}
	for _, md := range entries {
		if md == nil || md.Location == nil || md.Location.DimensionRange == nil {
			continue
		}
		idx := int(md.Location.DimensionRange.StartIndex)
		switch md.MetadataKey {
		case totals.SheetMetaItemID:
			meta.ItemIDs[idx] = md.MetadataValue
		case totals.SheetMetaPersonKey:
			meta.PersonKeys[idx] = md.MetadataValue
		}
	}
	return meta
}
