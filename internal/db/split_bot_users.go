package db

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// SplitBotUser maps the existing split_bot_users table so this service can look up display names.
//
// Ownership of this table is still unclear: split-bot created it, both services currently share
// DATABASE_URL, and we may split that later. Do not AutoMigrate or add a WhatsApp migration for it.
type SplitBotUser struct {
	ID               int        `gorm:"column:id;primaryKey"`
	Name             string     `gorm:"column:name"`
	Email            string     `gorm:"column:email"`
	TelegramUsername *string    `gorm:"column:telegram_username"`
	WhatsappNumber   *string    `gorm:"column:whatsapp_number"`
	WhatsappLID      *string    `gorm:"column:whatsapp_lid"`
	CreatedAt        *time.Time `gorm:"column:created_at;type:timestamp"`
	UpdatedAt        *time.Time `gorm:"column:updated_at;type:timestamp"`
}

func (SplitBotUser) TableName() string {
	return "split_bot_users"
}

// FindSplitBotUsers loads users whose WhatsApp LID/number or name matches the keys we already have.
// lids and names should be the identifiers from this bill (assignments + extra people), not a full dump.
func FindSplitBotUsers(gdb *gorm.DB, lids, names []string) ([]SplitBotUser, error) {
	lids = compactStrings(lids)
	names = compactStrings(names)
	if len(lids) == 0 && len(names) == 0 {
		return nil, nil
	}

	q := gdb.Model(&SplitBotUser{})
	switch {
	case len(lids) > 0 && len(names) > 0:
		q = q.Where("whatsapp_lid IN ? OR whatsapp_number IN ? OR LOWER(name) IN ?", lids, lids, names)
	case len(lids) > 0:
		q = q.Where("whatsapp_lid IN ? OR whatsapp_number IN ?", lids, lids)
	default:
		q = q.Where("LOWER(name) IN ?", names)
	}

	var users []SplitBotUser
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func compactStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
