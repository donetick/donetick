package label

import (
	"strconv"

	auth "donetick.com/core/internal/auth"
	lModel "donetick.com/core/internal/label/model"
	lRepo "donetick.com/core/internal/label/repo"
	"github.com/gin-gonic/gin"
)

type LabelReq struct {
	// Omitted on update preserves the current privacy setting.
	IsShared *bool  `json:"isShared"`
	Name     string `json:"name" binding:"required"`
	Color    string `json:"color"`
}

type UpdateLabelReq struct {
	ID int `json:"id" binding:"required"`
	LabelReq
}

type Handler struct {
	lRepo *lRepo.LabelRepository
}

func NewHandler(lRepo *lRepo.LabelRepository) *Handler {
	return &Handler{
		lRepo: lRepo,
	}
}

// getLabels godoc
//
//	@Summary		List labels
//	@Description	Returns the current user's own labels plus labels shared in their circle.
//	@Tags			labels
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{array}		lModel.Label		"labels"
//	@Failure		401	{object}	map[string]string	"error: User not authenticated"
//	@Failure		500	{object}	map[string]string	"error: Error getting labels"
//	@Router			/api/v1/labels [get]
func (h *Handler) getLabels(c *gin.Context) {
	// get current user:
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	labels, err := h.lRepo.GetUserLabels(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting labels",
		})
		return
	}
	c.JSON(200,
		labels,
	)
}

// createLabel godoc
//
//	@Summary	Create a label
//	@Tags		labels
//	@Accept		json
//	@Produce	json
//	@Security	JWTKeyAuth
//	@Security	APIKeyAuth
//	@Param		request	body		LabelReq				true	"Label"
//	@Success	200		{object}	map[string]lModel.Label	"res: created label"
//	@Failure	400		{object}	map[string]string		"error: Invalid request"
//	@Failure	401		{object}	map[string]string		"error: User not authenticated"
//	@Failure	500		{object}	map[string]string		"error: Error creating label"
//	@Router		/api/v1/labels [post]
func (h *Handler) createLabel(c *gin.Context) {
	// get current user:
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	var req LabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	label := &lModel.Label{
		Name:      req.Name,
		Color:     req.Color,
		CreatedBy: currentUser.ID,
		CircleID:  &currentUser.CircleID,
	}
	if req.IsShared != nil {
		label.IsShared = *req.IsShared
	}
	if err := h.lRepo.CreateLabels(c, []*lModel.Label{label}); err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating label",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": label,
	})
}

// updateLabel godoc
//
//	@Summary		Update a label
//	@Description	Updates a label's name and color. Omitting isShared keeps its current sharing setting.
//	@Tags			labels
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			request	body		UpdateLabelReq			true	"Label with ID"
//	@Success		200		{object}	map[string]lModel.Label	"res: updated label"
//	@Failure		400		{object}	map[string]string		"error: Invalid request"
//	@Failure		401		{object}	map[string]string		"error: User not authenticated"
//	@Failure		500		{object}	map[string]string		"error: Error updating label"
//	@Router			/api/v1/labels [put]
func (h *Handler) updateLabel(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	var req UpdateLabelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request: " + err.Error(),
		})
		return
	}

	label := &lModel.Label{
		Name:     req.Name,
		Color:    req.Color,
		ID:       req.ID,
		CircleID: &currentUser.CircleID,
	}
	if err := h.lRepo.UpdateLabel(c, currentUser.ID, label, req.IsShared); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating label",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": label,
	})
}

// deleteLabel godoc
//
//	@Summary		Delete a label
//	@Description	Removes the label from every chore, then deletes it.
//	@Tags			labels
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id	path		int					true	"Label ID"
//	@Success		200	{object}	map[string]string	"res: Label deleted"
//	@Failure		400	{object}	map[string]string	"error: Label ID is required | Invalid label ID"
//	@Failure		401	{object}	map[string]string	"error: User not authenticated"
//	@Failure		500	{object}	map[string]string	"error: Error unassociating label from chores"
//	@Router			/api/v1/labels/{id} [delete]
func (h *Handler) deleteLabel(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	// read label id from path:

	if !ok {
		c.JSON(401, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	labelIDRaw := c.Param("id")
	if labelIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Label ID is required",
		})
		return
	}

	labelID, err := strconv.Atoi(labelIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid label ID",
		})
		return
	}

	// unassociate label from all chores:
	if err := h.lRepo.DeassignLabelFromAllChoreAndDelete(c, currentUser.ID, labelID); err != nil {
		c.JSON(500, gin.H{
			"error": "Error unassociating label from chores",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Label deleted",
	})

}

func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {

	labelRoutes := r.Group("api/v1/labels")
	labelRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		labelRoutes.GET("", h.getLabels)
		labelRoutes.POST("", h.createLabel)
		labelRoutes.PUT("", h.updateLabel)
		labelRoutes.DELETE("/:id", h.deleteLabel)
	}

}
