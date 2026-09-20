package whatsapp

import (
	"sort"
	"testing"
)

func TestNormalizeUserKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		" 123 ":      "123",
		"@123":       "123",
		"123@lid":    "123",
		"@123@lid":   "123",
		"+6012":      "6012",
		"123:12@lid": "123",
		"Naseer":     "Naseer",
	}
	for in, want := range cases {
		if got := normalizeUserKey(in); got != want {
			t.Errorf("normalizeUserKey(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNameLookup(t *testing.T) {
	t.Parallel()
	fn := nameLookup(map[string]string{
		"@123@lid":           "Naseer Ahmed Khan",
		"6012":               "Gulshan",
		"naseer ahmed khan": "Naseer Ahmed Khan",
	})
	if got := fn("123"); got != "Naseer Ahmed Khan" {
		t.Fatalf("lid=%q", got)
	}
	if got := fn("Naseer Ahmed Khan"); got != "Naseer Ahmed Khan" {
		t.Fatalf("name=%q", got)
	}
	if got := fn("unknown"); got != "unknown" {
		t.Fatalf("unknown=%q", got)
	}
}

func TestCollectUserLookupKeys(t *testing.T) {
	t.Parallel()
	lids, names := collectUserLookupKeys(map[string][]string{
		"1": {"123@lid", "123"},
	}, []string{"@60123456789", "Sam", "  "})
	sort.Strings(lids)
	sort.Strings(names)
	wantLids := []string{"123", "123@lid", "60123456789", "60123456789@lid"}
	if len(lids) != len(wantLids) {
		t.Fatalf("lids=%v want %v", lids, wantLids)
	}
	for i, want := range wantLids {
		if lids[i] != want {
			t.Fatalf("lids=%v want %v", lids, wantLids)
		}
	}
	if len(names) != 1 || names[0] != "sam" {
		t.Fatalf("names=%v", names)
	}
}
