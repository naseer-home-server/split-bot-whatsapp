package totals

import (
	"encoding/json"
	"fmt"
)

// Epsilon is the max |calculated_total - total_in_bill| allowed to accept a bill.
const Epsilon = 0.05

const defaultPollTitle = "Who had what?"

// LineItem is one bill line as submitted by the bot (one poll option per line).
type LineItem struct {
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

// Tax is one tax or surcharge applied sequentially via multiplier (10% → 1.1).
type Tax struct {
	Name       string  `json:"name"`
	Multiplier float64 `json:"multiplier"`
}

// UnitItem is one poll-voteable bill line. Voters for this option share quantity * unit_price.
type UnitItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	PollOption string  `json:"poll_option"`
}

// LineTotal is quantity * unit_price. Missing quantity is treated as 1.
func (u UnitItem) LineTotal() float64 {
	qty := u.Quantity
	if qty == 0 {
		qty = 1
	}
	return qty * u.UnitPrice
}

// ShareItem is one line (or fraction of a shared line) on a person's breakdown.
type ShareItem struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Share float64 `json:"share"`
}

// PersonShare is one participant's food subtotal and amount owed.
type PersonShare struct {
	UserID       string      `json:"user_id"`
	FoodSubtotal float64     `json:"food_subtotal"`
	Owed         float64     `json:"owed"`
	Items        []ShareItem `json:"items"`
}

// Prepared is the validated bill used to persist a totals row.
type Prepared struct {
	Units           []UnitItem
	Tax             []Tax
	Discount        float64
	ItemTotal       float64
	CalculatedTotal float64
	TotalInBill     float64
	Difference      float64
}

// Snapshot is the GET/PUT response payload: stored bill plus live shares.
type Snapshot struct {
	Items           []UnitItem          `json:"items"`
	Tax             []Tax               `json:"tax"`
	Discount        float64             `json:"discount"`
	ItemTotal       float64             `json:"item_total"`
	TotalInBill     float64             `json:"total_in_bill"`
	CalculatedTotal float64             `json:"calculated_total"`
	Assignments     map[string][]string `json:"assignments"`
	Unassigned      []UnitItem          `json:"unassigned"`
	Shares          []PersonShare       `json:"shares"`
	UnassignedOwed  float64             `json:"unassigned_owed"`
}

// MismatchError means calculated_total does not match total_in_bill within Epsilon.
type MismatchError struct {
	ItemTotal       float64
	CalculatedTotal float64
	TotalInBill     float64
	Difference      float64
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf("totals mismatch: calculated %.2f vs bill %.2f (difference %.2f)",
		e.CalculatedTotal, e.TotalInBill, e.Difference)
}

func DefaultPollTitle() string {
	return defaultPollTitle
}

func MarshalItems(units []UnitItem) (json.RawMessage, error) {
	return marshalJSON(units)
}

func UnmarshalItems(raw json.RawMessage) ([]UnitItem, error) {
	if len(raw) == 0 {
		return []UnitItem{}, nil
	}
	var units []UnitItem
	if err := json.Unmarshal(raw, &units); err != nil {
		return nil, fmt.Errorf("decode items: %w", err)
	}
	if units == nil {
		return []UnitItem{}, nil
	}
	return units, nil
}

func MarshalTax(tax []Tax) (json.RawMessage, error) {
	if tax == nil {
		tax = []Tax{}
	}
	return marshalJSON(tax)
}

func UnmarshalTax(raw json.RawMessage) ([]Tax, error) {
	if len(raw) == 0 {
		return []Tax{}, nil
	}
	var tax []Tax
	if err := json.Unmarshal(raw, &tax); err != nil {
		return nil, fmt.Errorf("decode tax: %w", err)
	}
	if tax == nil {
		return []Tax{}, nil
	}
	return tax, nil
}

func MarshalAssignments(assignments map[string][]string) (json.RawMessage, error) {
	if assignments == nil {
		assignments = map[string][]string{}
	}
	return marshalJSON(assignments)
}

func UnmarshalAssignments(raw json.RawMessage) (map[string][]string, error) {
	if len(raw) == 0 {
		return map[string][]string{}, nil
	}
	var m map[string][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode assignments: %w", err)
	}
	if m == nil {
		return map[string][]string{}, nil
	}
	return m, nil
}

func marshalJSON(v any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
