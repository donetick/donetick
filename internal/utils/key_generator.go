package utils

import (
	"encoding/base64"

	crand "crypto/rand"

	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

func GenerateInviteCode(c *gin.Context) string {
	logger := logging.FromContext(c)
	// Define the length of the token (in bytes). For example, 32 bytes will result in a 44-character base64-encoded token.
	tokenLength := 12

	// Generate a random byte slice.
	tokenBytes := make([]byte, tokenLength)
	_, err := crand.Read(tokenBytes)
	if err != nil {
		logger.Errorw("utility.GenerateEmailResetToken failed to generate random bytes", "err", err)
	}

	// Encode the byte slice to a base64 string.
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	return token
}
