package totals

import (
	"strings"
	"testing"
)

func TestColLetter(t *testing.T) {
	t.Parallel()
	cases := map[int]string{1: "A", 4: "D", 5: "E", 26: "Z", 27: "AA", 28: "AB"}
	for n, want := range cases {
		if got := ColLetter(n); got != want {
			t.Errorf("ColLetter(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSheetTabTitle(t *testing.T) {
	t.Parallel()
	if got := SheetTabTitle(12, ""); got != "Bill split #12" {
		t.Fatalf("empty title=%q", got)
	}
	if got := SheetTabTitle(12, "  Sichuan  "); got != "Sichuan" {
		t.Fatalf("named title=%q", got)
	}
}

func TestCollectParticipantsAssignedThenExtra(t *testing.T) {
	t.Parallel()
	assignments := map[string][]string{
		"1": {"lid-b", "lid-a"},
		"2": {"lid-a"},
	}
	nameOf := func(id string) string {
		if id == "lid-a" {
			return "Alice"
		}
		if id == "lid-b" {
			return "Bob"
		}
		return id
	}
	people := CollectParticipants(assignments, []string{"Cara", "Alice", "lid-b"}, nameOf)
	if len(people) != 3 {
		t.Fatalf("len=%d want 3: %+v", len(people), people)
	}
	if people[0].Name != "Alice" || people[1].Name != "Bob" || people[2].Name != "Cara" {
		t.Fatalf("order/names = %+v", people)
	}
	if people[2].Key != "Cara" {
		t.Fatalf("extra key = %q", people[2].Key)
	}
}

func TestParseExtraPerson(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in    string
		key   string
		isLID bool
		ok    bool
	}{
		{"", "", false, false},
		{"  Sam Lee  ", "Sam Lee", false, true},
		{"@60123456789", "60123456789", true, true},
		{"60123456789", "60123456789", true, true},
		{"123@lid", "123", true, true},
		{"@123@lid", "123", true, true},
		{"+6012", "6012", true, true},
	}
	for _, tc := range cases {
		key, isLID, ok := ParseExtraPerson(tc.in)
		if key != tc.key || isLID != tc.isLID || ok != tc.ok {
			t.Errorf("ParseExtraPerson(%q)=%q %v %v want %q %v %v", tc.in, key, isLID, ok, tc.key, tc.isLID, tc.ok)
		}
	}
}

func TestCollectParticipantsExtraLIDLooksUpName(t *testing.T) {
	t.Parallel()
	assignments := map[string][]string{
		"1": {"lid-a"},
	}
	nameOf := func(id string) string {
		switch id {
		case "lid-a":
			return "Alice Smith"
		case "60123456789":
			return "Naseer Ahmed Khan"
		default:
			return id
		}
	}
	people := CollectParticipants(assignments, []string{"@60123456789", "Sam", "lid-a", "Alice Smith"}, nameOf)
	if len(people) != 3 {
		t.Fatalf("len=%d: %+v", len(people), people)
	}
	if people[0].Name != "Alice" || people[0].Key != "lid-a" {
		t.Fatalf("assigned=%+v", people[0])
	}
	if people[1].Name != "Naseer" || people[1].Key != "60123456789" {
		t.Fatalf("extra lid=%+v", people[1])
	}
	if people[2].Name != "Sam" || people[2].Key != "Sam" {
		t.Fatalf("extra name=%+v", people[2])
	}
}

func TestCollectParticipantsUsesFirstNames(t *testing.T) {
	t.Parallel()
	assignments := map[string][]string{
		"1": {"lid-a", "lid-b"},
	}
	nameOf := func(id string) string {
		if id == "lid-a" {
			return "Gulshan Fathima"
		}
		return "Naseer Ahmed Khan"
	}
	people := CollectParticipants(assignments, nil, nameOf)
	if len(people) != 2 {
		t.Fatalf("len=%d: %+v", len(people), people)
	}
	if people[0].Name != "Gulshan" || people[1].Name != "Naseer" {
		t.Fatalf("names=%q %q", people[0].Name, people[1].Name)
	}
}

func TestCollectParticipantsDisambiguatesSameFirstName(t *testing.T) {
	t.Parallel()
	assignments := map[string][]string{
		"1": {"lid-a", "lid-b"},
	}
	nameOf := func(id string) string {
		if id == "lid-a" {
			return "Alex Jones"
		}
		return "Alex Smith"
	}
	people := CollectParticipants(assignments, nil, nameOf)
	if len(people) != 2 {
		t.Fatalf("len=%d: %+v", len(people), people)
	}
	if people[0].Name != "Alex" || people[1].Name != "Alex S" {
		t.Fatalf("names=%q %q", people[0].Name, people[1].Name)
	}
}

func TestBuildSheetLayout(t *testing.T) {
	t.Parallel()
	items := []UnitItem{
		{ID: "1", Name: "Pizza", Quantity: 2, UnitPrice: 18, PollOption: "1. Pizza x2 $18"},
		{ID: "2", Name: "Salad", Quantity: 1, UnitPrice: 12, PollOption: "2. Salad x1 $12"},
	}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	assignments := map[string][]string{
		"1": {"alice"},
		"2": {"alice", "bob"},
	}
	people := []Participant{{Key: "alice", Name: "Alice"}, {Key: "bob", Name: "Bob"}}
	layout := BuildSheetLayout("Dinner", items, tax, 10, 41.8, assignments, people)

	if layout.DiscountRow != 4 || layout.TotalRow != 5 || layout.TaxTotalRow != 6 {
		t.Fatalf("rows discount=%d total=%d tax=%d", layout.DiscountRow, layout.TotalRow, layout.TaxTotalRow)
	}
	if layout.PersonCount != 2 {
		t.Fatalf("people=%d", layout.PersonCount)
	}
	if layout.LastColumn != 6 {
		t.Fatalf("last column=%d", layout.LastColumn)
	}
	if len(layout.ItemIDs) != 2 || layout.ItemIDs[0] != "1" || layout.ItemIDs[1] != "2" {
		t.Fatalf("item ids=%v", layout.ItemIDs)
	}
	if len(layout.People) != 2 || layout.People[0].Key != "alice" {
		t.Fatalf("people=%v", layout.People)
	}

	header := layout.Values[0]
	if header[0] != "Item" || header[1] != "Price" || header[2] != "Divided by" || header[3] != "Per person" || header[4] != "Alice" || header[5] != "Bob" {
		t.Fatalf("header=%v", header)
	}

	pizza := layout.Values[1]
	if pizza[0] != "1. Pizza x2 $18" {
		t.Fatalf("pizza name=%v", pizza[0])
	}
	if pizza[1] != 36.0 {
		t.Fatalf("pizza price=%v", pizza[1])
	}
	if pizza[2] != "=COUNTIF(E2:F2,TRUE)" {
		t.Fatalf("pizza divided=%v", pizza[2])
	}
	if pizza[3] != "=IF(C2=0,0,B2/C2)" {
		t.Fatalf("pizza per=%v", pizza[3])
	}
	if pizza[4] != true || pizza[5] != false {
		t.Fatalf("pizza checks alice=%v bob=%v", pizza[4], pizza[5])
	}

	salad := layout.Values[2]
	if salad[4] != true || salad[5] != true {
		t.Fatalf("salad checks=%v %v", salad[4], salad[5])
	}

	disc := layout.Values[3]
	if disc[0] != "Discount" || disc[1] != -10.0 {
		t.Fatalf("discount=%v", disc)
	}
	if disc[4] != true || disc[5] != true {
		t.Fatalf("discount checks=%v %v", disc[4], disc[5])
	}

	total := layout.Values[4]
	if !strings.HasPrefix(fmtString(total[1]), "=SUM(B2:B3)") {
		t.Fatalf("total price formula=%v", total[1])
	}
	if total[4] != "=SUMPRODUCT((E2:E3=TRUE)*$D$2:$D$3)" {
		t.Fatalf("alice food=%v", total[4])
	}

	withTax := layout.Values[5]
	if withTax[1] != "=B5*$B$8+B4" {
		t.Fatalf("total with tax formula=%v", withTax[1])
	}
	if withTax[4] != "=E5*$B$8+IF(E4=TRUE,$D$4,0)" {
		t.Fatalf("alice tax total=%v (tax factor %s)", withTax[4], layout.TaxProductCell)
	}
	if layout.TaxProductCell != "$B$8" {
		t.Fatalf("tax factor=%s", layout.TaxProductCell)
	}

	taxRow := layout.Values[7]
	if taxRow[0] != "GST" || taxRow[1] != 1.1 {
		t.Fatalf("tax row=%v", taxRow)
	}
	if layout.TaxStartRow != 8 {
		t.Fatalf("tax start=%d", layout.TaxStartRow)
	}
	bill := layout.Values[8]
	if bill[0] != "Bill total" || bill[1] != 41.8 {
		t.Fatalf("bill total=%v", bill)
	}
	if layout.LastDataRow != 9 {
		t.Fatalf("last data row=%d", layout.LastDataRow)
	}

	if layout.CheckboxRange.StartRow != 1 || layout.CheckboxRange.EndRow != 4 {
		t.Fatalf("checkbox rows=%+v", layout.CheckboxRange)
	}
	if layout.CheckboxRange.StartColumn != 4 || layout.CheckboxRange.EndColumn != 6 {
		t.Fatalf("checkbox cols=%+v", layout.CheckboxRange)
	}
}

func TestBuildSheetLayoutMultipliesAllTaxes(t *testing.T) {
	t.Parallel()
	items := []UnitItem{
		{ID: "1", Name: "Pizza", Quantity: 1, UnitPrice: 10, PollOption: "1. Pizza x1 $10"},
	}
	tax := []Tax{
		{Name: "Service", Multiplier: 1.2},
		{Name: "GST", Multiplier: 1.09},
	}
	people := []Participant{{Key: "alice", Name: "Alice"}}
	layout := BuildSheetLayout("Dinner", items, tax, 0, 13.08, map[string][]string{"1": {"alice"}}, people)

	if layout.TaxProductCell != "$B$7*$B$8" {
		t.Fatalf("tax factor=%s", layout.TaxProductCell)
	}
	withTax := layout.Values[4]
	if withTax[1] != "=B4*$B$7*$B$8+B3" {
		t.Fatalf("total with tax=%v", withTax[1])
	}
	if withTax[4] != "=E4*$B$7*$B$8+IF(E3=TRUE,$D$3,0)" {
		t.Fatalf("alice tax total=%v", withTax[4])
	}
	if layout.Values[6][0] != "Service" || layout.Values[6][1] != 1.2 {
		t.Fatalf("service row=%v", layout.Values[6])
	}
	if layout.Values[7][0] != "GST" || layout.Values[7][1] != 1.09 {
		t.Fatalf("gst row=%v", layout.Values[7])
	}
	if layout.TaxStartRow != 7 || layout.LastDataRow != 9 {
		t.Fatalf("tax start=%d last=%d", layout.TaxStartRow, layout.LastDataRow)
	}
}

func fmtString(v any) string {
	s, _ := v.(string)
	return s
}
