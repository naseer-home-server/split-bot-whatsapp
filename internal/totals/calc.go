package totals

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Prepare normalizes line items, applies sequential tax then discount, and rejects a billed total
// that differs from the calculated total by Epsilon or more.
func Prepare(items []LineItem, tax []Tax, discount, totalInBill float64) (*Prepared, error) {
	if discount < 0 {
		return nil, fmt.Errorf("discount cannot be negative")
	}
	if totalInBill < 0 {
		return nil, fmt.Errorf("total_in_bill cannot be negative")
	}
	if err := validateTax(tax); err != nil {
		return nil, err
	}
	units, err := NormalizeItems(items)
	if err != nil {
		return nil, err
	}
	itemTotal := roundMoney(SumLineTotals(units))
	calculated := roundMoney(ApplyTaxAndDiscount(itemTotal, tax, discount))
	diff := roundMoney(calculated - totalInBill)
	if math.Abs(calculated-totalInBill) >= Epsilon {
		return nil, &MismatchError{
			ItemTotal:       itemTotal,
			CalculatedTotal: calculated,
			TotalInBill:     totalInBill,
			Difference:      diff,
		}
	}
	return &Prepared{
		Units:           units,
		Tax:             tax,
		Discount:        discount,
		ItemTotal:       itemTotal,
		CalculatedTotal: calculated,
		TotalInBill:     totalInBill,
		Difference:      diff,
	}, nil
}

// NormalizeItems keeps one poll option per bill line and puts quantity in the label.
func NormalizeItems(items []LineItem) ([]UnitItem, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}
	units := make([]UnitItem, 0, len(items))
	for i, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			return nil, fmt.Errorf("item name is empty")
		}
		qty, err := parseQuantity(it.Quantity)
		if err != nil {
			return nil, fmt.Errorf("item %q: %w", name, err)
		}
		if it.UnitPrice < 0 {
			return nil, fmt.Errorf("item %q: unit_price cannot be negative", name)
		}
		optionNum := i + 1
		units = append(units, UnitItem{
			ID:         strconv.Itoa(optionNum),
			Name:       name,
			Quantity:   float64(qty),
			UnitPrice:  it.UnitPrice,
			PollOption: fmt.Sprintf("%d. %s x%s $%s", optionNum, name, formatPrice(float64(qty)), formatPrice(it.UnitPrice)),
		})
	}
	return units, nil
}

func parseQuantity(q float64) (int, error) {
	if q < 1 || q != math.Trunc(q) {
		return 0, fmt.Errorf("quantity must be a positive integer")
	}
	return int(q), nil
}

func formatPrice(p float64) string {
	if math.Abs(p-math.Round(p)) < 1e-9 {
		return strconv.FormatInt(int64(math.Round(p)), 10)
	}
	return strconv.FormatFloat(p, 'f', 2, 64)
}

func SumLineTotals(units []UnitItem) float64 {
	var sum float64
	for _, u := range units {
		sum += u.LineTotal()
	}
	return sum
}

func validateTax(tax []Tax) error {
	for i, t := range tax {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			return fmt.Errorf("tax[%d] name is empty", i)
		}
		if t.Multiplier <= 0 {
			return fmt.Errorf("tax %q multiplier must be positive", name)
		}
	}
	return nil
}

// TaxProduct is Π(multiplier); empty tax is 1.
func TaxProduct(tax []Tax) float64 {
	p := 1.0
	for _, t := range tax {
		p *= t.Multiplier
	}
	return p
}

// ApplyTaxAndDiscount returns itemTotal * Π(tax) - discount.
func ApplyTaxAndDiscount(itemTotal float64, tax []Tax, discount float64) float64 {
	return itemTotal*TaxProduct(tax) - discount
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// PollOptions returns poll labels in stored order.
func PollOptions(units []UnitItem) []string {
	out := make([]string, len(units))
	for i, u := range units {
		out[i] = u.PollOption
	}
	return out
}

func itemIDs(units []UnitItem) map[string]UnitItem {
	m := make(map[string]UnitItem, len(units))
	for _, u := range units {
		m[u.ID] = u
	}
	return m
}
