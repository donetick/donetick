package storage

import (
	"net/url"
	"testing"
	"time"

	"donetick.com/core/config"
)

func newTestSigner() *URLSignerLocal {
	return NewURLSignerLocal(&config.Config{
		Jwt: config.JwtConfig{Secret: "secret"},
	})
}

func parseSignedURL(t *testing.T, raw string) (*url.URL, url.Values) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse signed URL: %v", err)
	}
	return parsed, parsed.Query()
}

func TestSignReturnsDifferentSignaturesForDifferentPaths(t *testing.T) {
	signer := newTestSigner()
	url1, _ := signer.Sign("/a")
	url2, _ := signer.Sign("/b")
	if url1 == url2 {
		t.Error("Sign returned same signature for different paths")
	}
}

func TestSignReturnsDifferentSignaturesForDifferentExpiry(t *testing.T) {
	signer := newTestSigner()
	url1, _ := signer.Sign("/same")
	time.Sleep(2 * time.Second)
	url2, _ := signer.Sign("/same")
	// Expiry timestamps differ by ~2s, so signatures must differ.
	if url1 == url2 {
		t.Error("Sign returned same signature despite different expiry timestamps")
	}
}

func TestSignatureIsValid(t *testing.T) {
	signer := newTestSigner()
	signed, _ := signer.Sign("/same")
	parsed, query := parseSignedURL(t, signed)
	if !signer.IsValid(parsed.Path, query) {
		t.Error("Signature should be valid immediately after signing")
	}
}

func TestSignatureIsInvalidAfterExpiry(t *testing.T) {
	signer := newTestSigner()
	// Manually craft an already-expired signed URL.
	expires := time.Now().Add(-1 * time.Hour).Unix()
	sig := signer.sign("/expired", expires)
	query := url.Values{}
	query.Set("expires", "0")
	query.Set("sig", sig)
	if signer.IsValid("/expired", query) {
		t.Error("Expired signature should be rejected")
	}
}

func TestSignatureIsInvalidWithTamperedPath(t *testing.T) {
	signer := newTestSigner()
	signed, _ := signer.Sign("chore/1/original.png")
	_, query := parseSignedURL(t, signed)
	if signer.IsValid("chore/1/tampered.png", query) {
		t.Error("Signature for a different path should be rejected")
	}
}

func TestSignReturnsValidURL(t *testing.T) {
	signer := newTestSigner()
	signed, err := signer.Sign("chore/1/3f67e1d0-4a88-48ea-a815-21533f0a823a.png")
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	parsed, query := parseSignedURL(t, signed)
	if !signer.IsValid(parsed.Path, query) {
		t.Errorf("Signature is not valid for path %q", parsed.Path)
	}
}
