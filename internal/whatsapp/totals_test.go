package whatsapp

import "testing"

func TestMentionUser(t *testing.T) {
	t.Parallel()
	if got := mentionUser(" 12345@lid "); got != "@12345" {
		t.Fatalf("got %q", got)
	}
	if got := mentionUser(""); got != "" {
		t.Fatalf("empty got %q", got)
	}
}
