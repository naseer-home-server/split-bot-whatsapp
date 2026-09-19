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
	people := CollectParticipants(assignments, []string{"Cara", "Alice", " lid-b "}, nameOf)
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
	if withTax[4] != "=E5*$B$8+IF(E4=TRUE,$D$4,0)" {
		t.Fatalf("alice tax total=%v (tax cell %s)", withTax[4], layout.TaxProductCell)
	}

	taxRow := layout.Values[7]
	if taxRow[1] != 1.1 {
		t.Fatalf("tax product=%v", taxRow[1])
	}

	if layout.CheckboxRange.StartRow != 1 || layout.CheckboxRange.EndRow != 4 {
		t.Fatalf("checkbox rows=%+v", layout.CheckboxRange)
	}
	if layout.CheckboxRange.StartColumn != 4 || layout.CheckboxRange.EndColumn != 6 {
		t.Fatalf("checkbox cols=%+v", layout.CheckboxRange)
	}
}

func fmtString(v any) string {
	s, _ := v.(string)
	return s
}
