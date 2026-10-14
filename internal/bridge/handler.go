package bridge

import (
	"errors"
	"net/http"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/auth"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

// Handler exposes Core's own Bridge settings/status API, the surface
// Core's frontend calls instead of talking to Bridge directly.
// Every response here is built from
// InstanceStatus/sanitized errors only -- the stored instance token is
// never serialized by any handler in this file.
type Handler struct {
	svc        *Service
	circleRepo *cRepo.CircleRepository
}

func NewHandler(svc *Service, cr *cRepo.CircleRepository) *Handler {
	return &Handler{svc: svc, circleRepo: cr}
}

// requireCircleAdmin gates Bridge configuration behind the requesting
// user's circle-admin role -- only an administrator may configure Bridge,
// and Core has no separate instance-operator role today,
// so the circle admin/owner role is the closest existing concept, matching
// the gating pattern already used by internal/chore/handler.go for other
// circle-admin-only actions.
func (h *Handler) requireCircleAdmin(c *gin.Context) bool {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return false
	}
	role, err := h.circleRepo.GetUserCircleRole(c, currentUser.CircleID, currentUser.ID)
	if err != nil || role != cModel.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "only a circle administrator can manage Bridge settings"})
		return false
	}
	return true
}

func statusResponse(st *InstanceStatus) gin.H {
	return gin.H{
		"connected":          true,
		"instanceId":         st.InstanceID,
		"instanceName":       st.InstanceName,
		"enabled":            st.Enabled,
		"ownerEmailMasked":   st.OwnerEmailMasked,
		"ownerEmailVerified": st.OwnerEmailVerified,
		"plan":               st.PlanName,
		"subscriptionStatus": st.SubscriptionStatus,
		"usage": gin.H{
			"notificationsUsed":  st.NotificationsUsed,
			"notificationsLimit": st.NotificationsLimit,
			"periodStart":        st.PeriodStart,
			"periodEnd":          st.PeriodEnd,
			"activeDevices":      st.ActiveDevices,
			"deviceLimit":        st.DeviceLimit,
		},
	}
}

// errorStatusCode maps a bridge sentinel error to an HTTP status and a
// stable, frontend-matchable error code string -- never the raw error text.
func errorStatusCode(err error) (int, string) {
	switch {
	case errors.Is(err, ErrDisabled):
		return http.StatusOK, "DISABLED"
	case errors.Is(err, ErrUnauthorized):
		return http.StatusConflict, "REVOKED_TOKEN"
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests, "RATE_LIMITED"
	case errors.Is(err, ErrQuotaExceeded):
		return http.StatusForbidden, "QUOTA_EXCEEDED"
	case errors.Is(err, ErrDeviceLimitReached):
		return http.StatusForbidden, "DEVICE_LIMIT_REACHED"
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest, "VALIDATION_ERROR"
	default:
		return http.StatusInternalServerError, "INTERNAL"
	}
}

// GetStatus implements GET /api/v1/bridge/status.
func (h *Handler) GetStatus(c *gin.Context) {
	if !h.requireCircleAdmin(c) {
		return
	}
	st, err := h.svc.Status(c)
	if err != nil {
		code, errCode := errorStatusCode(err)
		if errors.Is(err, ErrDisabled) {
			c.JSON(code, gin.H{"connected": false, "code": errCode})
			return
		}
		logging.FromContext(c).Error("bridge_status_failed", "category", string(Classify(err)))
		c.JSON(code, gin.H{"connected": false, "code": errCode})
		return
	}
	c.JSON(http.StatusOK, statusResponse(st))
}

type connectRequest struct {
	BaseURL       string `json:"baseUrl" binding:"required"`
	InstanceID    string `json:"instanceId" binding:"required"`
	InstanceToken string `json:"instanceToken" binding:"required"`
}

// Connect implements POST /api/v1/bridge/connect. The instance token is
// read from the request body only long enough to validate and persist it;
// it is never logged (c.ShouldBindJSON's body is not logged by any
// middleware in this handler, and no log call below references req).
func (h *Handler) Connect(c *gin.Context) {
	if !h.requireCircleAdmin(c) {
		return
	}
	var req connectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	st, err := h.svc.Connect(c, ConnectInput{BaseURL: req.BaseURL, InstanceID: req.InstanceID, InstanceToken: req.InstanceToken})
	if err != nil {
		code, errCode := errorStatusCode(err)
		logging.FromContext(c).Warn("bridge_connect_failed", "category", string(Classify(err)))
		c.JSON(code, gin.H{"error": "unable to connect to Bridge", "code": errCode})
		return
	}
	c.JSON(http.StatusOK, statusResponse(st))
}

// Rotate implements POST /api/v1/bridge/rotate. Core cannot itself rotate
// an instance token (that is an owner-authenticated Bridge action) -- this
// endpoint re-validates and re-saves a new token the administrator obtained
// from Bridge's own UI, identical to Connect. Kept as a separate route so
// the frontend/UI language ("rotate credential") and confirmation-dialog
// flow can differ from initial connect even though the server-side effect
// is the same.
func (h *Handler) Rotate(c *gin.Context) {
	h.Connect(c)
}

// Disconnect implements POST /api/v1/bridge/disconnect.
func (h *Handler) Disconnect(c *gin.Context) {
	if !h.requireCircleAdmin(c) {
		return
	}
	if err := h.svc.Disconnect(c); err != nil {
		logging.FromContext(c).Error("bridge_disconnect_failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to disconnect Bridge"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": false})
}

// Routes registers Core's Bridge settings API under /api/v1/bridge. All
// routes require an authenticated Donetick session; role-gating to circle
// admins happens per-handler above (not in middleware) so GetStatus can
// later be relaxed to "any circle member can view" without duplicating the
// connect/disconnect checks, if desired.
func Routes(router *gin.Engine, h *Handler, authMw *jwt.GinJWTMiddleware, cfg *config.Config) {
	// if bridge not enabled then route should not be registered:
	if !cfg.Bridge.Enabled {
		return
	}

	g := router.Group("api/v1/bridge")
	g.Use(authMw.MiddlewareFunc())
	{
		g.GET("/status", h.GetStatus)
		g.POST("/connect", h.Connect)
		g.POST("/rotate", h.Rotate)
		g.POST("/disconnect", h.Disconnect)
	}
}
