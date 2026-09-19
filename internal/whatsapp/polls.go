package whatsapp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lib/pq"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"

	wa "go.mau.fi/whatsmeow"

	"github.com/naseer2426/split-bot-whatsapp/internal/db"
)

const maxPollOptionsPerWA = 12

// OptionSelection is one poll option label with user ids who selected it across all poll shards (from merged votes JSON).
type OptionSelection struct {
	Option string   `json:"option"`
	Users  []string `json:"users"`
}

// SendPoll sends one or more WhatsApp polls (split every maxPollOptionsPerWA options), persists a collective db.Poll
// row before sends (empty message_keys), then updates message_keys with stanza IDs from each SendMessage response.
// groupID is the group local id without @g.us (digits/agent id); a full ...@g.us JID is also accepted.
func (h *Handler) SendPoll(ctx context.Context, title string, options []string, groupID string) (*db.Poll, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("poll title is empty")
	}
	opts := normalizePollOptionStrings(options)
	if len(opts) == 0 {
		return nil, fmt.Errorf("poll needs at least one option")
	}
	groupLocal, err := normalizeGroupLocalPart(groupID)
	if err != nil {
		return nil, err
	}
	jid := types.NewJID(groupLocal, types.GroupServer)

	metaJSON, err := pollOptionsMetaJSON(opts)
	if err != nil {
		return nil, err
	}

	chunks := chunkPollOptions(opts, maxPollOptionsPerWA)
	n := len(chunks)

	row := db.Poll{
		MessageKeys: pq.StringArray{},
		OptionsMeta: metaJSON,
	}
	if err := h.db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create poll row: %w", err)
	}

	stanzaIDs := make([]string, 0, n)
	for i, chunk := range chunks {
		partTitle := pollPartTitle(title, i+1, n)
		pollMsg := h.client.BuildPollCreation(partTitle, chunk, len(chunk))
		resp, err := h.client.SendMessage(ctx, jid, pollMsg)
		if err != nil {
			_ = h.db.Delete(&db.Poll{}, row.ID)
			return nil, fmt.Errorf("send poll part %d/%d: %w", i+1, n, err)
		}
		stanzaIDs = append(stanzaIDs, string(resp.ID))
	}

	row.MessageKeys = pq.StringArray(stanzaIDs)
	if err := h.db.Save(&row).Error; err != nil {
		return nil, fmt.Errorf("save poll message_keys: %w", err)
	}

	return &row, nil
}

// ErrCollectivePollNotFound means no polls.message_keys contains the poll creation stanza from the vote.
var ErrCollectivePollNotFound = errors.New("collective poll not found for poll creation message key")

// HandlePollVote handles an incoming PollUpdateMessage: verifies kind, decrypts the vote payload, derives sender and
// poll creation stanza id, then finds the collective poll and upserts this user's vote slice for that stanza id
// (hex option hashes); other shards in votes JSON are unchanged. If a splitbot_totals row is linked to the poll
// and has not been exported to a sheet, assignments are merged from the current vote snapshot. If it has been
// exported, the voter is asked to update the Google Sheet instead.
func (h *Handler) HandlePollVote(ctx context.Context, evt *events.Message) error {
	if ctx == nil {
		ctx = context.Background()
	}
	stanzaID, userID, vote, err := h.parsePollVoteInput(ctx, evt)
	if err != nil {
		return err
	}
	poll, err := h.collectivePollByCreationStanza(ctx, stanzaID)
	if err == ErrCollectivePollNotFound {
		// There is a possibility that the poll message was for some other poll in the group, so we just ignore it
		fmt.Printf("collective poll not found for poll creation message key %s", stanzaID)
		return nil
	}
	if err != nil {
		return err
	}
	selected := pollVoteSelectedHashesHex(vote)
	var sync pollTotalsSync
	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertPollUserVote(tx, poll.ID, userID, stanzaID, selected); err != nil {
			return err
		}
		var syncErr error
		sync, syncErr = h.syncTotalsAssignmentsFromPoll(tx, poll.ID)
		return syncErr
	})
	if err != nil {
		return err
	}
	if sync.SkipForSheet {
		h.notifyExportedSheetVote(sync.GroupID, userID, sync.SheetGID)
	}
	return nil
}

// parsePollVoteInput validates evt as a poll vote, decrypts payload, and returns creation stanza id, sender id, proto vote.
func (h *Handler) parsePollVoteInput(ctx context.Context, evt *events.Message) (stanzaID, userID string, vote *waProto.PollVoteMessage, err error) {
	if evt == nil || evt.Message == nil {
		return "", "", nil, fmt.Errorf("message event is nil")
	}
	if kind := messageType(evt); kind != MessageTypePollVote {
		return "", "", nil, fmt.Errorf("not a poll vote message (got %q)", kind)
	}

	vote, err = h.getPollUpdate(ctx, evt)
	if err != nil {
		return "", "", nil, fmt.Errorf("decrypt poll vote: %w", err)
	}
	if vote == nil {
		return "", "", nil, fmt.Errorf("poll vote message is nil after decrypt")
	}

	pu := evt.Message.GetPollUpdateMessage()
	if pu == nil {
		return "", "", nil, fmt.Errorf("poll update message missing")
	}
	key := pu.GetPollCreationMessageKey()
	if key == nil {
		return "", "", nil, fmt.Errorf("poll creation message key is nil")
	}
	stanzaID = key.GetID()
	if stanzaID == "" {
		return "", "", nil, fmt.Errorf("poll creation message key id is empty")
	}
	userID = strings.TrimSpace(cleanSenderID(evt.Info.Sender.String()))
	if userID == "" {
		return "", "", nil, fmt.Errorf("user id is empty")
	}
	return stanzaID, userID, vote, nil
}

// collectivePollByCreationStanza loads the db.Poll whose message_keys contain the given WhatsApp poll creation stanza id.
func (h *Handler) collectivePollByCreationStanza(ctx context.Context, stanzaID string) (*db.Poll, error) {
	var poll db.Poll
	err := h.db.WithContext(ctx).
		Where("message_keys @> ?", pq.Array([]string{stanzaID})).
		First(&poll).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCollectivePollNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lookup collective poll: %w", err)
	}
	return &poll, nil
}

// upsertPollUserVote creates or updates votes for (pollID, userID), setting the slice for stanzaID to selectedHex.
func upsertPollUserVote(tx *gorm.DB, pollID int, userID, stanzaID string, selectedHex []string) error {
	var row db.Vote
	err := tx.Where("poll_id = ? AND user_id = ?", pollID, userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		vm := map[string][]string{
			stanzaID: append([]string(nil), selectedHex...),
		}
		raw, mErr := json.Marshal(vm)
		if mErr != nil {
			return fmt.Errorf("marshal votes: %w", mErr)
		}
		row = db.Vote{
			PollID: pollID,
			UserID: userID,
			Votes:  raw,
		}
		if err := tx.Create(&row).Error; err != nil {
			return fmt.Errorf("create vote: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("load vote: %w", err)
	}

	vm, err := decodeVotesByMessageKey(row.Votes)
	if err != nil {
		return err
	}
	vm[stanzaID] = append([]string(nil), selectedHex...)
	raw, err := json.Marshal(vm)
	if err != nil {
		return fmt.Errorf("marshal votes: %w", err)
	}
	row.Votes = raw
	if err := tx.Save(&row).Error; err != nil {
		return fmt.Errorf("update vote: %w", err)
	}
	return nil
}

// GetPollStatus loads votes for the collective poll id and groups voters by option label (from options_meta).
// Every option present in options_meta is returned; Users is empty when nobody voted for that option.
// Option rows are sorted by Option; Users are sorted lexically.
func (h *Handler) GetPollStatus(ctx context.Context, pollID int) ([]OptionSelection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return getPollStatus(h.db.WithContext(ctx), pollID)
}

func getPollStatus(tx *gorm.DB, pollID int) ([]OptionSelection, error) {
	if pollID <= 0 {
		return nil, fmt.Errorf("invalid poll id")
	}

	var poll db.Poll
	if err := tx.First(&poll, pollID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll %d not found: %w", pollID, err)
		}
		return nil, fmt.Errorf("load poll: %w", err)
	}

	meta, err := decodePollOptionsMeta(poll.OptionsMeta)
	if err != nil {
		return nil, err
	}

	usersByOption := make(map[string]map[string]struct{})
	for _, label := range meta {
		if usersByOption[label] == nil {
			usersByOption[label] = make(map[string]struct{})
		}
	}

	var voteRows []db.Vote
	if err := tx.Where("poll_id = ?", pollID).Find(&voteRows).Error; err != nil {
		return nil, fmt.Errorf("load votes: %w", err)
	}

	for _, vr := range voteRows {
		vm, err := decodeVotesByMessageKey(vr.Votes)
		if err != nil {
			return nil, fmt.Errorf("decode votes for user %s: %w", vr.UserID, err)
		}
		for _, hashes := range vm {
			for _, hashHex := range hashes {
				label, ok := meta[hashHex]
				if !ok {
					continue
				}
				usersByOption[label][vr.UserID] = struct{}{}
			}
		}
	}

	labels := make([]string, 0, len(usersByOption))
	for label := range usersByOption {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	out := make([]OptionSelection, 0, len(labels))
	for _, label := range labels {
		set := usersByOption[label]
		uids := make([]string, 0, len(set))
		for u := range set {
			uids = append(uids, u)
		}
		sort.Strings(uids)
		out = append(out, OptionSelection{Option: label, Users: uids})
	}
	return out, nil
}

func decodePollOptionsMeta(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode options_meta: %w", err)
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

// decodeVotesByMessageKey parses votes JSON: stanza id -> hex-encoded option hashes.
func decodeVotesByMessageKey(raw json.RawMessage) (map[string][]string, error) {
	if len(raw) == 0 {
		return map[string][]string{}, nil
	}
	var m map[string][]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode votes json: %w", err)
	}
	if m == nil {
		return map[string][]string{}, nil
	}
	return m, nil
}

func pollVoteSelectedHashesHex(vote *waProto.PollVoteMessage) []string {
	selected := vote.GetSelectedOptions()
	out := make([]string, 0, len(selected))
	for _, b := range selected {
		out = append(out, hex.EncodeToString(b))
	}
	return out
}

func normalizePollOptionStrings(options []string) []string {
	out := make([]string, 0, len(options))
	for _, o := range options {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

func pollOptionsMetaJSON(options []string) (json.RawMessage, error) {
	hashes := wa.HashPollOptions(options)
	meta := make(map[string]string, len(options))
	for i, opt := range options {
		meta[hex.EncodeToString(hashes[i])] = opt
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal options_meta: %w", err)
	}
	return raw, nil
}

func chunkPollOptions(options []string, chunkSize int) [][]string {
	if chunkSize <= 0 {
		chunkSize = maxPollOptionsPerWA
	}
	var chunks [][]string
	for len(options) > 0 {
		end := chunkSize
		if end > len(options) {
			end = len(options)
		}
		part := append([]string(nil), options[:end]...)
		chunks = append(chunks, part)
		options = options[end:]
	}
	return chunks
}

func pollPartTitle(title string, part, total int) string {
	if total <= 1 {
		return title
	}
	return fmt.Sprintf("%s (part %d/%d)", title, part, total)
}

func normalizeGroupLocalPart(groupID string) (string, error) {
	s := strings.TrimSpace(groupID)
	if s == "" {
		return "", fmt.Errorf("group id is empty")
	}
	if strings.Contains(s, "@") {
		j, err := types.ParseJID(s)
		if err != nil {
			return "", fmt.Errorf("parse group jid: %w", err)
		}
		if j.Server != types.GroupServer {
			return "", fmt.Errorf("expected group jid (server g.us), got %s", j.Server)
		}
		return j.User, nil
	}
	return s, nil
}
