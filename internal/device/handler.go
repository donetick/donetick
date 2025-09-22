package device

import (
	"net/http"
	"strconv"

	auth "donetick.com/core/internal/auth"
	dRepo "donetick.com/core/internal/device/repo"
	errorx "donetick.com/core/internal/error"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/ulule/limiter/v3"
)

type Handler struct {
	deviceRepo *dRepo.DeviceRepository
}

type RegisterDeviceTokenRequest struct {
	Token       string `json:"token" binding:"required"`
	DeviceID    string `json:"deviceId" binding:"required"`
	Platform    string `json:"platform" binding:"required,oneof=ios android"`
	AppVersion  string `json:"appVersion,omitempty"`
	DeviceModel string `json:"deviceModel,omitempty"`
}

type UnregisterDeviceTokenRequest struct {
	DeviceID string `json:"deviceId,omitempty"`
	Token    string `json:"token,omitempty"`
}

func NewHandler(dr *dRepo.DeviceRepository) *Handler {
	return &Handler{
		deviceRepo: dr,
	}
}

// RegisterDeviceToken registers a new FCM device token
func (h *Handler) RegisterDeviceToken(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error("Invalid request payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	deviceToken := &uModel.UserDeviceToken{
		UserID:      currentUser.ID,
		Token:       req.Token,
		DeviceID:    req.DeviceID,
		Platform:    req.Platform,
		AppVersion:  req.AppVersion,
		DeviceModel: req.DeviceModel,
	}

	if err := h.deviceRepo.RegisterDeviceToken(c, deviceToken); err != nil {
		log.Error("Failed to register device token", "error", err)

		// Check for device limit error
		if err == errorx.ErrDeviceLimitExceeded {
			c.JSON(http.StatusConflict, gin.H{
				"error": err.Error(),
				"code":  "DEVICE_LIMIT_EXCEEDED",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register device token"})
		return
	}

	log.Debugw("Device token registered successfully", "user_id", currentUser.ID, "device_id", req.DeviceID)
	c.JSON(http.StatusCreated, gin.H{
		"message":   "Device token registered successfully",
		"device_id": deviceToken.DeviceID,
		"id":        deviceToken.ID,
	})
}

// UnregisterDeviceToken removes a device token
func (h *Handler) UnregisterDeviceToken(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req UnregisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error("Invalid request payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Must provide either deviceId or token
	if req.DeviceID == "" && req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either deviceId or token must be provided"})
		return
	}

	var err error
	if req.DeviceID != "" {
		err = h.deviceRepo.UnregisterDeviceToken(c, currentUser.ID, req.DeviceID)
	} else {
		err = h.deviceRepo.UnregisterDeviceTokenByToken(c, currentUser.ID, req.Token)
	}

	if err != nil {
		log.Error("Failed to unregister device token", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister device token"})
		return
	}

	log.Info("Device token unregistered successfully", "user_id", currentUser.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Device token unregistered successfully"})
}

// GetDeviceTokens retrieves all device tokens for the current user
func (h *Handler) GetDeviceTokens(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Check if only active tokens are requested
	activeOnly := c.Query("active") == "true"

	var tokens []*uModel.UserDeviceToken
	var err error

	if activeOnly {
		tokens, err = h.deviceRepo.GetActiveDeviceTokens(c, currentUser.ID)
	} else {
		tokens, err = h.deviceRepo.GetUserDeviceTokens(c, currentUser.ID)
	}

	if err != nil {
		log.Error("Failed to retrieve device tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve device tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"res": tokens,
	})
}

// UpdateDeviceActivity updates the last active timestamp for a device
func (h *Handler) UpdateDeviceActivity(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Device ID is required"})
		return
	}

	if err := h.deviceRepo.UpdateDeviceTokenActivity(c, currentUser.ID, deviceID); err != nil {
		log.Error("Failed to update device activity", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update device activity"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device activity updated successfully"})
}

// GetDeviceCount returns the number of active devices for the current user
func (h *Handler) GetDeviceCount(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	count, err := h.deviceRepo.GetActiveDeviceCount(c, currentUser.ID)
	if err != nil {
		log.Error("Failed to get device count", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get device count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count": count,
		"limit": dRepo.MaxDevicesPerUser,
	})
}

// CleanupInactiveTokens removes tokens that haven't been active for specified days (admin only)
func (h *Handler) CleanupInactiveTokens(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// This could be an admin-only endpoint
	// For now, allowing any user to trigger cleanup for their own tokens would require additional logic

	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid days parameter"})
		return
	}

	if err := h.deviceRepo.CleanupInactiveTokens(c, days); err != nil {
		log.Error("Failed to cleanup inactive tokens", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup inactive tokens"})
		return
	}

	log.Info("Cleanup completed", "user_id", currentUser.ID, "days", days)
	c.JSON(http.StatusOK, gin.H{"message": "Cleanup completed successfully"})
}

// Routes sets up the device token management routes
func Routes(router *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {
	deviceRoutes := router.Group("api/v1/devices")
	deviceRoutes.Use(auth.MiddlewareFunc())
	{
		deviceRoutes.POST("/tokens", h.RegisterDeviceToken)
		deviceRoutes.DELETE("/tokens", h.UnregisterDeviceToken)
		deviceRoutes.GET("/tokens", h.GetDeviceTokens)
		deviceRoutes.GET("/count", h.GetDeviceCount)
		deviceRoutes.PUT("/tokens/:deviceId/activity", h.UpdateDeviceActivity)
	}
}
