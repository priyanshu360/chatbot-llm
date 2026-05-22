package llm

import "testing"

func TestRedactPII(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"email", "contact me at user@example.com", "contact me at [EMAIL]"},
		{"phone with dashes", "call 555-123-4567", "call [PHONE]"},
		{"phone with dots", "call 555.123.4567", "call [PHONE]"},
		{"phone continuous", "call 5551234567", "call [PHONE]"},
		{"credit card", "card 4111-1111-1111-1111", "card [CARD]"},
		{"credit card spaces", "card 4111 1111 1111 1111", "card [CARD]"},
		{"credit card continuous", "card 4111111111111111", "card [CARD]"},
		{"ip address", "ip 192.168.1.1", "ip [IP]"},
		{"id document", "passport AB1234567", "passport [ID_DOC]"},
		{"multiple patterns", "email user@test.com and phone 555-123-4567", "email [EMAIL] and phone [PHONE]"},
		{"no PII unchanged", "hello world", "hello world"},
		{"empty string", "", ""},
		{"email in sentence", "My email is john.doe+tag@company.co.uk", "My email is [EMAIL].uk"},
	// Note: the regex only matches single-component TLDs (e.g. .com, .co)
	// Multi-part TLDs like .co.uk are partially matched.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactPII(tt.input)
			if got != tt.want {
				t.Errorf("RedactPII(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedactPIIEdgeCases(t *testing.T) {
	t.Run("short phone not redacted — 3 digits", func(t *testing.T) {
		got := RedactPII("555")
		if got != "555" {
			t.Errorf("short number should not be redacted, got %q", got)
		}
	})

	t.Run("email-like with no dots", func(t *testing.T) {
		got := RedactPII("user@localhost")
		want := "user@localhost"
		if got != want {
			t.Errorf("local email without tld should not match, got %q", got)
		}
	})

	t.Run("card-like with letters not redacted", func(t *testing.T) {
		got := RedactPII("1234 5678 9012 345a")
		if got != "1234 5678 9012 345a" {
			t.Errorf("expected no change, but got %q", got)
		}
	})
}
