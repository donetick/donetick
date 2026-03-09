package filter

import (
	"strconv"

	auth "donetick.com/core/internal/auth"
	fModel "donetick.com/core/internal/filter/model"
	fRepo "donetick.com/core/internal/filter/repo"
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

// getFilters godoc
//
//	@Summary		Get all filters
//	@Description	Gets all filters for the current user's circle
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{array}		model.Filter		"array of filters"
//	@Failure		500	{object}	map[string]string	"error: Error getting current user | Error getting filters"
//	@Router			/filters [get]
func (h *Handler) getFilters(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetCircleFilters(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting filters",
		})
		return
	}

	c.JSON(200, filters)
}

// getFilterByID godoc
//
//	@Summary		Get filter by ID
//	@Description	Gets a specific filter by ID
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Filter ID"
//	@Success		200	{object}	model.Filter		"filter object"
//	@Failure		400	{object}	map[string]string	"error: Filter ID is required | Invalid filter ID"
//	@Failure		404	{object}	map[string]string	"error: Filter not found"
//	@Failure		500	{object}	map[string]string	"error: Error getting current user"
//	@Router			/filters/{id} [get]
func (h *Handler) getFilterByID(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	filter, err := h.fRepo.GetFilterByID(c, filterID, currentUser.CircleID)
	if err != nil {
		c.JSON(404, gin.H{
			"error": "Filter not found",
		})
		return
	}

	c.JSON(200, filter)
}

// createFilter godoc
//
//	@Summary		Create a new filter
//	@Description	Creates a new filter for the current user's circle
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			filter	body		model.FilterReq			true	"Filter creation request"
//	@Success		200		{object}	map[string]model.Filter	"res: created filter object"
//	@Failure		400		{object}	map[string]string		"error: Error binding filter data | Filter name already exists"
//	@Failure		500		{object}	map[string]string		"error: Error getting current user | Error checking filter name | Error creating filter"
//	@Router			/filters [post]
func (h *Handler) createFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	var req fModel.FilterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Error binding filter data",
		})
		return
	}

	// Check if filter name already exists
	exists, err := h.fRepo.FilterNameExists(c, req.Name, currentUser.CircleID, nil)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error checking filter name",
		})
		return
	}
	if exists {
		c.JSON(400, gin.H{
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
		c.JSON(500, gin.H{
			"error": "Error creating filter",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": filter,
	})
}

// updateFilter godoc
//
//	@Summary		Update a filter
//	@Description	Updates an existing filter by ID
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int						true	"Filter ID"
//	@Param			filter	body		model.FilterReq			true	"Filter update request"
//	@Success		200		{object}	map[string]model.Filter	"res: updated filter object"
//	@Failure		400		{object}	map[string]string		"error: Filter ID is required | Invalid filter ID | Error binding filter data | Filter name already exists"
//	@Failure		500		{object}	map[string]string		"error: Error getting current user | Error checking filter name | Error getting updated filter | internal error"
//	@Router			/filters/{id} [put]
func (h *Handler) updateFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	var req fModel.FilterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Error binding filter data",
		})
		return
	}

	// Check if filter name already exists (excluding current filter)
	exists, err := h.fRepo.FilterNameExists(c, req.Name, currentUser.CircleID, &filterID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error checking filter name",
		})
		return
	}
	if exists {
		c.JSON(400, gin.H{
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
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Get updated filter to return
	updatedFilter, err := h.fRepo.GetFilterByID(c, filterID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting updated filter",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": updatedFilter,
	})
}

// deleteFilter godoc
//
//	@Summary		Delete a filter
//	@Description	Deletes a filter by ID
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Filter ID"
//	@Success		200	{object}	map[string]string	"res: Filter deleted successfully"
//	@Failure		400	{object}	map[string]string	"error: Filter ID is required | Invalid filter ID"
//	@Failure		500	{object}	map[string]string	"error: Error getting current user | internal error"
//	@Router			/filters/{id} [delete]
func (h *Handler) deleteFilter(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	if err := h.fRepo.DeleteFilter(c, filterID, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Filter deleted successfully",
	})
}

// toggleFilterPin godoc
//
//	@Summary		Toggle filter pin status
//	@Description	Toggles the pin status of a filter by ID
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Filter ID"
//	@Success		200	{object}	map[string]interface{}	"res: {isPinned: bool}"
//	@Failure		400	{object}	map[string]string		"error: Filter ID is required | Invalid filter ID"
//	@Failure		500	{object}	map[string]string		"error: Error getting current user | internal error"
//	@Router			/filters/{id}/toggle-pin [post]
func (h *Handler) toggleFilterPin(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filterIDRaw := c.Param("id")
	if filterIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Filter ID is required",
		})
		return
	}

	filterID, err := strconv.Atoi(filterIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid filter ID",
		})
		return
	}

	isPinned, err := h.fRepo.ToggleFilterPin(c, filterID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"res": gin.H{
			"isPinned": isPinned,
		},
	})
}

// getPinnedFilters godoc
//
//	@Summary		Get pinned filters
//	@Description	Gets all pinned filters for the current user's circle
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{array}		model.Filter		"array of pinned filters"
//	@Failure		500	{object}	map[string]string	"error: Error getting current user | Error getting pinned filters"
//	@Router			/filters/pinned [get]
func (h *Handler) getPinnedFilters(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetPinnedFilters(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting pinned filters",
		})
		return
	}

	c.JSON(200, filters)
}

// getFiltersByUsage godoc
//
//	@Summary		Get filters by usage
//	@Description	Gets filters sorted by usage for the current user's circle
//	@Tags			filters
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{array}		model.Filter		"array of filters sorted by usage"
//	@Failure		500	{object}	map[string]string	"error: Error getting current user | Error getting filters by usage"
//	@Router			/filters/by-usage [get]
func (h *Handler) getFiltersByUsage(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	filters, err := h.fRepo.GetFiltersByUsage(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting filters by usage",
		})
		return
	}

	c.JSON(200, filters)
}

// Routes sets up the filter routes
func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	filterRoutes := r.Group("api/v1/filters")
	filterRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
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
