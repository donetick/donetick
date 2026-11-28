package user

import (
	"donetick.com/core/internal/auth"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// UpdateAPIToken lets an authenticated session migrate a legacy token without
// rotating its secret. The API-token route policy never permits this endpoint.
func (h *Handler) UpdateAPIToken(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req struct {
		Scopes    []string   `json:"scopes" binding:"required"`
		ExpiresAt *time.Time `json:"expiresAt" binding:"required"`
		CircleID  *int       `json:"circleId"`
		MFACode   string     `json:"mfaCode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !auth.ValidAPITokenScopes(req.Scopes) || req.ExpiresAt == nil || !req.ExpiresAt.After(time.Now().UTC()) || (req.CircleID != nil && *req.CircleID != user.CircleID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Explicit valid scopes, a future expiresAt, and an accessible circleId are required"})
		return
	}
	if user.MFAEnabled {
		if req.MFACode == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "MFA code required"})
			return
		}
		valid, used, err := h.mfaService.IsCodeValid(user.MFASecret, user.MFABackupCodes, user.MFARecoveryUsed, req.MFACode)
		if err != nil || !valid {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid MFA code"})
			return
		}
		if used != user.MFARecoveryUsed {
			if err := h.userRepo.UpdateMFARecoveryCodes(c, user.ID, used); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
		}
	}
	if err := h.userRepo.UpdateAPIToken(c.Request.Context(), user.ID, c.Param("id"), uRepo.APITokenOptions{Scopes: req.Scopes, ExpiresAt: req.ExpiresAt, CircleID: req.CircleID}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to update token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
