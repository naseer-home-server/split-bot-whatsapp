package totals

import (
	"errors"
	"strings"
)

const (
	// SheetMetaTotalsID is developer metadata on the exported tab.
	SheetMetaTotalsID = "splitbot_totals_id"
	// SheetMetaItemID is developer metadata on each item row.
	SheetMetaItemID = "splitbot_item_id"
	// SheetMetaPersonKey is developer metadata on each person column.
	SheetMetaPersonKey = "splitbot_person_key"
)

// SheetDimensionMeta maps 0-based spreadsheet coordinates to assignment keys.
type SheetDimensionMeta struct {
	ItemIDs    map[int]string // row -> item id
	PersonKeys map[int]string // column -> person key
}

// ErrSheetAssignmentsLocked is returned when assignments cannot be changed because a sheet already exists.
var ErrSheetAssignmentsLocked = errors.New("this bill has been exported to Google Sheets; the sheet is the source of truth for assignments and cannot be changed here. Update the checkboxes on the sheet instead")

// HasSheetExport is true when a totals row has been written to Google Sheets.
func HasSheetExport(sheetID *string) bool {
	return sheetID != nil && strings.TrimSpace(*sheetID) != ""
}

// ParseSheetAssignments reads TRUE checkboxes using item-row and person-column metadata.
func ParseSheetAssignments(meta SheetDimensionMeta, values [][]any) map[string][]string {
	out := make(map[string][]string)
	if len(meta.ItemIDs) == 0 || len(meta.PersonKeys) == 0 {
		return out
	}
	for row, itemID := range meta.ItemIDs {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		users := make([]string, 0)
		for col, key := range meta.PersonKeys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			if checkboxTrue(cellAt(values, row, col)) {
				users = append(users, key)
			}
		}
		cleaned, err := cleanUserIDs(users)
		if err != nil || len(cleaned) == 0 {
			continue
		}
		out[itemID] = cleaned
	}
	return out
}

// AssignmentsFromSheet keeps only item ids that exist on the bill.
func AssignmentsFromSheet(units []UnitItem, parsed map[string][]string) map[string][]string {
	valid := itemIDs(units)
	out := make(map[string][]string, len(valid))
	for id, users := range parsed {
		if _, ok := valid[id]; !ok {
			continue
		}
		if len(users) == 0 {
			continue
		}
		out[id] = append([]string(nil), users...)
	}
	return out
}

func cellAt(values [][]any, row, col int) any {
	if row < 0 || col < 0 || row >= len(values) {
		return nil
	}
	line := values[row]
	if col >= len(line) {
		return nil
	}
	return line[col]
}

func checkboxTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "yes" || s == "1"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}
