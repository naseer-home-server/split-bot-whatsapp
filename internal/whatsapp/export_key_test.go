package whatsapp

import "testing"

func TestNormalizeUserKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		" 123 ":      "123",
		"@123":       "123",
		"123@lid":    "123",
		"@123@lid":   "123",
		"+6012":      "6012",
		"123:12@lid": "123",
		"Naseer":     "Naseer",
	}
	for in, want := range cases {
		if got := normalizeUserKey(in); got != want {
			t.Errorf("normalizeUserKey(%q)=%q want %q", in, got, want)
		}
	}
}
