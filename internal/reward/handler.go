package reward

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	auth "donetick.com/core/internal/auth"
	circle "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/notifier/dispatch"
	nps "donetick.com/core/internal/notifier/service"
	xpRepo "donetick.com/core/internal/points/repo"
	rModel "donetick.com/core/internal/reward/model"
	rRepo "donetick.com/core/internal/reward/repo"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type RewardReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	PointsCost  int    `json:"pointsCost" binding:"required,gt=0,lte=2147483647"`
	IsActive    *bool  `json:"isActive"`
}

type RedeemReq struct {
	Note       *string `json:"note"`
	CategoryID *int    `json:"categoryId"`
}

type ResolveReq struct {
	Note *string `json:"note"`
}

type Handler struct {
	rRepo      *rRepo.RewardRepository
	circleRepo *cRepo.CircleRepository
	dispatcher *dispatch.Dispatcher
}

func NewHandler(rRepo *rRepo.RewardRepository, circleRepo *cRepo.CircleRepository, dispatcher *dispatch.Dispatcher) *Handler {
	return &Handler{
		rRepo:      rRepo,
		circleRepo: circleRepo,
		dispatcher: dispatcher,
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

	reward := &rModel.Reward{
		CircleID:    currentUser.CircleID,
		Name:        req.Name,
		Description: req.Description,
		PointsCost:  req.PointsCost,
		IsActive:    isActive,
		CreatedBy:   currentUser.ID,
	}
	if err := h.rRepo.CreateReward(c, reward); err != nil {
		c.JSON(500, gin.H{"error": "Error creating reward"})
		return
	}
	c.JSON(200, gin.H{"res": reward})
}

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
	if err := h.rRepo.UpdateReward(c, reward); err != nil {
		c.JSON(500, gin.H{"error": "Error updating reward"})
		return
	}
	c.JSON(200, gin.H{"res": reward})
}

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
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(400, gin.H{"error": "Invalid redemption request"})
		return
	}

	redemption, err := h.rRepo.RequestRedemption(c, id, currentUser.ID, currentUser.CircleID, req.Note, req.CategoryID)
	if err != nil {
		if errors.Is(err, xpRepo.ErrCategoryRequired) || errors.Is(err, xpRepo.ErrInvalidCategory) || errors.Is(err, xpRepo.ErrInvalidXP) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err == rRepo.ErrInsufficientPoints {
			c.JSON(400, gin.H{"error": "Not enough points to redeem this reward"})
			return
		}
		logging.FromContext(c).Error("Failed to request redemption", "error", err)
		c.JSON(500, gin.H{"error": "Error requesting redemption"})
		return
	}

	requesterName := currentUser.DisplayName
	if requesterName == "" {
		requesterName = currentUser.Username
	}
	h.notifyAdminsOfRedemptionRequest(c, currentUser.CircleID, currentUser, requesterName, redemption)

	c.JSON(200, gin.H{"res": redemption})
}

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

	if err := h.rRepo.ApproveRedemption(c, id, currentUser.CircleID, currentUser.ID); err != nil {
		if errors.Is(err, xpRepo.ErrCategoryRequired) || errors.Is(err, xpRepo.ErrInvalidCategory) || errors.Is(err, xpRepo.ErrInsufficientXP) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, xpRepo.ErrMembershipChanged) || errors.Is(err, xpRepo.ErrBalanceLimit) {
			c.JSON(409, gin.H{"error": err.Error()})
			return
		}
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
	c.JSON(200, gin.H{"message": "Redemption approved"})
}

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

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {
	rewardRoutes := r.Group("api/v1/rewards")
	rewardRoutes.Use(auth.MiddlewareFunc())
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
