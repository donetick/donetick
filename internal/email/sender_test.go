package email

import (
	"strings"
	"testing"
)

func TestEncodeDecodeEmailAndCode_RoundTrip(t *testing.T) {
	cases := []struct{ email, code string }{
		{"evan@example.com", "abc123"},
		{"a.b@example.co.uk", "Zm9vYmFyX2Jhei0xMjM"},
		{"someone+tagged@example.org", "tok_with-url_safe-chars"},
	}
	for _, tc := range cases {
		enc := encodeEmailAndCode(tc.email, tc.code)

		// Must be URL-safe so it survives query parsing untouched (the bug).
		if strings.ContainsAny(enc, "+/") {
			t.Fatalf("encoded value contains URL-unsafe chars %q for (%q,%q)", enc, tc.email, tc.code)
		}

		gotEmail, gotCode, err := DecodeEmailAndCode(enc)
		if err != nil {
			t.Fatalf("DecodeEmailAndCode(%q) error: %v", enc, err)
		}
		if gotEmail != tc.email || gotCode != tc.code {
			t.Fatalf("round-trip mismatch: got (%q,%q), want (%q,%q)", gotEmail, gotCode, tc.email, tc.code)
		}
	}
}
