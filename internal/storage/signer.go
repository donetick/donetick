package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"

	"donetick.com/core/config"
)

type URLSigner struct {
	Secret []byte
}

func NewURLSigner(config *config.Config) *URLSigner {
	return &URLSigner{Secret: []byte(config.Jwt.Secret)}
}

// sign method without expiration:
func (s *URLSigner) Sign(rawPath string) (string, error) {
	sig := s.sign(rawPath)
	values := url.Values{}
	values.Set("sig", sig)

	return fmt.Sprintf("%s?%s", rawPath, values.Encode()), nil
}

func (s *URLSigner) sign(path string) string {
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(path))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *URLSigner) IsValid(rawPath string, providedSig string) bool {

	expectedSig := s.sign(rawPath)
	return hmac.Equal([]byte(expectedSig), []byte(providedSig))
}
