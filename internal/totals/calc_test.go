package totals

import (
	"errors"
	"math"
	"testing"
)

func TestNormalizeItemsKeepsQuantityOnOneOption(t *testing.T) {
	t.Parallel()

	units, err := NormalizeItems([]LineItem{
		{Name: "Pizza", Quantity: 2, UnitPrice: 18},
		{Name: "Salad", Quantity: 1, UnitPrice: 12.5},
	})
	if err != nil {
		t.Fatalf("NormalizeItems: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("len(units) = %d, want 2", len(units))
	}
	if units[0].ID != "1" || units[1].ID != "2" {
		t.Fatalf("ids = %q, %q", units[0].ID, units[1].ID)
	}
	if units[0].Quantity != 2 || units[1].Quantity != 1 {
		t.Fatalf("quantities = %v, %v", units[0].Quantity, units[1].Quantity)
	}
	if units[0].PollOption != "1. Pizza x2 $18" {
		t.Fatalf("pizza label = %q", units[0].PollOption)
	}
	if units[1].PollOption != "2. Salad x1 $12.50" {
		t.Fatalf("salad label = %q", units[1].PollOption)
	}
	if !almostEqual(SumLineTotals(units), 48.5) {
		t.Fatalf("line totals = %v, want 48.5", SumLineTotals(units))
	}
}

func TestPrepareSequentialTax(t *testing.T) {
	t.Parallel()

	items := []LineItem{{Name: "Food", Quantity: 1, UnitPrice: 100}}
	tax := []Tax{
		{Name: "Service", Multiplier: 1.10},
		{Name: "GST", Multiplier: 1.09},
	}
	// 100 * 1.10 * 1.09 - 5 = 119.9 - 5 = 114.9
	prepared, err := Prepare(items, tax, 5, 114.9)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !almostEqual(prepared.ItemTotal, 100) {
		t.Fatalf("item_total = %v, want 100", prepared.ItemTotal)
	}
	if !almostEqual(prepared.CalculatedTotal, 114.9) {
		t.Fatalf("calculated_total = %v, want 114.9", prepared.CalculatedTotal)
	}
}

func TestPrepareMismatch(t *testing.T) {
	t.Parallel()

	items := []LineItem{{Name: "Food", Quantity: 1, UnitPrice: 10}}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	_, err := Prepare(items, tax, 0, 12)
	var mismatch *MismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Prepare err = %v, want MismatchError", err)
	}
	if !almostEqual(mismatch.CalculatedTotal, 11) {
		t.Fatalf("calculated = %v, want 11", mismatch.CalculatedTotal)
	}
}

func TestPrepareEpsilon(t *testing.T) {
	t.Parallel()

	items := []LineItem{{Name: "Food", Quantity: 1, UnitPrice: 10}}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	if _, err := Prepare(items, tax, 0, 11.04); err != nil {
		t.Fatalf("11.04 should pass epsilon: %v", err)
	}
	if _, err := Prepare(items, tax, 0, 11.06); err == nil {
		t.Fatal("11.06 should fail epsilon")
	}
}

func TestComputeSharesEqualDiscountUnevenFood(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "Steak", UnitPrice: 80, PollOption: "1. Steak $80"},
		{ID: "2", Name: "Salad", UnitPrice: 20, PollOption: "2. Salad $20"},
	}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	assignments := map[string][]string{
		"1": {"alice"},
		"2": {"bob"},
	}
	// calculated = 100*1.1 - 10 = 100
	shares, unassigned, unassignedOwed := ComputeShares(units, tax, 10, 100, assignments)
	if len(unassigned) != 0 || unassignedOwed != 0 {
		t.Fatalf("unassigned = %+v owed %v", unassigned, unassignedOwed)
	}
	if len(shares) != 2 {
		t.Fatalf("len(shares) = %d", len(shares))
	}
	byUser := map[string]PersonShare{}
	for _, s := range shares {
		byUser[s.UserID] = s
	}
	// alice: 80*1.1 - 5 = 83; bob: 20*1.1 - 5 = 17
	if !almostEqual(byUser["alice"].Owed, 83) {
		t.Fatalf("alice owed = %v, want 83", byUser["alice"].Owed)
	}
	if !almostEqual(byUser["bob"].Owed, 17) {
		t.Fatalf("bob owed = %v, want 17", byUser["bob"].Owed)
	}
	if !almostEqual(byUser["alice"].Owed+byUser["bob"].Owed, 100) {
		t.Fatalf("sum owed = %v, want 100", byUser["alice"].Owed+byUser["bob"].Owed)
	}
}

func TestComputeSharesSharedQuantity(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "Pizza", Quantity: 2, UnitPrice: 18, PollOption: "1. Pizza x2 $18"},
	}
	assignments := map[string][]string{"1": {"alice", "bob"}}
	// line total 36; each food 18; no tax/discount; calculated 36
	shares, _, _ := ComputeShares(units, nil, 0, 36, assignments)
	if len(shares) != 2 {
		t.Fatalf("len(shares) = %d", len(shares))
	}
	for _, s := range shares {
		if !almostEqual(s.FoodSubtotal, 18) {
			t.Fatalf("%s food = %v, want 18", s.UserID, s.FoodSubtotal)
		}
		if !almostEqual(s.Owed, 18) {
			t.Fatalf("%s owed = %v, want 18", s.UserID, s.Owed)
		}
	}
}

func TestPrepareQuantityInItemTotal(t *testing.T) {
	t.Parallel()

	items := []LineItem{{Name: "Pizza", Quantity: 2, UnitPrice: 18}}
	prepared, err := Prepare(items, nil, 0, 36)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !almostEqual(prepared.ItemTotal, 36) {
		t.Fatalf("item_total = %v, want 36", prepared.ItemTotal)
	}
	if len(prepared.Units) != 1 {
		t.Fatalf("len(units) = %d, want 1", len(prepared.Units))
	}
}

func TestComputeSharesSharedItem(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "Pizza", UnitPrice: 18, PollOption: "1. Pizza $18"},
	}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	assignments := map[string][]string{"1": {"alice", "bob"}}
	shares, _, _ := ComputeShares(units, tax, 0, 19.8, assignments)
	if len(shares) != 2 {
		t.Fatalf("len(shares) = %d", len(shares))
	}
	for _, s := range shares {
		if !almostEqual(s.FoodSubtotal, 9) {
			t.Fatalf("%s food = %v, want 9", s.UserID, s.FoodSubtotal)
		}
		if !almostEqual(s.Owed, 9.9) {
			t.Fatalf("%s owed = %v, want 9.9", s.UserID, s.Owed)
		}
	}
}

func TestComputeSharesUnassignedRemainder(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "Pizza", UnitPrice: 18, PollOption: "1. Pizza $18"},
		{ID: "2", Name: "Pasta", UnitPrice: 12, PollOption: "2. Pasta $12"},
	}
	tax := []Tax{{Name: "GST", Multiplier: 1.1}}
	assignments := map[string][]string{"1": {"alice"}}
	// calculated = 30*1.1 - 5 = 28
	shares, unassigned, unassignedOwed := ComputeShares(units, tax, 5, 28, assignments)
	if len(shares) != 1 || shares[0].UserID != "alice" {
		t.Fatalf("shares = %+v", shares)
	}
	// alice: 18*1.1 - 5 = 14.8; unassigned: 12*1.1 = 13.2
	if !almostEqual(shares[0].Owed, 14.8) {
		t.Fatalf("alice owed = %v, want 14.8", shares[0].Owed)
	}
	if len(unassigned) != 1 || unassigned[0].ID != "2" {
		t.Fatalf("unassigned = %+v", unassigned)
	}
	if !almostEqual(unassignedOwed, 13.2) {
		t.Fatalf("unassigned_owed = %v, want 13.2", unassignedOwed)
	}
	if !almostEqual(shares[0].Owed+unassignedOwed, 28) {
		t.Fatalf("sum = %v, want 28", shares[0].Owed+unassignedOwed)
	}
}

func TestComputeSharesNoAssignments(t *testing.T) {
	t.Parallel()

	units := []UnitItem{{ID: "1", Name: "Pizza", UnitPrice: 10, PollOption: "1. Pizza $10"}}
	shares, unassigned, unassignedOwed := ComputeShares(units, nil, 0, 10, nil)
	if len(shares) != 0 {
		t.Fatalf("shares = %+v, want empty", shares)
	}
	if len(unassigned) != 1 {
		t.Fatalf("unassigned = %+v", unassigned)
	}
	if !almostEqual(unassignedOwed, 10) {
		t.Fatalf("unassigned_owed = %v, want 10", unassignedOwed)
	}
}

func TestMergeSetAssignments(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "A", UnitPrice: 1, PollOption: "1. A $1"},
		{ID: "2", Name: "B", UnitPrice: 1, PollOption: "2. B $1"},
	}
	existing := map[string][]string{"1": {"alice"}}
	out, err := MergeSetAssignments(existing, map[string][]string{
		"2": {"bob"},
		"1": {},
	}, units)
	if err != nil {
		t.Fatalf("MergeSetAssignments: %v", err)
	}
	if _, ok := out["1"]; ok {
		t.Fatalf("item 1 should be unassigned, got %v", out["1"])
	}
	if len(out["2"]) != 1 || out["2"][0] != "bob" {
		t.Fatalf("item 2 = %v, want [bob]", out["2"])
	}
}

func TestMergePollVotesKeepsSetWhenNoVoters(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "A", UnitPrice: 1, PollOption: "1. A $1"},
		{ID: "2", Name: "B", UnitPrice: 1, PollOption: "2. B $1"},
	}
	existing := map[string][]string{"2": {"chat-user"}}
	out := MergePollVotes(existing, units, map[string][]string{
		"1. A $1": {"voter"},
		"2. B $1": {},
	})
	if len(out["1"]) != 1 || out["1"][0] != "voter" {
		t.Fatalf("item 1 = %v, want [voter]", out["1"])
	}
	if len(out["2"]) != 1 || out["2"][0] != "chat-user" {
		t.Fatalf("item 2 = %v, want [chat-user]", out["2"])
	}
}

func TestComputeSharesRoundingPennyAdjust(t *testing.T) {
	t.Parallel()

	units := []UnitItem{
		{ID: "1", Name: "A", UnitPrice: 10, PollOption: "1. A $10"},
		{ID: "2", Name: "B", UnitPrice: 10, PollOption: "2. B $10"},
		{ID: "3", Name: "C", UnitPrice: 10, PollOption: "3. C $10"},
	}
	assignments := map[string][]string{
		"1": {"alice"},
		"2": {"bob"},
		"3": {"cara"},
	}
	shares, _, _ := ComputeShares(units, nil, 10, 20, assignments)
	var sum float64
	for _, s := range shares {
		sum += s.Owed
	}
	if !almostEqual(sum, 20) {
		t.Fatalf("sum owed = %v, want 20; shares=%+v", sum, shares)
	}
}

func TestPrepareRejectsBadQuantity(t *testing.T) {
	t.Parallel()

	if _, err := Prepare([]LineItem{{Name: "X", Quantity: 1.5, UnitPrice: 1}}, nil, 0, 1); err == nil {
		t.Fatal("expected error for fractional quantity")
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}
