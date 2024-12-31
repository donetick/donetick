package circle

import (
	"log"

	"strconv"
	"time"

	auth "donetick.com/core/internal/authorization"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	circleRepo *cRepo.CircleRepository
	userRepo   *uRepo.UserRepository
	choreRepo  *chRepo.ChoreRepository
}

func NewHandler(cr *cRepo.CircleRepository, ur *uRepo.UserRepository, c *chRepo.ChoreRepository) *Handler {
	return &Handler{
		circleRepo: cr,
		userRepo:   ur,
		choreRepo:  c,
	}
}

func (h *Handler) GetCircleMembers(c *gin.Context) {
	// Get the circle ID from the JWT
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		log.Error("Error getting current user")
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	// Get all the members of the circle
	members, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle members",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": members,
	})
}

func (h *Handler) JoinCircle(c *gin.Context) {
	// Get the circle ID from the JWT
	log := logging.FromContext(c)
	log.Debug("handlder.go: JoinCircle")
	currentUser, ok := auth.CurrentUser(c)

	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	requestedCircleID := c.Query("invite_code")
	if requestedCircleID == "" {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	circle, err := h.circleRepo.GetCircleByInviteCode(c, requestedCircleID)

	if circle.ID == currentUser.CircleID {
		c.JSON(409, gin.H{
			"error": "You are already a member of this circle",
		})
		return
	}

	// Add the user to the circle
	err = h.circleRepo.AddUserToCircle(c, &cModel.UserCircle{
		CircleID: circle.ID,
		UserID:   currentUser.ID,
		Role:     "member",
		IsActive: false,
	})

	if err != nil {
		log.Error("Error adding user to circle:", err)
		c.JSON(500, gin.H{
			"error": "Error adding user to circle",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "User Requested to join circle successfully",
	})
}

func (h *Handler) LeaveCircle(c *gin.Context) {
	log := logging.FromContext(c)
	log.Debug("handler.go: LeaveCircle")
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	rawCircleID := c.Query("circle_id")
	circleID, err := strconv.Atoi(rawCircleID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	orginalCircleID, err := h.circleRepo.GetUserOriginalCircle(c, currentUser.ID)
	if err != nil {
		log.Error("Error getting user original circle:", err)
		c.JSON(500, gin.H{
			"error": "Error getting user original circle",
		})
		return
	}

	// START : HANDLE USER LEAVING CIRCLE
	// bulk update chores:
	if err := handleUserLeavingCircle(h, c, currentUser, orginalCircleID); err != nil {
		log.Error("Error handling user leaving circle:", err)
		c.JSON(500, gin.H{
			"error": "Error handling user leaving circle",
		})
		return
	}

	// END: HANDLE USER LEAVING CIRCLE

	err = h.circleRepo.LeaveCircleByUserID(c, circleID, currentUser.ID)
	if err != nil {
		log.Error("Error leaving circle:", err)
		c.JSON(500, gin.H{
			"error": "Error leaving circle",
		})
		return
	}

	if err := h.userRepo.UpdateUserCircle(c, currentUser.ID, orginalCircleID); err != nil {
		log.Error("Error updating user circle:", err)
		c.JSON(500, gin.H{
			"error": "Error updating user circle",
		})
		return
	}
	c.JSON(200, gin.H{
		"res": "User left circle successfully",
	})
}

func handleUserLeavingCircle(h *Handler, c *gin.Context, leavingUser *uModel.User, orginalCircleID int) error {
	userAssignedCircleChores, err := h.choreRepo.GetChores(c, leavingUser.CircleID, leavingUser.ID, true)
	if err != nil {
		return err
	}
	for _, ch := range userAssignedCircleChores {

		if ch.CreatedBy == leavingUser.ID && ch.AssignedTo != leavingUser.ID {
			ch.AssignedTo = leavingUser.ID
			ch.UpdatedAt = time.Now()
			ch.UpdatedBy = leavingUser.ID
			ch.CircleID = orginalCircleID
		} else if ch.CreatedBy != leavingUser.ID && ch.AssignedTo == leavingUser.ID {
			chore.RemoveAssigneeAndReassign(ch, leavingUser.ID)
		}

	}

	h.choreRepo.UpdateChores(c, userAssignedCircleChores)
	h.choreRepo.RemoveChoreAssigneeByCircleID(c, leavingUser.ID, leavingUser.CircleID)
	h.circleRepo.AssignDefaultCircle(c, leavingUser.ID)
	return nil
}

func (h *Handler) DeleteCircleMember(c *gin.Context) {
	log := logging.FromContext(c)
	log.Debug("handler.go: DeleteCircleMember")
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	rawCircleID := c.Param("id")
	circleID, err := strconv.Atoi(rawCircleID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	rawMemeberIDToDeleted := c.Query("member_id")
	memberIDToDeleted, err := strconv.Atoi(rawMemeberIDToDeleted)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	admins, err := h.circleRepo.GetCircleAdmins(c, circleID)
	if err != nil {
		log.Error("Error getting circle admins:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle admins",
		})
		return
	}
	isAdmin := false
	for _, admin := range admins {
		if admin.UserID == currentUser.ID {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "You are not an admin of this circle",
		})
		return
	}
	orginalCircleID, err := h.circleRepo.GetUserOriginalCircle(c, memberIDToDeleted)
	if handleUserLeavingCircle(h, c, &uModel.User{ID: memberIDToDeleted, CircleID: circleID}, orginalCircleID) != nil {
		log.Error("Error handling user leaving circle:", err)
		c.JSON(500, gin.H{
			"error": "Error handling user leaving circle",
		})
		return
	}

	err = h.circleRepo.DeleteMemberByID(c, circleID, memberIDToDeleted)
	if err != nil {
		log.Error("Error deleting circle member:", err)
		c.JSON(500, gin.H{
			"error": "Error deleting circle member",
		})
		return
	}
	c.JSON(200, gin.H{
		"res": "User deleted from circle successfully",
	})
}

func (h *Handler) GetUserCircles(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	circles, err := h.circleRepo.GetUserCircles(c, currentUser.ID)
	if err != nil {
		log.Error("Error getting user circles:", err)
		c.JSON(500, gin.H{
			"error": "Error getting user circles",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": circles,
	})
}

func (h *Handler) GetPendingCircleMembers(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	currentMemebers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle members",
		})
		return
	}

	// confirm that the current user is an admin:
	isAdmin := false
	for _, member := range currentMemebers {
		if member.UserID == currentUser.ID && member.Role == "admin" {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "You are not an admin of this circle",
		})
		return
	}

	members, err := h.circleRepo.GetPendingJoinRequests(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting pending circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting pending circle members",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": members,
	})
}

func (h *Handler) AcceptJoinRequest(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	rawRequestID := c.Query("requestId")
	requestID, err := strconv.Atoi(rawRequestID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	currentMemebers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle members",
		})
		return
	}

	// confirm that the current user is an admin:
	isAdmin := false
	for _, member := range currentMemebers {
		if member.UserID == currentUser.ID && member.Role == "admin" {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "You are not an admin of this circle",
		})
		return
	}
	pendingRequests, err := h.circleRepo.GetPendingJoinRequests(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting pending circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting pending circle members",
		})
		return
	}
	isActiveRequest := false
	var requestedCircle *cModel.UserCircleDetail
	for _, request := range pendingRequests {
		if request.ID == requestID {
			requestedCircle = request
			isActiveRequest = true
			break
		}
	}
	if !isActiveRequest {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	err = h.circleRepo.AcceptJoinRequest(c, currentUser.CircleID, requestID)
	if err != nil {
		log.Error("Error accepting join request:", err)
		c.JSON(500, gin.H{
			"error": "Error accepting join request",
		})
		return
	}

	if err := h.userRepo.UpdateUserCircle(c, requestedCircle.UserID, currentUser.CircleID); err != nil {
		log.Error("Error updating user circle:", err)
		c.JSON(500, gin.H{
			"error": "Error updating user circle",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Join request accepted successfully",
	})
}

func Routes(router *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {
	log.Println("Registering routes")

	circleRoutes := router.Group("circles")
	circleRoutes.Use(auth.MiddlewareFunc())
	{
		circleRoutes.GET("/members", h.GetCircleMembers)
		circleRoutes.GET("/members/requests", h.GetPendingCircleMembers)
		circleRoutes.PUT("/members/requests/accept", h.AcceptJoinRequest)
		circleRoutes.GET("/", h.GetUserCircles)
		circleRoutes.POST("/join", h.JoinCircle)
		circleRoutes.DELETE("/leave", h.LeaveCircle)
		circleRoutes.DELETE("/:id/members/delete", h.DeleteCircleMember)

	}

}
