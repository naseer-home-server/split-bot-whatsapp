package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/naseer2426/split-bot-whatsapp/internal/gsheets"
	"github.com/naseer2426/split-bot-whatsapp/internal/splitbot"
	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

// ExportBillToGoogleSheet writes the totals snapshot to a Google Sheet with live formulas.
func (h *Handler) ExportBillToGoogleSheet(ctx context.Context, totalsID int, extraPeople []string) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	row, snap, err := h.GetBillAssignments(ctx, totalsID)
	if err != nil {
		return "", "", err
	}

	nameOf := displayNameFunc()
	people := totals.CollectParticipants(snap.Assignments, extraPeople, nameOf)
	title := fmt.Sprintf("Bill split #%d", row.ID)
	layout := totals.BuildSheetLayout(title, snap.Items, snap.Tax, snap.Discount, snap.TotalInBill, snap.Assignments, people)

	existing := ""
	if row.SheetID != nil {
		existing = strings.TrimSpace(*row.SheetID)
	}

	client, err := gsheets.NewClient(ctx)
	if err != nil {
		return "", "", err
	}
	result, err := client.Export(ctx, layout, existing)
	if err != nil {
		return "", "", err
	}

	now := time.Now().UTC()
	id := result.SpreadsheetID
	row.SheetID = &id
	row.LastExportedAt = &now
	if err := h.db.WithContext(ctx).Save(row).Error; err != nil {
		return "", "", fmt.Errorf("save sheet_id: %w", err)
	}
	return result.SpreadsheetID, result.URL, nil
}

func displayNameFunc() func(string) string {
	users, err := splitbot.GetAllUsers(splitbot.GetAllUsersOptions{})
	if err != nil {
		return func(id string) string { return id }
	}
	byLID := make(map[string]string)
	byNumber := make(map[string]string)
	for _, u := range users {
		if u.Name == "" {
			continue
		}
		if u.WhatsappLID != nil {
			if lid := normalizeUserKey(*u.WhatsappLID); lid != "" {
				byLID[lid] = u.Name
			}
		}
		if u.WhatsappNumber != nil {
			if num := normalizeUserKey(*u.WhatsappNumber); num != "" {
				byNumber[num] = u.Name
			}
		}
	}
	return func(id string) string {
		key := normalizeUserKey(id)
		if name, ok := byLID[key]; ok {
			return name
		}
		if name, ok := byNumber[key]; ok {
			return name
		}
		return id
	}
}

func normalizeUserKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	return s
}
