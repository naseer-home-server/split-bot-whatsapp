package db

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// WhatsappBotChatMeta matches migrations through 000004_whatsapp_bot_chat_meta_mode.*.
type WhatsappBotChatMeta struct {
	ID           int       `gorm:"column:id;primaryKey;autoIncrement"`
	GroupID      string    `gorm:"column:group_id;type:varchar;not null;uniqueIndex"`
	PlatformType string    `gorm:"column:platform_type;type:varchar;not null"`
	Mode         string    `gorm:"column:mode;type:varchar;not null;default:silent"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz"`
}

func (WhatsappBotChatMeta) TableName() string {
	return "whatsapp_bot_chat_meta"
}

// Poll matches migrations/000005_polls_and_votes.* — collective poll spanning multiple WA poll messages.
type Poll struct {
	ID          int             `gorm:"column:id;primaryKey;autoIncrement"`
	MessageKeys pq.StringArray  `gorm:"column:message_keys;type:text[]"`
	OptionsMeta json.RawMessage `gorm:"column:options_meta;type:jsonb"`
}

func (Poll) TableName() string {
	return "polls"
}

// Vote matches migrations through 000006_polls_votes_jsonb.* — one row per (poll, user).
// Votes is JSON (from 000006 onward): { "<poll_message_stanza_id>": ["<hex>", ...], ... } per WhatsApp poll shard.
type Vote struct {
	ID     int             `gorm:"column:id;primaryKey;autoIncrement"`
	PollID int             `gorm:"column:poll_id;not null;uniqueIndex:idx_votes_poll_id_user_id"`
	UserID string          `gorm:"column:user_id;type:varchar;not null;uniqueIndex:idx_votes_poll_id_user_id"`
	Votes  json.RawMessage `gorm:"column:votes;type:jsonb"`
}

func (Vote) TableName() string {
	return "votes"
}

// SplitbotTotals matches migrations/000007_splitbot_totals.* — one bill with poll-backed item assignments.
type SplitbotTotals struct {
	ID              int             `gorm:"column:id;primaryKey;autoIncrement"`
	GroupID         string          `gorm:"column:group_id;type:varchar;not null"`
	PollID          *int            `gorm:"column:poll_id"`
	Items           json.RawMessage `gorm:"column:items;type:jsonb"`
	Tax             json.RawMessage `gorm:"column:tax;type:jsonb"`
	Discount        float64         `gorm:"column:discount;type:numeric"`
	TotalInBill     float64         `gorm:"column:total_in_bill;type:numeric"`
	CalculatedTotal float64         `gorm:"column:calculated_total;type:numeric"`
	Assignments     json.RawMessage `gorm:"column:assignments;type:jsonb"`
	CreatedAt       time.Time       `gorm:"column:created_at;type:timestamptz"`
	UpdatedAt       time.Time       `gorm:"column:updated_at;type:timestamptz"`
}

func (SplitbotTotals) TableName() string {
	return "splitbot_totals"
}
