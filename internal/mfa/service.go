package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	"donetick.com/core/config"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type MFAService struct {
	appName string
}

func NewService(cfg *config.Config) *MFAService {
	appName := "Donetick"
	if cfg.Name != "" {
		appName = cfg.Name
	}
	return &MFAService{
		appName: appName,
	}
}

func NewMFAService(appName string) *MFAService {
	return &MFAService{
		appName: appName,
	}
}

func (s *MFAService) GenerateSecret(userEmail string) (*otp.Key, error) {
	return totp.Generate(totp.GenerateOpts{
		Issuer:      s.appName,
		AccountName: userEmail,
		SecretSize:  32,
	})
}

func (s *MFAService) VerifyTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

func (s *MFAService) GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := s.generateRandomCode(8)
		if err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}

func (s *MFAService) VerifyBackupCode(backupCodesJSON, usedCodesJSON, inputCode string) (bool, string, error) {
	var backupCodes []string
	var usedCodes []string

	if backupCodesJSON != "" {
		if err := json.Unmarshal([]byte(backupCodesJSON), &backupCodes); err != nil {
			return false, "", err
		}
	}

	if usedCodesJSON != "" {
		if err := json.Unmarshal([]byte(usedCodesJSON), &usedCodes); err != nil {
			return false, "", err
		}
	}

	// Normalize input code
	inputCode = strings.ToUpper(strings.ReplaceAll(inputCode, "-", ""))

	// Check if code exists and is not used
	for _, code := range backupCodes {
		normalizedCode := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
		if normalizedCode == inputCode {
			// Check if already used
			for _, used := range usedCodes {
				if strings.ToUpper(strings.ReplaceAll(used, "-", "")) == inputCode {
					return false, "", nil // Code already used
				}
			}

			// Mark as used
			usedCodes = append(usedCodes, code)
			updatedUsedJSON, err := json.Marshal(usedCodes)
			if err != nil {
				return false, "", err
			}

			return true, string(updatedUsedJSON), nil
		}
	}

	return false, "", nil // Code not found
}

func (s *MFAService) generateRandomCode(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}

	// Format as XXXX-XXXX for readability :)
	code := string(b)
	if length == 8 {
		return fmt.Sprintf("%s-%s", code[:4], code[4:]), nil
	}
	return code, nil
}

func (s *MFAService) GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(b), nil
}

func (s *MFAService) IsCodeValid(secret, backupCodesJSON, usedCodesJSON, inputCode string) (bool, string, error) {
	// First try TOTP
	if s.VerifyTOTP(secret, inputCode) {
		return true, usedCodesJSON, nil
	}

	// Then try backup codes
	if backupCodesJSON != "" {
		valid, newUsedJSON, err := s.VerifyBackupCode(backupCodesJSON, usedCodesJSON, inputCode)
		return valid, newUsedJSON, err
	}

	return false, usedCodesJSON, nil
}
