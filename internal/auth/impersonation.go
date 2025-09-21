package auth

import (
	"strconv"

	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

const (
	ImpersonateUserIDHeader = "X-Impersonate-User-ID"
	ImpersonatedUserKey     = "impersonated_user"
	ActualUserKey           = "id"
)

// ImpersonationMiddleware handles user impersonation for admin/manager users
func ImpersonationMiddleware(userRepo *uRepo.UserRepository, circleRepo *cRepo.CircleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger := logging.FromContext(c)

		// Get the current authenticated user
		currentUser, ok := CurrentUser(c)
		if !ok {
			c.Next()
			return
		}

		// Check if impersonation is requested
		impersonateUserIDStr := c.GetHeader(ImpersonateUserIDHeader)
		if impersonateUserIDStr == "" {
			c.Next()
			return
		}

		// Parse the impersonate user ID
		impersonateUserID, err := strconv.Atoi(impersonateUserIDStr)
		if err != nil {
			logger.Error("Invalid impersonate user ID", "impersonateUserID", impersonateUserIDStr, "error", err)
			c.JSON(400, gin.H{"error": "Invalid impersonate user ID"})
			c.Abort()
			return
		}

		// Get circle users to validate permissions and membership
		circleUsers, err := circleRepo.GetCircleUsers(c, currentUser.CircleID)
		if err != nil {
			logger.Error("Failed to get circle users", "error", err, "circleID", currentUser.CircleID)
			c.JSON(500, gin.H{"error": "Failed to validate permissions"})
			c.Abort()
			return
		}

		// Check if current user has permission to impersonate (admin or manager)
		var currentUserRole cModel.UserRole
		var impersonateUserInCircle bool

		for _, circleUser := range circleUsers {
			if circleUser.UserID == currentUser.ID {
				currentUserRole = circleUser.Role
			}
			if circleUser.UserID == impersonateUserID {
				impersonateUserInCircle = true
			}
		}

		// Validate current user has admin or manager role
		if currentUserRole != cModel.UserRoleAdmin && currentUserRole != cModel.UserRoleManager {
			logger.Warn("User attempted impersonation without permission",
				"userID", currentUser.ID, "role", currentUserRole, "impersonateUserID", impersonateUserID)
			c.JSON(403, gin.H{"error": "Insufficient permissions for impersonation"})
			c.Abort()
			return
		}

		// Validate impersonate user is in the same circle
		if !impersonateUserInCircle {
			logger.Warn("User attempted to impersonate user outside circle",
				"userID", currentUser.ID, "impersonateUserID", impersonateUserID, "circleID", currentUser.CircleID)
			c.JSON(403, gin.H{"error": "Cannot impersonate user outside your circle"})
			c.Abort()
			return
		}

		// Get full user details for the impersonated user
		impersonatedUser, err := userRepo.GetUserByID(c, impersonateUserID)
		if err != nil {
			logger.Error("Failed to get impersonated user details", "error", err, "impersonateUserID", impersonateUserID)
			c.JSON(500, gin.H{"error": "Failed to get impersonated user details"})
			c.Abort()
			return
		}

		// Create UserDetails for the impersonated user by copying all fields
		impersonatedUserDetails := &uModel.UserDetails{
			User:       *impersonatedUser,
			WebhookURL: nil, // This will be populated from circle if needed
		}

		// Clear the password for security
		impersonatedUserDetails.Password = ""

		// Store both users in context
		c.Set(ActualUserKey, currentUser)
		c.Set(ImpersonatedUserKey, impersonatedUserDetails)

		logger.Debugw("User impersonation started",
			"actualUserID", currentUser.ID,
			"actualUsername", currentUser.Username,
			"impersonatedUserID", impersonatedUser.ID,
			"impersonatedUsername", impersonatedUser.Username)

		c.Next()
	}
}

// CurrentUserWithImpersonation returns both the actual user and impersonated user (if any)
// Returns (actualUser, impersonatedUser, hasImpersonation)
func CurrentUserWithImpersonation(c *gin.Context) (*uModel.UserDetails, *uModel.UserDetails, bool) {
	actualUser, ok := c.Get(ActualUserKey)
	if !ok {
		// No impersonation, return current user
		currentUser, exists := CurrentUser(c)
		return currentUser, nil, exists && false
	}

	impersonatedUser, impersonationExists := c.Get(ImpersonatedUserKey)
	if !impersonationExists {
		// Fallback to current user
		currentUser, exists := CurrentUser(c)
		return currentUser, nil, exists && false
	}

	actualUserDetails, actualOk := actualUser.(*uModel.UserDetails)
	impersonatedUserDetails, impersonatedOk := impersonatedUser.(*uModel.UserDetails)

	if !actualOk || !impersonatedOk {
		// Fallback to current user
		currentUser, exists := CurrentUser(c)
		return currentUser, nil, exists && false
	}

	return actualUserDetails, impersonatedUserDetails, true
}

// GetEffectiveUser returns the user that should be used for operations
// If impersonating, returns the impersonated user, otherwise returns the current user
func GetEffectiveUser(c *gin.Context) (*uModel.UserDetails, bool) {
	_, impersonatedUser, hasImpersonation := CurrentUserWithImpersonation(c)
	if hasImpersonation {
		return impersonatedUser, true
	}

	return CurrentUser(c)
}

// GetActualUser returns the actual authenticated user (ignoring impersonation)
func GetActualUser(c *gin.Context) (*uModel.UserDetails, bool) {
	actualUser, _, hasImpersonation := CurrentUserWithImpersonation(c)
	if hasImpersonation {
		return actualUser, true
	}

	return CurrentUser(c)
}
