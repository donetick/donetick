package apple

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"donetick.com/core/config"
	"github.com/golang-jwt/jwt/v5"
)

type AppleService struct {
	clientID    string
	keys        map[string]*rsa.PublicKey
	keysFetched time.Time
}

type AppleUserInfo struct {
	Sub            string `json:"sub"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	IsPrivateEmail bool   `json:"is_private_email"`
	GivenName      string `json:"given_name,omitempty"`
	FamilyName     string `json:"family_name,omitempty"`
}

type ApplePublicKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type AppleKeysResponse struct {
	Keys []ApplePublicKey `json:"keys"`
}

type AppleClaims struct {
	jwt.RegisteredClaims
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	IsPrivateEmail bool   `json:"is_private_email"`
}

func NewAppleService(config *config.Config) *AppleService {
	return &AppleService{
		clientID: config.DonetickCloudConfig.AppleClientID,
		keys:     make(map[string]*rsa.PublicKey),
	}
}

func (s *AppleService) ValidateIDToken(ctx context.Context, idToken string) (*AppleUserInfo, error) {
	// Parse the JWT token to get the header
	token, err := jwt.ParseWithClaims(idToken, &AppleClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get key ID from header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		// Get the public key for validation
		publicKey, err := s.getPublicKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*AppleClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token or claims")
	}

	// Validate audience (client ID)
	if len(claims.Audience) == 0 || claims.Audience[0] != s.clientID {
		return nil, fmt.Errorf("invalid audience: expected %s", s.clientID)
	}

	// Validate issuer
	if claims.Issuer != "https://appleid.apple.com" {
		return nil, fmt.Errorf("invalid issuer")
	}

	// Validate expiration
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return &AppleUserInfo{
		Sub:            claims.Subject,
		Email:          claims.Email,
		EmailVerified:  claims.EmailVerified,
		IsPrivateEmail: claims.IsPrivateEmail,
	}, nil
}

func (s *AppleService) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// Check if we have the key and it's not too old (refresh every hour)
	if key, exists := s.keys[kid]; exists && time.Since(s.keysFetched) < time.Hour {
		return key, nil
	}

	// Fetch keys from Apple
	if err := s.fetchApplePublicKeys(ctx); err != nil {
		return nil, err
	}

	key, exists := s.keys[kid]
	if !exists {
		return nil, fmt.Errorf("key with kid %s not found", kid)
	}

	return key, nil
}

func (s *AppleService) fetchApplePublicKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://appleid.apple.com/auth/keys", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch keys: status %d", resp.StatusCode)
	}

	var keysResponse AppleKeysResponse
	if err := json.NewDecoder(resp.Body).Decode(&keysResponse); err != nil {
		return fmt.Errorf("failed to decode keys response: %w", err)
	}

	// Clear existing keys
	s.keys = make(map[string]*rsa.PublicKey)

	// Convert Apple keys to RSA public keys
	for _, key := range keysResponse.Keys {
		if key.Kty != "RSA" {
			continue
		}

		publicKey, err := parseRSAPublicKey(key.N, key.E)
		if err != nil {
			return fmt.Errorf("failed to parse RSA public key for kid %s: %w", key.Kid, err)
		}

		s.keys[key.Kid] = publicKey
	}

	s.keysFetched = time.Now()
	return nil
}

func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	// Add padding if needed for base64 decoding
	nStr = addPadding(nStr)
	eStr = addPadding(eStr)

	nBytes, err := base64.URLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode n: %w", err)
	}

	eBytes, err := base64.URLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode e: %w", err)
	}

	// Convert bytes to big.Int for N
	n := new(big.Int)
	n.SetBytes(nBytes)

	// Convert bytes to int for E
	var e int
	for _, b := range eBytes {
		e = e*256 + int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

func addPadding(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}
