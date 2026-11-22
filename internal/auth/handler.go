package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"donetick.com/core/internal/mfa"
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
	mfaService   *mfa.MFAService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tokenService *TokenService, userRepo *uRepo.UserRepository, jwtAuth *jwt.GinJWTMiddleware, mfaService *mfa.MFAService) *AuthHandler {
	return &AuthHandler{
		tokenService: tokenService,
		userRepo:     userRepo,
		jwtAuth:      jwtAuth,
		mfaService:   mfaService,
	}
}

// RefreshTokenHandler handles POST /auth/refresh
//
//	@Summary		Refresh tokens
//	@Description	Exchanges a refresh token (refresh_token cookie, or refresh_token in the body) for new access and refresh tokens.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshRequest		false	"Refresh token, if not sent as a cookie"
//	@Success		200		{object}	TokenResponse		"tokens"
//	@Failure		400		{object}	map[string]string	"error: Invalid request | Refresh token required"
//	@Failure		401		{object}	map[string]string	"error: Invalid or expired refresh token"
//	@Router			/api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshTokenHandler(c *gin.Context) {
	// Try to get refresh token from cookie first (httpOnly), fallback to body (legacy)
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		// Fallback to JSON body for backward compatibility
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request: " + err.Error(),
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
//
//	@Summary		Log out (revoke refresh token)
//	@Description	Revokes the refresh token (cookie or body) and clears the cookie. Always succeeds.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshRequest		false	"Refresh token, if not sent as a cookie"
//	@Success		200		{object}	map[string]string	"message: Logged out successfully"
//	@Router			/api/v1/auth/logout [post]
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
//
//	@Summary		Revoke all sessions
//	@Description	Revokes every refresh token for the current user, logging out all devices.
//	@Tags			auth
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Success		200	{object}	map[string]string	"message: All sessions revoked successfully"
//	@Failure		401	{object}	map[string]string	"error: Unauthorized"
//	@Failure		500	{object}	map[string]string	"error: Failed to revoke sessions"
//	@Router			/api/v1/auth/revoke-all [post]
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
//
//	@Summary		Log in
//	@Description	Logs in with username and password and returns access and refresh tokens (the refresh token is also set as an httpOnly cookie). Users with MFA get {mfaRequired: true, sessionToken} instead; finish with POST /api/v1/auth/mfa/verify.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		signIn				true	"Credentials"
//	@Success		200		{object}	TokenResponse		"tokens (or mfaRequired + sessionToken)"
//	@Failure		400		{object}	map[string]string	"error: Invalid request"
//	@Failure		401		{object}	map[string]string	"error: Invalid credentials"
//	@Failure		403		{object}	map[string]string	"error: Password authentication is disabled on this instance"
//	@Failure		500		{object}	map[string]string	"error: Authentication failed | Failed to generate tokens"
//	@Router			/api/v1/auth/login [post]
func (h *AuthHandler) EnhancedLoginHandler(c *gin.Context) {
	logger := logging.FromContext(c)
	var loginReq struct {
		Username string `form:"username" json:"username" binding:"required"`
		Password string `form:"password" json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request: " + err.Error(),
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

	// Check if user has MFA enabled
	if userDetails.MFAEnabled {
		sessionToken, err := h.mfaService.GenerateSessionToken()
		if err != nil {
			logger.Errorw("Failed to generate MFA session token", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
			return
		}

		mfaSession := &uModel.MFASession{
			SessionToken: sessionToken,
			UserID:       userDetails.ID,
			AuthMethod:   "password",
			Verified:     false,
			CreatedAt:    time.Now().UTC(),
			ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
			UserData:     userDetails.Username,
		}

		if err := h.userRepo.CreateMFASession(c, mfaSession); err != nil {
			logger.Errorw("Failed to create MFA session", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"mfaRequired":  true,
			"sessionToken": sessionToken,
		})
		return
	}

	// Generate both access and refresh tokens
	tokenResponse, err := h.tokenService.GenerateTokens(c.Request.Context(), userDetails)
	if err != nil {
		logger.Errorw("Failed to generate tokens", "error", err)
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

	return user, nil
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
