package reward

import (
	"fmt"
	"strconv"

	auth "donetick.com/core/internal/auth"
	circle "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/events"
	"donetick.com/core/internal/notifier/dispatch"
	nps "donetick.com/core/internal/notifier/service"
	rModel "donetick.com/core/internal/reward/model"
	rRepo "donetick.com/core/internal/reward/repo"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

type RewardReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	PointsCost  int    `json:"pointsCost" binding:"required,gt=0"`
	IsActive    *bool  `json:"isActive"`
	AutoApprove *bool  `json:"autoApprove"`
}

type RedeemReq struct {
	Note   *string `json:"note"`
	UserID *int    `json:"userId"`
}

type ResolveReq struct {
	Note *string `json:"note"`
}

type Handler struct {
	rRepo         *rRepo.RewardRepository
	circleRepo    *cRepo.CircleRepository
	dispatcher    *dispatch.Dispatcher
	eventProducer *events.EventsProducer
}

func NewHandler(rRepo *rRepo.RewardRepository, circleRepo *cRepo.CircleRepository, dispatcher *dispatch.Dispatcher, eventProducer *events.EventsProducer) *Handler {
	return &Handler{
		rRepo:         rRepo,
		circleRepo:    circleRepo,
		dispatcher:    dispatcher,
		eventProducer: eventProducer,
	}
}

// notifyAdminsOfRedemptionRequest tells the admins and managers who can resolve
// a redemption that one is waiting on them. Delivery (devices + any legacy
// channel, once each, gated by the recipient's notification profile) is the
// dispatcher's job; who hears about it is decided here.
func (h *Handler) notifyAdminsOfRedemptionRequest(c *gin.Context, circleID int, requester *uModel.UserDetails, requesterName string, redemption *rModel.RewardRedemption) {
	circleUsers, err := h.circleRepo.GetCircleUsers(c, circleID)
	if err != nil {
		logging.FromContext(c).Error("Failed to fetch circle users for redemption notification", "error", err)
		return
	}

	approvers := redemptionApprovers(circleUsers, requester.ID)
	if len(approvers) == 0 {
		return
	}

	text := fmt.Sprintf("%s requested to redeem \"%s\" for %d points", requesterName, redemption.RewardName, redemption.PointsCost)
	h.dispatcher.NotifyUsers(c, dispatch.Request{
		EventType: nps.EventTypeOnRedemptionRequest,
		CircleID:  circleID,
		Subject:   redemption.RewardName,
		PushTitle: "Redemption Requested",
		PushBody:  text,
		PushData: map[string]string{
			"type":          "reward_redemption_request",
			"redemption_id": strconv.Itoa(redemption.ID),
			"reward_name":   redemption.RewardName,
			"from_user_id":  strconv.Itoa(requester.ID),
			"from_user":     requesterName,
		},
		Text:             text,
		IdempotencyScope: "redemption",
	}, approvers)
}

// redemptionApprovers picks who is told about a redemption waiting to be
// resolved. Rewards are circle-wide, so every active admin and manager can act
// on it -- except the requester, who does not need telling what they just did
// (the same exclusion chore.pendingApprovalRecipients makes for a completion).
func redemptionApprovers(circleUsers []*circle.UserCircleDetail, requesterID int) []*circle.UserCircleDetail {
	approvers := make([]*circle.UserCircleDetail, 0, len(circleUsers))
	for _, member := range circleUsers {
		if member.UserID == requesterID || !member.IsActive {
			continue
		}
		if member.Role != circle.UserRoleAdmin && member.Role != circle.UserRoleManager {
			continue
		}
		approvers = append(approvers, member)
	}
	return approvers
}

func (h *Handler) isAdminOrManager(c *gin.Context, userID int, circleID int) (bool, error) {
	circleUsers, err := h.circleRepo.GetCircleUsers(c, circleID)
	if err != nil {
		return false, err
	}
	for _, u := range circleUsers {
		if u.UserID == userID && (u.Role == circle.UserRoleAdmin || u.Role == circle.UserRoleManager) {
			return true, nil
		}
	}
	return false, nil
}

// getRewards godoc
//
//	@Summary		List rewards
//	@Description	Returns every reward in the current user's circle.
//	@Tags			rewards
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{array}		rModel.Reward		"rewards"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		500	{object}	map[string]string	"error: Error getting rewards"
//	@Router			/api/v1/rewards [get]
func (h *Handler) getRewards(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}
	rewards, err := h.rRepo.GetRewardsByCircle(c, currentUser.CircleID)
	if err != nil {
		logging.FromContext(c).Error("Failed to fetch rewards", "error", err)
		c.JSON(500, gin.H{"error": "Error getting rewards"})
		return
	}
	c.JSON(200, rewards)
}

// createReward godoc
//
//	@Summary		Create a reward
//	@Description	Admins and managers only.
//	@Tags			rewards
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			request	body		RewardReq					true	"Reward"
//	@Success		200		{object}	map[string]rModel.Reward	"res: created reward"
//	@Failure		400		{object}	map[string]string			"error: Invalid request"
//	@Failure		401		{object}	map[string]string			"error: Authentication failed"
//	@Failure		403		{object}	map[string]string			"error: Only admins or managers can create rewards"
//	@Failure		500		{object}	map[string]string			"error: Error creating reward"
//	@Router			/api/v1/rewards [post]
func (h *Handler) createReward(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "Only admins or managers can create rewards"})
		return
	}

	var req RewardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	autoApprove := false
	if req.AutoApprove != nil {
		autoApprove = *req.AutoApprove
	}

	reward := &rModel.Reward{
		CircleID:    currentUser.CircleID,
		Name:        req.Name,
		Description: req.Description,
		PointsCost:  req.PointsCost,
		IsActive:    isActive,
		AutoApprove: autoApprove,
		CreatedBy:   currentUser.ID,
	}
	if err := h.rRepo.CreateReward(c, reward); err != nil {
		c.JSON(500, gin.H{"error": "Error creating reward"})
		return
	}
	c.JSON(200, gin.H{"res": reward})
}

// updateReward godoc
//
//	@Summary		Update a reward
//	@Description	Admins and managers only.
//	@Tags			rewards
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int							true	"Reward ID"
//	@Param			request	body		RewardReq					true	"Reward"
//	@Success		200		{object}	map[string]rModel.Reward	"res: updated reward"
//	@Failure		400		{object}	map[string]string			"error: Invalid reward ID | Invalid request"
//	@Failure		401		{object}	map[string]string			"error: Authentication failed"
//	@Failure		403		{object}	map[string]string			"error: Only admins or managers can update rewards"
//	@Failure		500		{object}	map[string]string			"error: Error updating reward"
//	@Router			/api/v1/rewards/{id} [put]
func (h *Handler) updateReward(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "Only admins or managers can update rewards"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	var req RewardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	reward := &rModel.Reward{
		ID:          id,
		CircleID:    currentUser.CircleID,
		Name:        req.Name,
		Description: req.Description,
		PointsCost:  req.PointsCost,
		IsActive:    isActive,
	}
	if err := h.rRepo.UpdateReward(c, reward, req.AutoApprove); err != nil {
		c.JSON(500, gin.H{"error": "Error updating reward"})
		return
	}
	updatedReward, err := h.rRepo.GetReward(c, id, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error retrieving updated reward"})
		return
	}
	c.JSON(200, gin.H{"res": updatedReward})
}

// deleteReward godoc
//
//	@Summary		Delete a reward
//	@Description	Admins and managers only.
//	@Tags			rewards
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id	path		int					true	"Reward ID"
//	@Success		200	{object}	map[string]string	"res: Reward deleted"
//	@Failure		400	{object}	map[string]string	"error: Invalid reward ID"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only admins or managers can delete rewards"
//	@Failure		500	{object}	map[string]string	"error: Error deleting reward"
//	@Router			/api/v1/rewards/{id} [delete]
func (h *Handler) deleteReward(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "Only admins or managers can delete rewards"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	if err := h.rRepo.DeleteReward(c, id, currentUser.CircleID); err != nil {
		c.JSON(500, gin.H{"error": "Error deleting reward"})
		return
	}
	c.JSON(200, gin.H{"res": "Reward deleted"})
}

// redeemReward godoc
//
//	@Summary		Redeem a reward
//	@Description	Spends points on a reward. Auto-approve rewards are approved immediately; otherwise the redemption stays pending and admins are notified. Admins and managers may set userId to redeem for another member. The body is optional.
//	@Tags			rewards
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int									true	"Reward ID"
//	@Param			request	body		RedeemReq							false	"Optional note and member"
//	@Success		200		{object}	map[string]rModel.RewardRedemption	"res: redemption"
//	@Failure		400		{object}	map[string]string					"error: Invalid reward ID | Selected user is not an active member of this circle | Not enough points to redeem this reward"
//	@Failure		401		{object}	map[string]string					"error: Authentication failed"
//	@Failure		403		{object}	map[string]string					"error: Only admins or managers can request a reward for another member"
//	@Failure		500		{object}	map[string]string					"error: Error requesting redemption"
//	@Router			/api/v1/rewards/{id}/redeem [post]
func (h *Handler) redeemReward(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid reward ID"})
		return
	}

	var req RedeemReq
	// body is optional (note is optional), so ignore binding errors on an empty body
	_ = c.ShouldBindJSON(&req)
	claimant := currentUser
	if req.UserID != nil && *req.UserID != currentUser.ID {
		isAdmin, roleErr := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
		if roleErr != nil {
			c.JSON(500, gin.H{"error": "Error checking permissions"})
			return
		}
		if !isAdmin {
			c.JSON(403, gin.H{"error": "Only admins or managers can request a reward for another member"})
			return
		}
		members, membersErr := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
		if membersErr != nil {
			c.JSON(500, gin.H{"error": "Error validating circle member"})
			return
		}
		found := false
		for _, member := range members {
			if member.UserID == *req.UserID && member.IsActive {
				found = true
				break
			}
		}
		if !found {
			c.JSON(400, gin.H{"error": "Selected user is not an active member of this circle"})
			return
		}
		claimant = &uModel.UserDetails{User: uModel.User{ID: *req.UserID, CircleID: currentUser.CircleID}}
		for _, member := range members {
			if member.UserID == *req.UserID {
				claimant.Username = member.Username
				claimant.DisplayName = member.DisplayName
				break
			}
		}
	}

	redemption, err := h.rRepo.RequestRedemption(c, id, claimant.ID, currentUser.CircleID, req.Note)
	if err != nil {
		if err == rRepo.ErrInsufficientPoints {
			c.JSON(400, gin.H{"error": "Not enough points to redeem this reward"})
			return
		}
		logging.FromContext(c).Error("Failed to request redemption", "error", err)
		c.JSON(500, gin.H{"error": "Error requesting redemption"})
		return
	}

	requesterName := claimant.DisplayName
	if requesterName == "" {
		requesterName = claimant.Username
	}
	if redemption.Status == rModel.RedemptionStatusApproved {
		if h.eventProducer != nil {
			h.eventProducer.RewardRedeemApproved(c, currentUser.WebhookURL, events.RewardRedemptionData{
				RedemptionID:    redemption.ID,
				RewardID:        redemption.RewardID,
				RecipientUserID: redemption.UserID,
			})
		}
	} else {
		h.notifyAdminsOfRedemptionRequest(c, currentUser.CircleID, claimant, requesterName, redemption)
	}

	c.JSON(200, gin.H{"res": redemption})
}

// getRedemptions godoc
//
//	@Summary		List reward redemptions
//	@Description	Admins and managers get the whole circle's redemptions, optionally filtered by status. Everyone else gets only their own, and the filter is ignored.
//	@Tags			rewards
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			status	query		string					false	"Filter (admins/managers only)"	Enums(pending, approved, rejected)
//	@Success		200		{array}		rModel.RewardRedemption	"redemptions"
//	@Failure		401		{object}	map[string]string		"error: Authentication failed"
//	@Failure		500		{object}	map[string]string		"error: Error getting redemptions"
//	@Router			/api/v1/rewards/redemptions [get]
func (h *Handler) getRedemptions(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}

	// Non-admins only ever see their own redemption history.
	if !isAdmin {
		redemptions, err := h.rRepo.GetRedemptionsByUser(c, currentUser.ID, currentUser.CircleID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error getting redemptions"})
			return
		}
		c.JSON(200, redemptions)
		return
	}

	var status *rModel.RedemptionStatus
	switch c.Query("status") {
	case "pending":
		s := rModel.RedemptionStatusPending
		status = &s
	case "approved":
		s := rModel.RedemptionStatusApproved
		status = &s
	case "rejected":
		s := rModel.RedemptionStatusRejected
		status = &s
	}

	redemptions, err := h.rRepo.GetRedemptionsByCircle(c, currentUser.CircleID, status)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting redemptions"})
		return
	}
	c.JSON(200, redemptions)
}

// approveRedemption godoc
//
//	@Summary		Approve a redemption
//	@Description	Admins and managers only. Deducts the points from the member.
//	@Tags			rewards
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id	path		int					true	"Redemption ID"
//	@Success		200	{object}	map[string]string	"message: Redemption approved"
//	@Failure		400	{object}	map[string]string	"error: Invalid redemption ID | User no longer has enough points for this redemption | Redemption is not pending"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: Only admins or managers can approve redemptions"
//	@Failure		500	{object}	map[string]string	"error: Error approving redemption"
//	@Router			/api/v1/rewards/redemptions/{id}/approve [post]
func (h *Handler) approveRedemption(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "Only admins or managers can approve redemptions"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid redemption ID"})
		return
	}

	redemption, err := h.rRepo.ApproveRedemption(c, id, currentUser.CircleID, currentUser.ID)
	if err != nil {
		if err == rRepo.ErrInsufficientPoints {
			c.JSON(400, gin.H{"error": "User no longer has enough points for this redemption"})
			return
		}
		if err == rRepo.ErrRedemptionNotPending {
			c.JSON(400, gin.H{"error": "Redemption is not pending"})
			return
		}
		logging.FromContext(c).Error("Failed to approve redemption", "error", err)
		c.JSON(500, gin.H{"error": "Error approving redemption"})
		return
	}
	if h.eventProducer != nil {
		h.eventProducer.RewardRedeemApproved(c, currentUser.WebhookURL, events.RewardRedemptionData{
			RedemptionID:    redemption.ID,
			RewardID:        redemption.RewardID,
			RecipientUserID: redemption.UserID,
		})
	}
	c.JSON(200, gin.H{"message": "Redemption approved"})
}

// rejectRedemption godoc
//
//	@Summary		Reject a redemption
//	@Description	Admins and managers only. The body is optional.
//	@Tags			rewards
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int					true	"Redemption ID"
//	@Param			request	body		ResolveReq			false	"Optional note"
//	@Success		200		{object}	map[string]string	"message: Redemption rejected"
//	@Failure		400		{object}	map[string]string	"error: Invalid redemption ID | Redemption is not pending"
//	@Failure		401		{object}	map[string]string	"error: Authentication failed"
//	@Failure		403		{object}	map[string]string	"error: Only admins or managers can reject redemptions"
//	@Failure		500		{object}	map[string]string	"error: Error rejecting redemption"
//	@Router			/api/v1/rewards/redemptions/{id}/reject [post]
func (h *Handler) rejectRedemption(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return
	}

	isAdmin, err := h.isAdminOrManager(c, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error checking permissions"})
		return
	}
	if !isAdmin {
		c.JSON(403, gin.H{"error": "Only admins or managers can reject redemptions"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid redemption ID"})
		return
	}

	var req ResolveReq
	_ = c.ShouldBindJSON(&req)

	if err := h.rRepo.RejectRedemption(c, id, currentUser.CircleID, currentUser.ID, req.Note); err != nil {
		if err == rRepo.ErrRedemptionNotPending {
			c.JSON(400, gin.H{"error": "Redemption is not pending"})
			return
		}
		logging.FromContext(c).Error("Failed to reject redemption", "error", err)
		c.JSON(500, gin.H{"error": "Error rejecting redemption"})
		return
	}
	c.JSON(200, gin.H{"message": "Redemption rejected"})
}

func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	rewardRoutes := r.Group("api/v1/rewards")
	rewardRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		rewardRoutes.GET("", h.getRewards)
		rewardRoutes.POST("", h.createReward)
		rewardRoutes.PUT("/:id", h.updateReward)
		rewardRoutes.DELETE("/:id", h.deleteReward)
		rewardRoutes.POST("/:id/redeem", h.redeemReward)

		rewardRoutes.GET("/redemptions", h.getRedemptions)
		rewardRoutes.POST("/redemptions/:id/approve", h.approveRedemption)
		rewardRoutes.POST("/redemptions/:id/reject", h.rejectRedemption)
	}
}
