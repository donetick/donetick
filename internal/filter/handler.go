package filter

import (
	"net/http"
	"strconv"

	auth "donetick.com/core/internal/auth"
	fModel "donetick.com/core/internal/filter/model"
	fRepo "donetick.com/core/internal/filter/repo"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	fRepo *fRepo.FilterRepository
}

func NewHandler(fRepo *fRepo.FilterRepository) *Handler {
	return &Handler{
		fRepo: fRepo,
	}
}

// getFilters gets all filters for the current user's circle
func (h *Handler) getFilters(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetCircleFilters(c, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting filters",
		})
		return
	}

	c.JSON(http.StatusOK, filters)
}

// getFilterByID gets a specific filter by ID
func (h *Handler) getFilterByID(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	filter, err := h.fRepo.GetFilterByID(c, filterID, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Filter not found",
		})
		return
	}

	c.JSON(http.StatusOK, filter)
}

// createFilter creates a new filter
func (h *Handler) createFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	var req fModel.FilterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Error binding filter data",
		})
		return
	}

	// Check if filter name already exists
	exists, err := h.fRepo.FilterNameExists(c, req.Name, currentUser.CircleID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error checking filter name",
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter name already exists",
		})
		return
	}

	// Set default operator if not provided
	if req.Operator == nil {
		defaultOp := fModel.LogicalOperatorAND
		req.Operator = &defaultOp
	}

	filter := &fModel.Filter{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		Conditions:  req.Conditions,
		Operator:    *req.Operator,
		CircleID:    currentUser.CircleID,
		CreatedBy:   currentUser.ID,
		IsPinned:    req.IsPinned,
	}

	if err := h.fRepo.CreateFilter(c, filter); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error creating filter",
		})
		return
	}

	c.JSON(http.StatusOK, filter)
}

// updateFilter updates an existing filter
func (h *Handler) updateFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	var req fModel.FilterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Error binding filter data",
		})
		return
	}

	// Check if filter name already exists (excluding current filter)
	exists, err := h.fRepo.FilterNameExists(c, req.Name, currentUser.CircleID, &filterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error checking filter name",
		})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter name already exists",
		})
		return
	}

	// Set default operator if not provided
	if req.Operator == nil {
		defaultOp := fModel.LogicalOperatorAND
		req.Operator = &defaultOp
	}
	filter := &fModel.Filter{
		ID:          filterID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
		Conditions:  req.Conditions,
		Operator:    *req.Operator,
		IsPinned:    req.IsPinned,
	}

	if err := h.fRepo.UpdateFilter(c, filter, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get updated filter to return
	updatedFilter, err := h.fRepo.GetFilterByID(c, filterID, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting updated filter",
		})
		return
	}

	c.JSON(http.StatusOK, updatedFilter)
}

// deleteFilter deletes a filter
func (h *Handler) deleteFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	if err := h.fRepo.DeleteFilter(c, filterID, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, "Filter deleted successfully")
}

// toggleFilterPin toggles the pin status of a filter
func (h *Handler) toggleFilterPin(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	isPinned, err := h.fRepo.ToggleFilterPin(c, filterID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"isPinned": isPinned,
	},
	)
}

// getPinnedFilters gets all pinned filters
func (h *Handler) getPinnedFilters(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetPinnedFilters(c, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting pinned filters",
		})
		return
	}

	c.JSON(http.StatusOK, filters)
}

// getFiltersByUsage gets filters sorted by usage
func (h *Handler) getFiltersByUsage(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetFiltersByUsage(c, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error getting filters by usage",
		})
		return
	}

	c.JSON(http.StatusOK, filters)
}

// Routes sets up the filter routes
func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {
	filterRoutes := r.Group("api/v1/filters")
	filterRoutes.Use(auth.MiddlewareFunc())
	{
		filterRoutes.GET("", h.getFilters)
		filterRoutes.GET("/pinned", h.getPinnedFilters)
		filterRoutes.GET("/by-usage", h.getFiltersByUsage)
		filterRoutes.GET("/:id", h.getFilterByID)
		filterRoutes.POST("", h.createFilter)
		filterRoutes.PUT("/:id", h.updateFilter)
		filterRoutes.DELETE("/:id", h.deleteFilter)
		filterRoutes.POST("/:id/toggle-pin", h.toggleFilterPin)
	}
}
