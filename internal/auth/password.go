package auth

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"

	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+-=[]{}|;':,.<>?/~"

func EncodePassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func Matches(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func GenerateRandomPassword(length int) string {
	// Create a buffer to hold the random bytes.
	buffer := make([]byte, length)

	// Compute the maximum index for the characters.
	maxIndex := big.NewInt(int64(len(chars)))

	// Generate random bytes and use them to select characters from the set.
	for i := 0; i < length; i++ {
		randomIndex, _ := rand.Int(rand.Reader, maxIndex)
		buffer[i] = chars[randomIndex.Int64()]
	}

	return string(buffer)
}

func GenerateEmailResetToken(c *gin.Context) (string, error) {
	logger := logging.FromContext(c)
	// Define the length of the token (in bytes). For example, 32 bytes will result in a 44-character base64-encoded token.
	tokenLength := 32

	// Generate a random byte slice.
	tokenBytes := make([]byte, tokenLength)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		logger.Errorw("password.GenerateEmailResetToken failed to generate random bytes", "err", err)
		return "", err
	}

	// Encode the byte slice to a base64 string.
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	return token, nil
}
