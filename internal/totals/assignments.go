package totals

import (
	"fmt"
	"sort"
	"strings"
)

// MergeSetAssignments replaces existing assignment keys present in patch. An empty user list unassigns that item.
func MergeSetAssignments(existing map[string][]string, patch map[string][]string, units []UnitItem) (map[string][]string, error) {
	valid := itemIDs(units)
	out := cloneAssignments(existing)
	if patch == nil {
		return out, nil
	}
	for id, users := range patch {
		if _, ok := valid[id]; !ok {
			return nil, fmt.Errorf("unknown item id %q", id)
		}
		cleaned, err := cleanUserIDs(users)
		if err != nil {
			return nil, fmt.Errorf("item %s: %w", id, err)
		}
		if len(cleaned) == 0 {
			delete(out, id)
			continue
		}
		out[id] = cleaned
	}
	return out, nil
}

// MergePollVotes overwrites assignments for poll options that currently have voters.
// Options with zero voters keep any existing set-assignments.
func MergePollVotes(existing map[string][]string, units []UnitItem, votersByOption map[string][]string) map[string][]string {
	out := cloneAssignments(existing)
	if votersByOption == nil {
		votersByOption = map[string][]string{}
	}
	for _, u := range units {
		voters := votersByOption[u.PollOption]
		if len(voters) == 0 {
			continue
		}
		cleaned, err := cleanUserIDs(voters)
		if err != nil || len(cleaned) == 0 {
			continue
		}
		out[u.ID] = cleaned
	}
	return out
}

func cleanUserIDs(users []string) ([]string, error) {
	seen := make(map[string]struct{}, len(users))
	out := make([]string, 0, len(users))
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u == "" {
			return nil, fmt.Errorf("user id is empty")
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	sort.Strings(out)
	return out, nil
}

func cloneAssignments(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// ComputeShares builds per-person owed amounts: person_food * Π(tax) - discount/n.
// n is unique assigned users. Voters on a line share quantity * unit_price. Unassigned lines do not receive discount.
func ComputeShares(units []UnitItem, tax []Tax, discount, calculatedTotal float64, assignments map[string][]string) (shares []PersonShare, unassigned []UnitItem, unassignedOwed float64) {
	if assignments == nil {
		assignments = map[string][]string{}
	}
	taxProd := TaxProduct(tax)

	type running struct {
		food  float64
		items []ShareItem
	}
	people := make(map[string]*running)
	unassigned = make([]UnitItem, 0)
	var unassignedFood float64

	for _, u := range units {
		lineTotal := u.LineTotal()
		users := assignments[u.ID]
		if len(users) == 0 {
			unassigned = append(unassigned, u)
			unassignedFood += lineTotal
			continue
		}
		share := lineTotal / float64(len(users))
		for _, userID := range users {
			r := people[userID]
			if r == nil {
				r = &running{}
				people[userID] = r
			}
			r.food += share
			r.items = append(r.items, ShareItem{ID: u.ID, Name: u.Name, Share: roundMoney(share)})
		}
	}

	n := len(people)
	unassignedOwed = roundMoney(unassignedFood * taxProd)
	if n == 0 {
		return []PersonShare{}, unassigned, unassignedOwed
	}

	userIDs := make([]string, 0, n)
	for id := range people {
		userIDs = append(userIDs, id)
	}
	sort.Strings(userIDs)

	shares = make([]PersonShare, 0, n)
	var assignedOwed float64
	perPersonDiscount := discount / float64(n)
	for _, id := range userIDs {
		r := people[id]
		owed := roundMoney(r.food*taxProd - perPersonDiscount)
		assignedOwed += owed
		shares = append(shares, PersonShare{
			UserID:       id,
			FoodSubtotal: roundMoney(r.food),
			Owed:         owed,
			Items:        r.items,
		})
	}

	target := roundMoney(calculatedTotal)
	drift := roundMoney(target - (assignedOwed + unassignedOwed))
	if drift != 0 && len(shares) > 0 {
		last := &shares[len(shares)-1]
		last.Owed = roundMoney(last.Owed + drift)
	}

	return shares, unassigned, unassignedOwed
}

// BuildSnapshot computes shares from stored units, tax, discount, and assignments.
func BuildSnapshot(units []UnitItem, tax []Tax, discount, totalInBill, calculatedTotal float64, assignments map[string][]string) Snapshot {
	if assignments == nil {
		assignments = map[string][]string{}
	}
	shares, unassigned, unassignedOwed := ComputeShares(units, tax, discount, calculatedTotal, assignments)
	return Snapshot{
		Items:           units,
		Tax:             tax,
		Discount:        discount,
		ItemTotal:       roundMoney(SumLineTotals(units)),
		TotalInBill:     totalInBill,
		CalculatedTotal: calculatedTotal,
		Assignments:     assignments,
		Unassigned:      unassigned,
		Shares:          shares,
		UnassignedOwed:  unassignedOwed,
	}
}
