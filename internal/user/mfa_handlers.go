package user

import (
	"net/http"

	auth "donetick.com/core/internal/auth"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

// setupMFA initiates MFA setup for the current user
//
//	@Summary		Start MFA setup
//	@Description	Generates a TOTP secret, QR code URL and backup codes. MFA is not enabled until POST /api/v1/users/mfa/confirm.
//	@Tags			mfa
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	uModel.MFASetupResponse	"secret, QR code URL, backup codes"
//	@Failure		400	{object}	map[string]string		"error: MFA is already enabled"
//	@Failure		500	{object}	map[string]string		"error: Failed to generate MFA secret | Failed to generate backup codes"
//	@Router			/api/v1/users/mfa/setup [post]
func (h *Handler) setupMFA(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	// Check if MFA is already enabled
	if currentUser.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA is already enabled"})
		return
	}

	// Generate TOTP secret
	key, err := h.mfaService.GenerateSecret(currentUser.Email)
	if err != nil {
		logger.Errorw("Failed to generate MFA secret", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate MFA secret"})
		return
	}

	// Generate backup codes
	backupCodes, err := h.mfaService.GenerateBackupCodes(8)
	if err != nil {
		logger.Errorw("Failed to generate backup codes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate backup codes"})
		return
	}

	response := uModel.MFASetupResponse{
		Secret:      key.Secret(),
		QRCodeURL:   key.URL(),
		BackupCodes: backupCodes,
	}

	c.JSON(http.StatusOK, response)
}

// confirmMFA confirms and enables MFA for the current user
//
//	@Summary		Enable MFA
//	@Description	Enables MFA using the secret and backup codes from setup and a current TOTP code.
//	@Tags			mfa
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			request	body		user.confirmMFA.ConfirmMFARequest	true	"Secret, code and backup codes"
//	@Success		200		{object}	map[string]string					"message: MFA enabled successfully"
//	@Failure		400		{object}	map[string]string					"error: MFA is already enabled | Invalid request | Invalid verification code"
//	@Failure		500		{object}	map[string]string					"error: Failed to enable MFA"
//	@Router			/api/v1/users/mfa/confirm [post]
func (h *Handler) confirmMFA(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	// Check if MFA is already enabled
	if currentUser.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA is already enabled"})
		return
	}

	type ConfirmMFARequest struct {
		Secret      string   `json:"secret" binding:"required"`
		Code        string   `json:"code" binding:"required"`
		BackupCodes []string `json:"backupCodes" binding:"required"`
	}

	var req ConfirmMFARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Verify the TOTP code
	if !h.mfaService.VerifyTOTP(req.Secret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	// Enable MFA in database
	if err := h.userRepo.EnableMFA(c, currentUser.ID, req.Secret, req.BackupCodes); err != nil {
		logger.Errorw("Failed to enable MFA", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable MFA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MFA enabled successfully"})
}

// disableMFA disables MFA for the current user
//
//	@Summary		Disable MFA
//	@Description	Requires a current TOTP code or a backup code.
//	@Tags			mfa
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			request	body		uModel.MFAVerifyRequest	true	"Verification code"
//	@Success		200		{object}	map[string]string		"message: MFA disabled successfully"
//	@Failure		400		{object}	map[string]string		"error: MFA is not enabled | Invalid request | Invalid verification code"
//	@Failure		500		{object}	map[string]string		"error: Failed to validate code | Failed to disable MFA"
//	@Router			/api/v1/users/mfa/disable [post]
func (h *Handler) disableMFA(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	// Check if MFA is enabled
	if !currentUser.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA is not enabled"})
		return
	}

	var req uModel.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Verify the code before disabling
	valid, newUsedCodes, err := h.mfaService.IsCodeValid(
		currentUser.MFASecret,
		currentUser.MFABackupCodes,
		currentUser.MFARecoveryUsed,
		req.Code,
	)

	if err != nil {
		logger.Errorw("Error validating MFA code", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate code"})
		return
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	// Update used codes if a backup code was used
	if newUsedCodes != currentUser.MFARecoveryUsed {
		if err := h.userRepo.UpdateMFARecoveryCodes(c, currentUser.ID, newUsedCodes); err != nil {
			logger.Errorw("Failed to update recovery codes", "error", err)
		}
	}

	// Disable MFA in database
	if err := h.userRepo.DisableMFA(c, currentUser.ID); err != nil {
		logger.Errorw("Failed to disable MFA", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable MFA"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MFA disabled successfully"})
}

// verifyMFA verifies MFA code during login process
//
//	@Summary		Finish an MFA login
//	@Description	Completes a login that returned mfaRequired, using its sessionToken and a TOTP or backup code.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		uModel.MFAVerifyRequest	true	"Session token and code"
//	@Success		200		{object}	auth.TokenResponse		"tokens"
//	@Failure		400		{object}	map[string]string		"error: Invalid request | Invalid or expired session | Invalid verification code"
//	@Failure		500		{object}	map[string]string		"error: Failed to verify user | Unable to Generate a Token"
//	@Router			/api/v1/auth/mfa/verify [post]
func (h *Handler) verifyMFA(c *gin.Context) {
	logger := logging.FromContext(c)

	var req uModel.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get MFA session
	mfaSession, err := h.userRepo.GetMFASession(c, req.SessionToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired session"})
		return
	}

	// Get user details
	user, err := h.userRepo.GetUserByUsername(c, mfaSession.UserData) // Assuming UserData contains username
	if err != nil {
		logger.Errorw("Failed to get user for MFA verification", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
		return
	}

	// Verify the MFA code
	valid, newUsedCodes, err := h.mfaService.IsCodeValid(
		user.MFASecret,
		user.MFABackupCodes,
		user.MFARecoveryUsed,
		req.Code,
	)

	if err != nil {
		logger.Errorw("Error validating MFA code", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate code"})
		return
	}

	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	// Update used codes if a backup code was used
	if newUsedCodes != user.MFARecoveryUsed {
		if err := h.userRepo.UpdateMFARecoveryCodes(c, user.ID, newUsedCodes); err != nil {
			logger.Errorw("Failed to update recovery codes", "error", err)
		}
	}

	// Mark session as verified
	mfaSession.Verified = true
	if err := h.userRepo.UpdateMFASession(c, mfaSession); err != nil {
		logger.Errorw("Failed to update MFA session", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete verification"})
		return
	}

	// Clean up MFA session
	h.userRepo.DeleteMFASession(c, req.SessionToken)

	// Generate tokens including refresh token
	tokenResponse, err := h.tokenService.GenerateTokens(c.Request.Context(), user)
	if err != nil {
		logger.Errorw("Unable to generate tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to Generate a Token"})
		return
	}
	c.SetCookie("refresh_token", tokenResponse.RefreshToken, int(h.tokenService.RefreshTokenExpiry().Seconds()), "/", "", true, true)
	c.JSON(http.StatusOK, tokenResponse)
}

// getMFAStatus returns the current MFA status for the user
//
//	@Summary	Get my MFA status
//	@Tags		mfa
//	@Produce	json
//	@Security	JWTKeyAuth
//	@Security	APIKeyAuth
//	@Success	200	{object}	map[string]bool		"mfaEnabled"
//	@Failure	500	{object}	map[string]string	"error: Failed to get current user"
//	@Router		/api/v1/users/mfa/status [get]
func (h *Handler) getMFAStatus(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mfaEnabled": currentUser.MFAEnabled,
	})
}

// regenerateBackupCodes generates new backup codes for the user
func (h *Handler) regenerateBackupCodes(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	// Check if MFA is enabled
	if !currentUser.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MFA is not enabled"})
		return
	}

	var req uModel.MFAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Verify the current TOTP code
	if !h.mfaService.VerifyTOTP(currentUser.MFASecret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
		return
	}

	// Generate new backup codes
	newBackupCodes, err := h.mfaService.GenerateBackupCodes(8)
	if err != nil {
		logger.Errorw("Failed to generate backup codes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate backup codes"})
		return
	}

	// Update backup codes in database
	if err := h.userRepo.EnableMFA(c, currentUser.ID, currentUser.MFASecret, newBackupCodes); err != nil {
		logger.Errorw("Failed to update backup codes", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update backup codes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"backupCodes": newBackupCodes,
	})
}
