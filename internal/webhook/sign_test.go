package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestSignMatchesTheDocumentedScheme(t *testing.T) {
	ts := time.Unix(1764600000, 0)
	body := []byte(`{"type":"task.completed"}`)

	got := Sign("s3cret", ts, body)

	mac := hmac.New(sha256.New, []byte("s3cret"))
	mac.Write([]byte("1764600000." + string(body)))
	want := "t=1764600000,v1=" + hex.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}
}

func TestSignCoversTimestampAndBody(t *testing.T) {
	body := []byte(`{"type":"task.completed"}`)
	ts := time.Unix(1764600000, 0)

	base := Sign("s3cret", ts, body)

	if Sign("s3cret", ts.Add(time.Second), body) == base {
		t.Fatal("a different timestamp must produce a different signature")
	}
	if Sign("s3cret", ts, []byte(`{"type":"task.skipped"}`)) == base {
		t.Fatal("a different body must produce a different signature")
	}
	if Sign("other", ts, body) == base {
		t.Fatal("a different secret must produce a different signature")
	}
}

func TestNewDeliveryIDIsUnique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewDeliveryID()
		if id == "" {
			t.Fatal("delivery id is empty")
		}
		if seen[id] {
			t.Fatalf("duplicate delivery id %q", id)
		}
		seen[id] = true
	}
}

func TestSignatureHasNoWhitespace(t *testing.T) {
	// Receivers split on "," and "=", so stray whitespace would break them.
	if s := Sign("s3cret", time.Now(), []byte("{}")); strings.ContainsAny(s, " \t\n") {
		t.Fatalf("signature contains whitespace: %q", s)
	}
}
