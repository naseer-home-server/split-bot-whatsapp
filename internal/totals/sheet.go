package totals

import (
	"fmt"
	"sort"
	"strings"
)

const (
	sheetColItem      = 0
	sheetColPrice     = 1
	sheetColDivided   = 2
	sheetColPerPerson = 3
	sheetFirstPerson  = 4
)

// SheetFirstPersonCol is the 0-based spreadsheet column of the first person checkbox.
func SheetFirstPersonCol() int {
	return sheetFirstPerson
}

// Participant is one person column in the exported sheet.
type Participant struct {
	Key  string // WhatsApp user id, or extra name if they are not in assignments
	Name string // display header
}

// SheetLayout is a Google Sheet spec with live formulas and checkbox cells.
type SheetLayout struct {
	Title          string
	Values         [][]any
	CheckboxRange  GridRange // item + discount rows, person columns
	ItemRowCount   int
	DiscountRow    int // 1-based
	TotalRow       int
	TaxTotalRow    int
	TaxProductCell string // e.g. $B$8*$B$9, or 1 if no taxes
	TaxStartRow    int    // 1-based first tax row; 0 if none
	LastDataRow    int    // 1-based last row with values
	PersonCount    int
	LastColumn     int // exclusive, 0-based
	TotalsID       int
	People         []Participant
	ItemIDs        []string // item row order; spreadsheet row 2 is index 0
}

// GridRange is a 0-based half-open range (Sheets API style).
type GridRange struct {
	StartRow    int
	EndRow      int
	StartColumn int
	EndColumn   int
}

// CollectParticipants returns assigned people (named, sorted) then extra names that are not already present.
func CollectParticipants(assignments map[string][]string, extra []string, nameOf func(string) string) []Participant {
	if nameOf == nil {
		nameOf = func(id string) string { return id }
	}
	seenKey := make(map[string]struct{})
	seenName := make(map[string]struct{})
	assigned := make([]Participant, 0)
	for _, users := range assignments {
		for _, id := range users {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seenKey[id]; ok {
				continue
			}
			seenKey[id] = struct{}{}
			name := strings.TrimSpace(nameOf(id))
			if name == "" {
				name = id
			}
			assigned = append(assigned, Participant{Key: id, Name: name})
			seenName[strings.ToLower(name)] = struct{}{}
			seenName[strings.ToLower(id)] = struct{}{}
		}
	}
	sort.Slice(assigned, func(i, j int) bool {
		if assigned[i].Name == assigned[j].Name {
			return assigned[i].Key < assigned[j].Key
		}
		return assigned[i].Name < assigned[j].Name
	})

	out := append([]Participant(nil), assigned...)
	for _, raw := range extra {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if _, ok := seenKey[name]; ok {
			continue
		}
		if _, ok := seenName[lower]; ok {
			continue
		}
		seenKey[name] = struct{}{}
		seenName[lower] = struct{}{}
		out = append(out, Participant{Key: name, Name: name})
	}
	return applyFirstNames(out)
}

func firstName(full string) string {
	fields := strings.Fields(strings.TrimSpace(full))
	if len(fields) == 0 {
		return strings.TrimSpace(full)
	}
	return fields[0]
}

func applyFirstNames(people []Participant) []Participant {
	used := make(map[string]struct{}, len(people))
	out := make([]Participant, len(people))
	for i, p := range people {
		name := disambiguateFirstName(p.Name, used)
		out[i] = Participant{Key: p.Key, Name: name}
	}
	return out
}

func disambiguateFirstName(full string, used map[string]struct{}) string {
	first := firstName(full)
	if first == "" {
		first = strings.TrimSpace(full)
	}
	if markUnused(first, used) {
		return first
	}
	fields := strings.Fields(strings.TrimSpace(full))
	if len(fields) >= 2 {
		runes := []rune(fields[1])
		if len(runes) > 0 {
			candidate := first + " " + string(runes[0])
			if markUnused(candidate, used) {
				return candidate
			}
		}
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s %d", first, i)
		if markUnused(candidate, used) {
			return candidate
		}
	}
}

func markUnused(name string, used map[string]struct{}) bool {
	key := strings.ToLower(name)
	if _, ok := used[key]; ok {
		return false
	}
	used[key] = struct{}{}
	return true
}

// BuildSheetLayout builds values (formulas as strings starting with =) for a live split sheet.
func BuildSheetLayout(title string, items []UnitItem, tax []Tax, discount float64, totalInBill float64, assignments map[string][]string, people []Participant) SheetLayout {
	if title == "" {
		title = "Bill split"
	}
	if assignments == nil {
		assignments = map[string][]string{}
	}
	nItems := len(items)
	nPeople := len(people)
	itemStart := 2 // 1-based
	itemEnd := 1 + nItems
	if nItems == 0 {
		itemEnd = 1
	}
	discountRow := itemEnd + 1
	if nItems == 0 {
		discountRow = 2
	}
	totalRow := discountRow + 1
	taxTotalRow := totalRow + 1
	taxStartRow := taxTotalRow + 2
	taxFactor := taxFactorExpr(tax, taxStartRow)
	if len(tax) == 0 {
		taxStartRow = 0
	}

	lastPersonCol := sheetFirstPerson + nPeople - 1
	header := []any{"Item", "Price", "Divided by", "Per person"}
	for _, p := range people {
		header = append(header, p.Name)
	}

	rows := make([][]any, 0, taxTotalRow+len(tax)+3)
	rows = append(rows, header)

	assignedOn := func(itemID, userKey string) bool {
		for _, u := range assignments[itemID] {
			if u == userKey {
				return true
			}
		}
		return false
	}

	for i, item := range items {
		rowNum := itemStart + i
		label := item.PollOption
		if strings.TrimSpace(label) == "" {
			label = item.Name
		}
		row := []any{label, roundMoney(item.LineTotal())}
		if nPeople > 0 {
			row = append(row, dividedByFormula(rowNum, nPeople), perPersonFormula(rowNum))
			for _, p := range people {
				row = append(row, assignedOn(item.ID, p.Key))
			}
		} else {
			row = append(row, 0, 0)
		}
		rows = append(rows, row)
	}

	discRow := []any{"Discount", roundMoney(-discount)}
	if nPeople > 0 {
		discRow = append(discRow, dividedByFormula(discountRow, nPeople), perPersonFormula(discountRow))
		for range people {
			discRow = append(discRow, true)
		}
	} else {
		discRow = append(discRow, 0, 0)
	}
	rows = append(rows, discRow)

	totalRowVals := []any{"Total", fmt.Sprintf("=SUM(B%d:B%d)", itemStart, max(itemEnd, itemStart))}
	if nPeople > 0 {
		totalRowVals = append(totalRowVals, "", "")
		for i := range people {
			col := colLetter(sheetFirstPerson + i + 1)
			totalRowVals = append(totalRowVals, foodTotalFormula(col, itemStart, itemEnd))
		}
	} else {
		totalRowVals = append(totalRowVals, "", "")
	}
	rows = append(rows, totalRowVals)

	taxRowVals := []any{"Total with tax", fmt.Sprintf("=B%d*%s+B%d", totalRow, taxFactor, discountRow)}
	if nPeople > 0 {
		taxRowVals = append(taxRowVals, "", "")
		for i := range people {
			col := colLetter(sheetFirstPerson + i + 1)
			taxRowVals = append(taxRowVals, taxTotalFormula(col, totalRow, discountRow, taxFactor))
		}
	} else {
		taxRowVals = append(taxRowVals, "", "")
	}
	rows = append(rows, taxRowVals)

	rows = append(rows, nil)
	for _, t := range tax {
		label := strings.TrimSpace(t.Name)
		if label == "" {
			label = "Tax"
		}
		rows = append(rows, []any{label, t.Multiplier})
	}
	rows = append(rows, []any{"Bill total", roundMoney(totalInBill)})

	layout := SheetLayout{
		Title:          title,
		Values:         rows,
		ItemRowCount:   nItems,
		DiscountRow:    discountRow,
		TotalRow:       totalRow,
		TaxTotalRow:    taxTotalRow,
		TaxProductCell: taxFactor,
		TaxStartRow:    taxStartRow,
		LastDataRow:    taxTotalRow + 1 + len(tax) + 1,
		PersonCount:    nPeople,
		LastColumn:     4 + nPeople,
		People:         append([]Participant(nil), people...),
		ItemIDs:        itemIDsOf(items),
	}
	if nPeople > 0 && nItems+1 > 0 {
		endRow := discountRow // 1-based inclusive discount
		layout.CheckboxRange = GridRange{
			StartRow:    1, // 0-based, skip header
			EndRow:      endRow,
			StartColumn: sheetFirstPerson,
			EndColumn:   lastPersonCol + 1,
		}
	}
	return layout
}

func dividedByFormula(row, nPeople int) string {
	start := colLetter(sheetFirstPerson + 1)
	end := colLetter(sheetFirstPerson + nPeople)
	return fmt.Sprintf("=COUNTIF(%s%d:%s%d,TRUE)", start, row, end, row)
}

func perPersonFormula(row int) string {
	return fmt.Sprintf("=IF(C%d=0,0,B%d/C%d)", row, row, row)
}

func foodTotalFormula(col string, itemStart, itemEnd int) string {
	if itemEnd < itemStart {
		itemEnd = itemStart
	}
	return fmt.Sprintf("=SUMPRODUCT((%s%d:%s%d=TRUE)*$D$%d:$D$%d)", col, itemStart, col, itemEnd, itemStart, itemEnd)
}

func taxTotalFormula(col string, totalRow, discountRow int, taxFactor string) string {
	return fmt.Sprintf("=%s%d*%s+IF(%s%d=TRUE,$D$%d,0)", col, totalRow, taxFactor, col, discountRow, discountRow)
}

func taxFactorExpr(tax []Tax, firstRow int) string {
	if len(tax) == 0 {
		return "1"
	}
	parts := make([]string, 0, len(tax))
	for i := range tax {
		parts = append(parts, fmt.Sprintf("$B$%d", firstRow+i))
	}
	return strings.Join(parts, "*")
}

// ColLetter returns the 1-based A1 column name (1=A, 27=AA).
func ColLetter(n int) string {
	return colLetter(n)
}

func colLetter(n int) string {
	if n <= 0 {
		return ""
	}
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func itemIDsOf(items []UnitItem) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}
