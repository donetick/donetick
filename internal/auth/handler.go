package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	tokenService *TokenService
	userRepo     *uRepo.UserRepository
	jwtAuth      *jwt.GinJWTMiddleware
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tokenService *TokenService, userRepo *uRepo.UserRepository, jwtAuth *jwt.GinJWTMiddleware) *AuthHandler {
	return &AuthHandler{
		tokenService: tokenService,
		userRepo:     userRepo,
		jwtAuth:      jwtAuth,
	}
}

// RefreshTokenHandler handles POST /auth/refresh
func (h *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	// Try to get refresh token from cookie first (httpOnly), fallback to body (legacy)
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		// Fallback to JSON body for backward compatibility
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No refresh token provided",
			})
			return
		}
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Refresh token required",
		})
		return
	}

	// Validate and refresh tokens
	tokenResponse, err := h.tokenService.RefreshTokens(c.Request.Context(), refreshToken)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to refresh token", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired refresh token",
			"code":  "INVALID_REFRESH_TOKEN",
		})
		return
	}

	// Set new refresh token as httpOnly cookie
	c.SetCookie("refresh_token", tokenResponse.RefreshToken, int(h.tokenService.refreshTokenExpiry.Seconds()), "/", "", true, true)

	c.JSON(http.StatusOK, tokenResponse)
}

// LogoutHandler handles POST /auth/logout
func (h *AuthHandler) LogoutHandler(c *gin.Context) {
	// Try to get refresh token from cookie first (httpOnly), fallback to body (legacy)
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		// Fallback to JSON body for backward compatibility
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// No token provided, but logout should still succeed
			refreshToken = ""
		} else {
			refreshToken = req.RefreshToken
		}
	}

	// Revoke the refresh token if we have one
	if refreshToken != "" {
		if err := h.tokenService.RevokeToken(c.Request.Context(), refreshToken); err != nil {
			logging.FromContext(c).Errorw("Failed to revoke token", "error", err)
			// Don't return error - logout should always succeed from client perspective
		}
	}

	// Clear the httpOnly cookie
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// RevokeAllHandler handles POST /auth/revoke-all
func (h *AuthHandler) RevokeAllHandler(c *gin.Context) {
	// Get current user from JWT token
	currentUser, ok := CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	// Revoke all sessions for the user
	if err := h.tokenService.RevokeAllUserTokens(c.Request.Context(), currentUser.ID); err != nil {
		logging.FromContext(c).Errorw("Failed to revoke all tokens", "error", err, "userID", currentUser.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to revoke sessions",
		})
		return
	}

	// Clear the httpOnly cookie since all sessions are revoked
	c.SetCookie("refresh_token", "", -1, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "All sessions revoked successfully",
	})
}

// EnhancedLoginHandler provides enhanced login with refresh tokens
func (h *AuthHandler) EnhancedLoginHandler(c *gin.Context) {
	var loginReq struct {
		Username string `form:"username" json:"username" binding:"required"`
		Password string `form:"password" json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// Authenticate using the existing authenticator logic
	userDetails, err := h.authenticateUser(c, loginReq.Username, loginReq.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid credentials",
		})
		return
	}

	// Generate both access and refresh tokens
	tokenResponse, err := h.tokenService.GenerateTokens(c.Request.Context(), userDetails)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to generate tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate tokens",
		})
		return
	}
	c.SetCookie("refresh_token", tokenResponse.RefreshToken, int(h.tokenService.refreshTokenExpiry.Seconds()), "/", "", true, true)
	c.JSON(http.StatusOK, tokenResponse)
}

// authenticateUser performs user authentication (extracted from JWT middleware)
func (h *AuthHandler) authenticateUser(c *gin.Context, username, password string) (*uModel.UserDetails, error) {
	user, err := h.userRepo.GetUserByUsername(c.Request.Context(), username)
	if err != nil || user.Disabled {
		return nil, fmt.Errorf("user not found or disabled")
	}

	err = Matches(user.Password, password)
	if err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// Handle MFA if enabled (simplified for now)
	if user.MFAEnabled {
		return nil, fmt.Errorf("MFA verification required")
	}

	return &uModel.UserDetails{
		User: uModel.User{
			ID:        user.ID,
			Username:  user.Username,
			Password:  "",
			Image:     user.Image,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Disabled:  user.Disabled,
			CircleID:  user.CircleID,
		},
		WebhookURL: user.WebhookURL,
	}, nil
}

// Enhanced login response that uses token service
func (h *AuthHandler) EnhancedLoginResponse(c *gin.Context, code int, token string, expire time.Time, user *uModel.UserDetails) {
	// Generate refresh token along with access token
	tokenResponse, err := h.tokenService.GenerateTokens(context.TODO(), user)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to generate refresh token", "error", err)
		// Fallback to original response
		c.JSON(code, gin.H{
			"code":   code,
			"token":  token,
			"expire": expire,
		})
		return
	}

	c.JSON(code, tokenResponse)
}

// CleanupExpiredSessions runs periodic cleanup of expired sessions
func (h *AuthHandler) CleanupExpiredSessions(ctx context.Context) error {
	return h.userRepo.CleanupExpiredSessions(ctx)
}
