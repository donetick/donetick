package storage

import (
	"net/url"
	"testing"
	"time"

	"donetick.com/core/config"
)

func TestSignReturnsDifferentSignaturesForDifferentPaths(t *testing.T) {
	signer := NewURLSignerLocal(&config.Config{
		Jwt: config.JwtConfig{
			Secret: "secret",
		},
	})

	url1, _ := signer.Sign("/a")
	url2, _ := signer.Sign("/b")
	if url1 == url2 {
		t.Error("Sign returned same signature for different paths")
	}
}

func TestSignReturnsDifferentSignaturesForDifferentExpiry(t *testing.T) {
	signer := NewURLSignerLocal(&config.Config{
		Jwt: config.JwtConfig{
			Secret: "secret",
		},
	})
	url1, _ := signer.Sign("/same")
	time.Sleep(2 * time.Second)
	url2, _ := signer.Sign("/same")
	if url1 != url2 {
		t.Error("Sign returned different signatures for same path")
	}
}

func TestSigntureIsValid(t *testing.T) {
	signer := NewURLSignerLocal(&config.Config{
		Jwt: config.JwtConfig{
			Secret: "secret",
		},
	})
	url1, _ := signer.Sign("/same")
	parsed, err := url.Parse(url1)
	if err != nil {
		t.Error("Failed to parse signed URL")
	}
	sig := parsed.Query().Get("sig")
	if !signer.IsValid(parsed.Path, sig) {
		t.Error("Signature is not valid")
	}
}

// test creating signed url for this chore/1/3f67e1d0-4a88-48ea-a815-21533f0a823a.png:
func TestSignReturnsValidURL(t *testing.T) {
	signer := NewURLSignerLocal(&config.Config{
		Jwt: config.JwtConfig{
			Secret: "secret",
		},
	})
	url1, err := signer.Sign("chore/1/3f67e1d0-4a88-48ea-a815-21533f0a823a.png")
	if err != nil {
		t.Error(url1)
	}

	parsed, err := url.Parse(url1)
	if err != nil {
		t.Error("Failed to parse signed URL")
	}
	sig := parsed.Query().Get("sig")
	if !signer.IsValid(parsed.Path, sig) {
		t.Errorf("Signature is not valid: want valid signature for path %q, got invalid", parsed.Path)

	}

}
