package auth

import (
	"net/http"

	"donetick.com/core/internal/mfa"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

// APITokenMiddleware provides authentication via API tokens
func APITokenMiddleware(userRepo *uRepo.UserRepository) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		apiToken := c.GetHeader("secretkey")
		if apiToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API token required"})
			c.Abort()
			return
		}

		user, err := userRepo.GetUserByToken(c, apiToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API token"})
			c.Abort()
			return
		}

		if user.Disabled {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User account is disabled"})
			c.Abort()
			return
		}

		// Set the user in context
		c.Set(identityKey, user)
		c.Next()
	})
}

// OptionalMFAMiddleware provides optional MFA verification for API endpoints
// This middleware checks for an optional MFA code in headers for enhanced security
func OptionalMFAMiddleware(userRepo *uRepo.UserRepository) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		logger := logging.FromContext(c)

		// Get current user from context (set by APITokenMiddleware or JWT middleware)
		user, exists := c.Get(identityKey)
		if !exists {
			c.Next()
			return
		}

		userDetails, ok := user.(*uModel.UserDetails)
		if !ok {
			c.Next()
			return
		}

		// Check if user has MFA enabled and if an MFA code is provided
		mfaCode := c.GetHeader("X-MFA-Code")
		if userDetails.MFAEnabled && mfaCode != "" {
			mfaService := mfa.NewMFAService("Donetick")
			valid, newUsedCodes, err := mfaService.IsCodeValid(
				userDetails.MFASecret,
				userDetails.MFABackupCodes,
				userDetails.MFARecoveryUsed,
				mfaCode,
			)

			if err != nil {
				logger.Errorw("Error validating MFA code in API request", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate MFA code"})
				c.Abort()
				return
			}

			if !valid {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid MFA code"})
				c.Abort()
				return
			}

			// Update used codes if a backup code was used
			if newUsedCodes != userDetails.MFARecoveryUsed {
				if err := userRepo.UpdateMFARecoveryCodes(c, userDetails.ID, newUsedCodes); err != nil {
					logger.Errorw("Failed to update recovery codes", "error", err)
				}
			}

			// Mark request as MFA verified for downstream handlers
			c.Set("mfa_verified", true)
		}

		c.Next()
	})
}

// RequireMFAMiddleware requires MFA verification for API endpoints when user has MFA enabled
func RequireMFAMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Get current user from context
		user, exists := c.Get(identityKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userDetails, ok := user.(*uModel.UserDetails)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
			c.Abort()
			return
		}

		// If user has MFA enabled, require MFA verification
		if userDetails.MFAEnabled {
			mfaVerified, exists := c.Get("mfa_verified")
			if !exists || !mfaVerified.(bool) {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "MFA verification required",
					"message": "This endpoint requires MFA verification. Please provide a valid MFA code in the X-MFA-Code header.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	})
}

// RequirePlusMemberMiddleware requires that the authenticated user is a plus member
func RequirePlusMemberMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Get current user from context (should be set by APITokenMiddleware)
		user, exists := c.Get(identityKey)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		userDetails, ok := user.(*uModel.UserDetails)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
			c.Abort()
			return
		}

		// Check if user is a plus member
		if !userDetails.IsPlusMember() {
			c.JSON(http.StatusForbidden, gin.H{"error": "Only plus members can access this endpoint"})
			c.Abort()
			return
		}

		c.Next()
	})
}
