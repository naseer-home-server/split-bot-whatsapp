package whatsapp

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/naseer2426/split-bot-whatsapp/internal/db"
	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

// CreateBillTotals validates the bill, persists a splitbot_totals row, and sends a WhatsApp poll of line items.
func (h *Handler) CreateBillTotals(ctx context.Context, groupID, title string, items []totals.LineItem, tax []totals.Tax, discount, totalInBill float64) (*db.SplitbotTotals, *totals.Prepared, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	prepared, err := totals.Prepare(items, tax, discount, totalInBill)
	if err != nil {
		return nil, nil, err
	}

	itemsJSON, err := totals.MarshalItems(prepared.Units)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal items: %w", err)
	}
	taxJSON, err := totals.MarshalTax(prepared.Tax)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal tax: %w", err)
	}
	assignJSON, err := totals.MarshalAssignments(map[string][]string{})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal assignments: %w", err)
	}

	row := db.SplitbotTotals{
		GroupID:         groupID,
		Items:           itemsJSON,
		Tax:             taxJSON,
		Discount:        prepared.Discount,
		TotalInBill:     prepared.TotalInBill,
		CalculatedTotal: prepared.CalculatedTotal,
		Assignments:     assignJSON,
	}
	if err := h.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, nil, fmt.Errorf("create totals row: %w", err)
	}

	if title == "" {
		title = totals.DefaultPollTitle()
	}
	poll, err := h.SendPoll(ctx, title, totals.PollOptions(prepared.Units), groupID)
	if err != nil {
		_ = h.db.WithContext(ctx).Delete(&db.SplitbotTotals{}, row.ID)
		return nil, nil, fmt.Errorf("create poll: %w", err)
	}
	row.PollID = &poll.ID
	if err := h.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, nil, fmt.Errorf("save totals poll_id: %w", err)
	}
	return &row, prepared, nil
}

// GetBillAssignments loads a totals row and returns the computed assignment snapshot.
func (h *Handler) GetBillAssignments(ctx context.Context, totalsID int) (*db.SplitbotTotals, totals.Snapshot, error) {
	row, err := h.loadTotals(ctx, totalsID)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	snap, err := snapshotFromRow(row)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	return row, snap, nil
}

// SetBillAssignments merges a partial assignments map onto the totals row and returns the new snapshot.
func (h *Handler) SetBillAssignments(ctx context.Context, totalsID int, patch map[string][]string) (*db.SplitbotTotals, totals.Snapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	row, err := h.loadTotals(ctx, totalsID)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	units, err := totals.UnmarshalItems(row.Items)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	existing, err := totals.UnmarshalAssignments(row.Assignments)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	merged, err := totals.MergeSetAssignments(existing, patch, units)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	raw, err := totals.MarshalAssignments(merged)
	if err != nil {
		return nil, totals.Snapshot{}, fmt.Errorf("marshal assignments: %w", err)
	}
	row.Assignments = raw
	if err := h.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, totals.Snapshot{}, fmt.Errorf("save assignments: %w", err)
	}
	snap, err := snapshotFromRow(row)
	if err != nil {
		return nil, totals.Snapshot{}, err
	}
	return row, snap, nil
}

func (h *Handler) loadTotals(ctx context.Context, totalsID int) (*db.SplitbotTotals, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if totalsID <= 0 {
		return nil, fmt.Errorf("invalid totals id")
	}
	var row db.SplitbotTotals
	if err := h.db.WithContext(ctx).First(&row, totalsID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func snapshotFromRow(row *db.SplitbotTotals) (totals.Snapshot, error) {
	units, err := totals.UnmarshalItems(row.Items)
	if err != nil {
		return totals.Snapshot{}, err
	}
	tax, err := totals.UnmarshalTax(row.Tax)
	if err != nil {
		return totals.Snapshot{}, err
	}
	assignments, err := totals.UnmarshalAssignments(row.Assignments)
	if err != nil {
		return totals.Snapshot{}, err
	}
	return totals.BuildSnapshot(units, tax, row.Discount, row.TotalInBill, row.CalculatedTotal, assignments), nil
}

func (h *Handler) syncTotalsAssignmentsFromPoll(tx *gorm.DB, pollID int) error {
	var row db.SplitbotTotals
	err := tx.Where("poll_id = ?", pollID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load totals for poll %d: %w", pollID, err)
	}

	status, err := getPollStatus(tx, pollID)
	if err != nil {
		return err
	}
	votersByOption := make(map[string][]string, len(status))
	for _, opt := range status {
		votersByOption[opt.Option] = opt.Users
	}

	units, err := totals.UnmarshalItems(row.Items)
	if err != nil {
		return err
	}
	existing, err := totals.UnmarshalAssignments(row.Assignments)
	if err != nil {
		return err
	}
	merged := totals.MergePollVotes(existing, units, votersByOption)
	raw, err := totals.MarshalAssignments(merged)
	if err != nil {
		return fmt.Errorf("marshal assignments: %w", err)
	}
	row.Assignments = raw
	if err := tx.Save(&row).Error; err != nil {
		return fmt.Errorf("save totals assignments: %w", err)
	}
	return nil
}
