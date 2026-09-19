package totals

import "testing"

func TestHasSheetExport(t *testing.T) {
	t.Parallel()
	if HasSheetExport(nil) {
		t.Fatal("nil should be false")
	}
	empty := "  "
	if HasSheetExport(&empty) {
		t.Fatal("blank should be false")
	}
	gid := "123"
	if !HasSheetExport(&gid) {
		t.Fatal("gid should be true")
	}
}

func TestParseSheetAssignments(t *testing.T) {
	t.Parallel()
	meta := SheetDimensionMeta{
		ItemIDs:    map[int]string{1: "1", 2: "2"},
		PersonKeys: map[int]string{4: "alice", 5: "bob"},
	}
	values := [][]any{
		{"Item", "Price", "Divided by", "Per person", "Alice", "Bob"},
		{"Pizza", 10, 1, 10, true, false},
		{"Salad", 8, 2, 4, true, true},
		{"Discount", 0, 2, 0, true, true},
	}
	got := ParseSheetAssignments(meta, values)
	if len(got["1"]) != 1 || got["1"][0] != "alice" {
		t.Fatalf("item 1=%v", got["1"])
	}
	if len(got["2"]) != 2 || got["2"][0] != "alice" || got["2"][1] != "bob" {
		t.Fatalf("item 2=%v", got["2"])
	}
	if _, ok := got["3"]; ok {
		t.Fatalf("unexpected discount item: %v", got)
	}
}

func TestAssignmentsFromSheetDropsUnknownItems(t *testing.T) {
	t.Parallel()
	units := []UnitItem{{ID: "1", Name: "Pizza"}}
	parsed := map[string][]string{"1": {"alice"}, "99": {"bob"}}
	got := AssignmentsFromSheet(units, parsed)
	if len(got) != 1 || len(got["1"]) != 1 || got["1"][0] != "alice" {
		t.Fatalf("got=%v", got)
	}
}

func TestCheckboxTrue(t *testing.T) {
	t.Parallel()
	if !checkboxTrue(true) || checkboxTrue(false) {
		t.Fatal("bool")
	}
	if !checkboxTrue("TRUE") || checkboxTrue("no") {
		t.Fatal("string")
	}
	if !checkboxTrue(1.0) || checkboxTrue(0) {
		t.Fatal("number")
	}
}
