package whatsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/naseer2426/split-bot-whatsapp/internal/db"
	"github.com/naseer2426/split-bot-whatsapp/internal/gsheets"
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

	people := totals.CollectParticipants(snap.Assignments, extraPeople, h.displayNameOf(ctx, snap.Assignments, extraPeople))
	title := ""
	if row.Title != nil {
		title = *row.Title
	}
	layout := totals.BuildSheetLayout(totals.SheetTabTitle(row.ID, title), snap.Items, snap.Tax, snap.Discount, snap.TotalInBill, snap.Assignments, people)
	layout.TotalsID = row.ID

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
	gid := strconv.FormatInt(result.SheetGID, 10)
	row.SheetID = &gid
	row.LastExportedAt = &now
	if err := h.db.WithContext(ctx).Save(row).Error; err != nil {
		return "", "", fmt.Errorf("save sheet_id: %w", err)
	}
	return gid, result.URL, nil
}

func (h *Handler) displayNameOf(ctx context.Context, assignments map[string][]string, extra []string) func(string) string {
	lids, names := collectUserLookupKeys(assignments, extra)
	users, err := db.FindSplitBotUsers(h.db.WithContext(ctx), lids, names)
	if err != nil {
		fmt.Printf("load split_bot_users: %v\n", err)
		return func(id string) string { return id }
	}
	lookup := make(map[string]string, len(users)*3)
	for _, user := range users {
		name := strings.TrimSpace(user.Name)
		if name == "" {
			continue
		}
		lookup[strings.ToLower(name)] = name
		if user.WhatsappLID != nil {
			if key := normalizeUserKey(*user.WhatsappLID); key != "" {
				lookup[key] = name
			}
		}
		if user.WhatsappNumber != nil {
			if key := normalizeUserKey(*user.WhatsappNumber); key != "" {
				lookup[key] = name
			}
		}
	}
	return nameLookup(lookup)
}

func collectUserLookupKeys(assignments map[string][]string, extra []string) (lids, names []string) {
	lidSet := make(map[string]struct{})
	nameSet := make(map[string]struct{})
	addLID := func(raw string) {
		for _, variant := range lidLookupVariants(raw) {
			lidSet[variant] = struct{}{}
		}
	}
	for _, users := range assignments {
		for _, id := range users {
			addLID(id)
		}
	}
	for _, raw := range extra {
		key, isLID, ok := totals.ParseExtraPerson(raw)
		if !ok {
			continue
		}
		if isLID {
			addLID(key)
			continue
		}
		name := strings.ToLower(strings.TrimSpace(key))
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}
	lids = make([]string, 0, len(lidSet))
	for lid := range lidSet {
		lids = append(lids, lid)
	}
	names = make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	return lids, names
}

func lidLookupVariants(raw string) []string {
	raw = strings.TrimSpace(raw)
	key := normalizeUserKey(raw)
	out := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(raw)
	add(key)
	if key != "" {
		add(key + "@lid")
	}
	return out
}

func nameLookup(names map[string]string) func(string) string {
	norm := make(map[string]string, len(names))
	for k, v := range names {
		nk := normalizeUserKey(k)
		v = strings.TrimSpace(v)
		if nk == "" || v == "" {
			continue
		}
		norm[nk] = v
	}
	return func(id string) string {
		key := normalizeUserKey(id)
		if name, ok := norm[key]; ok {
			return name
		}
		if name, ok := names[strings.ToLower(strings.TrimSpace(id))]; ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
		if name, ok := names[id]; ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
		return id
	}
}

func normalizeUserKey(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	s = strings.TrimPrefix(s, "@")
	if n := strings.Index(s, ":"); n != -1 {
		s = s[:n]
	}
	if len(s) > 4 && strings.EqualFold(s[len(s)-4:], "@lid") {
		s = s[:len(s)-4]
	}
	return strings.TrimSpace(s)
}
