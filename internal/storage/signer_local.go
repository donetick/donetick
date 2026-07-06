package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"donetick.com/core/config"
)

const localSignedURLTTL = 7 * 24 * time.Hour

type URLSignerLocal struct {
	Secret []byte
}

func NewURLSignerLocal(config *config.Config) *URLSignerLocal {
	return &URLSignerLocal{Secret: []byte(config.Jwt.Secret)}
}

func (s *URLSignerLocal) Sign(rawPath string) (string, error) {
	expires := time.Now().UTC().Add(localSignedURLTTL).Unix()
	sig := s.sign(rawPath, expires)
	values := url.Values{}
	values.Set("expires", strconv.FormatInt(expires, 10))
	values.Set("sig", sig)
	return fmt.Sprintf("%s?%s", rawPath, values.Encode()), nil
}

func (s *URLSignerLocal) sign(path string, expires int64) string {
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(fmt.Sprintf("%s:%d", path, expires)))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *URLSignerLocal) IsValid(rawPath string, query url.Values) bool {
	expiresStr := query.Get("expires")
	providedSig := query.Get("sig")
	if expiresStr == "" || providedSig == "" {
		return false
	}
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expires {
		return false
	}
	expected := s.sign(rawPath, expires)
	return hmac.Equal([]byte(expected), []byte(providedSig))
}

func (s *URLSignerLocal) SignIfLocal(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	signed, err := s.Sign(path)
	if err != nil {
		return ""
	}
	return signed
}

func (s *URLSignerLocal) SignAndGetPublicURL(rawPath string) (string, error) {
	// For local storage, we can return a signed URL since it's not publicly accessible.
	return s.Sign(rawPath)
}
