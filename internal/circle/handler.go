package circle

import (
	"fmt"
	"log"

	"strconv"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	"donetick.com/core/internal/chore"
	chRepo "donetick.com/core/internal/chore/repo"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/notifier/push"
	"donetick.com/core/internal/notifier/service/fcm"
	pRepo "donetick.com/core/internal/points/repo"
	pjRepo "donetick.com/core/internal/project/repo"
	"donetick.com/core/internal/storage"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	circleRepo           *cRepo.CircleRepository
	userRepo             *uRepo.UserRepository
	choreRepo            *chRepo.ChoreRepository
	pointRepo            *pRepo.PointsRepository
	pushSender           *push.Sender
	projectRepo          *pjRepo.ProjectRepository
	signer               storage.URLSigner
	isDonetickDotCom     bool
	maxCircleMembers     int
	plusMaxCircleMembers int
	singleCircleInstance bool
}

func NewHandler(cr *cRepo.CircleRepository, ur *uRepo.UserRepository, c *chRepo.ChoreRepository, pr *pRepo.PointsRepository,
	ps *push.Sender, signer storage.URLSigner, config *config.Config, projectRepo *pjRepo.ProjectRepository) *Handler {
	return &Handler{
		circleRepo:           cr,
		userRepo:             ur,
		choreRepo:            c,
		pointRepo:            pr,
		pushSender:           ps,
		projectRepo:          projectRepo,
		signer:               signer,
		isDonetickDotCom:     config.IsDoneTickDotCom,
		maxCircleMembers:     config.FeatureLimits.MaxCircleMembers,
		plusMaxCircleMembers: config.FeatureLimits.PlusCircleMaxMembers,
		singleCircleInstance: config.SingleCircleInstance,
	}
}

// GetCircleMembers godoc
//
//	@Summary		Get circle members
//	@Description	Retrieves all members of the current user's circle
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string][]cModel.UserCircleDetail	"res: array of circle members"
//	@Failure		500	{object}	map[string]string						"error: Error getting current user"
//	@Failure		500	{object}	map[string]string						"error: Error getting circle members"
//	@Router			/circles/members [get]
func (h *Handler) GetCircleMembers(c *gin.Context) {
	// Get the circle ID from the JWT
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c) // TODO we might want to change this from 500 to 401
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

	for i := range members {
		members[i].Image = h.signer.SignIfLocal(members[i].Image)
	}

	c.JSON(200, gin.H{
		"res": members,
	})
}

// JoinCircle godoc
//
//	@Summary		Request to join a circle
//	@Description	Requests to join a circle using an invite code
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			invite_code	query		string				true	"Invite code"
//	@Success		200			{object}	map[string]string	"res: User Requested to join circle successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid request"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		409			{object}	map[string]string	"error: You are already a member of this circle"
//	@Failure		500			{object}	map[string]string	"error: Error adding user to circle"
//	@Router			/circles/join [post]
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
	if err != nil {
		log.Error("Error getting circle by invite code:", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

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

	h.notifyAdminsOfJoinRequest(c, circle, &currentUser.User)

	c.JSON(200, gin.H{
		"res": "User Requested to join circle successfully",
	})
}

// notifyAdminsOfJoinRequest pushes a notification to every admin of the circle
// so they can review the pending join request. Failures are logged only: the
// join request itself already succeeded.
func (h *Handler) notifyAdminsOfJoinRequest(c *gin.Context, circle *cModel.Circle, requester *uModel.User) {
	log := logging.FromContext(c)

	admins, err := h.circleRepo.GetCircleAdmins(c, circle.ID)
	if err != nil {
		log.Error("Error getting circle admins for join request notification:", err)
		return
	}

	adminIDs := make([]int, 0, len(admins))
	seen := make(map[int]bool, len(admins))
	for _, admin := range admins {
		if admin.UserID == requester.ID || seen[admin.UserID] {
			continue
		}
		seen[admin.UserID] = true
		adminIDs = append(adminIDs, admin.UserID)
	}
	if len(adminIDs) == 0 {
		return
	}

	payload := fcm.FCMNotificationPayload{
		Title: "New Join Request",
		Body:  fmt.Sprintf("%s wants to join %s", displayNameOrUsername(requester), circle.Name),
		Data: map[string]string{
			"type":           "circle_join_request",
			"circle_id":      strconv.Itoa(circle.ID),
			"circle_name":    circle.Name,
			"requester_id":   strconv.Itoa(requester.ID),
			"requester_name": displayNameOrUsername(requester),
		},
	}

	h.pushSender.SendToUsers(c, adminIDs, payload)
}

func displayNameOrUsername(u *uModel.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// LeaveCircle godoc
//
//	@Summary		Leave a circle
//	@Description	Removes the current user from a circle and restores their original circle
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			circle_id	query		int					true	"Circle ID"
//	@Success		200			{object}	map[string]string	"res: User left circle successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid request"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		500			{object}	map[string]string	"error: Error leaving circle"
//	@Router			/circles/leave [delete]
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

	if circleID != currentUser.CircleID {
		c.JSON(400, gin.H{
			"error": "circle_id must match your current circle",
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
	if err := handleUserLeavingCircle(h, c, &currentUser.User, orginalCircleID); err != nil {
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

// reassignChoresForLeavingUser moves the chores the user created in the circle
// they are leaving to destinationCircleID (keeping them assigned to the user),
// and hands the chores that were merely assigned to them back to the circle.
//
// Chores the user created inside a project that is private to or shared with
// selected members belong to that project's work: they stay there and the
// project owner takes them over, since nobody could see a restricted project's
// chore in another circle. Restricted projects the user owns are removed (their
// remaining chores are detached and kept); circle-wide projects stay.
func reassignChoresForLeavingUser(h *Handler, c *gin.Context, leavingUser *uModel.User, destinationCircleID int) error {
	userAssignedCircleChores, err := h.choreRepo.GetChores(c, leavingUser.CircleID, leavingUser.ID, true, nil, false)
	if err != nil {
		return err
	}
	restricted, err := h.projectRepo.GetRestrictedProjects(c, leavingUser.CircleID)
	if err != nil {
		return err
	}
	movedChoreIDs := make([]int, 0)
	for _, ch := range userAssignedCircleChores {
		if ch.CreatedBy == leavingUser.ID && ch.ProjectID != nil {
			if project, ok := restricted[*ch.ProjectID]; ok && project.CreatedBy != leavingUser.ID {
				chore.HandOverToProjectOwner(ch, leavingUser.ID, project.CreatedBy)
				continue
			}
		}

		if ch.CreatedBy == leavingUser.ID && (ch.AssignedTo == nil || *ch.AssignedTo != leavingUser.ID) {
			ch.AssignedTo = &leavingUser.ID
			ch.UpdatedAt = time.Now().UTC()
			ch.UpdatedBy = leavingUser.ID
			ch.CircleID = destinationCircleID
			// The project belongs to the circle being left; a dangling reference would
			// hide the chore in the new circle's project views.
			ch.ProjectID = nil
			movedChoreIDs = append(movedChoreIDs, ch.ID)
		} else if ch.CreatedBy != leavingUser.ID && (ch.AssignedTo != nil && *ch.AssignedTo == leavingUser.ID) {
			chore.RemoveAssigneeAndReassign(ch, leavingUser.ID)
		}

	}

	h.choreRepo.UpdateChores(c, userAssignedCircleChores)
	// The moved chores now live in another circle; tell the remaining members'
	// devices to drop their copies.
	if err := h.choreRepo.TombstoneChoresLeavingCircle(c, leavingUser.CircleID, movedChoreIDs); err != nil {
		logging.FromContext(c).Error("Error recording tombstones for moved chores", "error", err)
	}
	h.choreRepo.RemoveChoreAssigneeByCircleID(c, leavingUser.ID, leavingUser.CircleID)
	if err := h.choreRepo.DeletePendingNotificationsForUser(c, leavingUser.CircleID, leavingUser.ID); err != nil {
		logging.FromContext(c).Error("Error removing pending notifications of leaving user", "error", err)
	}
	if err := h.projectRepo.RemoveProjectsOwnedBy(c, leavingUser.CircleID, leavingUser.ID); err != nil {
		logging.FromContext(c).Error("Error removing projects of leaving user", "error", err)
	}
	return nil
}

func handleUserLeavingCircle(h *Handler, c *gin.Context, leavingUser *uModel.User, orginalCircleID int) error {
	if err := reassignChoresForLeavingUser(h, c, leavingUser, orginalCircleID); err != nil {
		return err
	}
	h.circleRepo.AssignDefaultCircle(c, leavingUser.ID)
	return nil
}

// DeleteCircleMember godoc
//
//	@Summary		Delete a circle member
//	@Description	Removes a member from a circle (admin only)
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id			path		int					true	"Circle ID"
//	@Param			member_id	query		int					true	"Member ID to delete"
//	@Success		200			{object}	map[string]string	"res: User deleted from circle successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid request"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		403			{object}	map[string]string	"error: You are not an admin of this circle"
//	@Failure		500			{object}	map[string]string	"error: Error deleting circle member"
//	@Router			/circles/{id}/members/delete [delete]
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

// GetUserCircles godoc
//
//	@Summary		Get user circles
//	@Description	Retrieves all circles the current user belongs to
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string][]cModel.Circle	"res: array of circles"
//	@Failure		401	{object}	map[string]string			"error: Authentication failed"
//	@Failure		500	{object}	map[string]string			"error: Error getting user circles"
//	@Router			/circles [get]
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

// GetPendingCircleMembers godoc
//
//	@Summary		Get pending join requests
//	@Description	Retrieves pending circle join requests (admin only)
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{object}	map[string][]cModel.UserCircleDetail	"res: array of pending members"
//	@Failure		401	{object}	map[string]string						"error: Authentication failed"
//	@Failure		403	{object}	map[string]string						"error: You are not an admin of this circle"
//	@Failure		500	{object}	map[string]string						"error: Error getting pending circle members"
//	@Router			/circles/members/requests [get]
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

	for i := range members {
		members[i].Image = h.signer.SignIfLocal(members[i].Image)
	}

	c.JSON(200, gin.H{
		"res": members,
	})
}

// requireCircleAdmin writes an error response and returns false when userID is
// not an admin among the given circle members.
func (h *Handler) requireCircleAdmin(c *gin.Context, members []*cModel.UserCircleDetail, userID int) bool {
	for _, member := range members {
		if member.UserID == userID && member.Role == cModel.UserRoleAdmin {
			return true
		}
	}
	c.JSON(403, gin.H{
		"error": "You are not an admin of this circle",
	})
	return false
}

// findPendingRequest resolves requestID to one of the circle's pending join
// requests, writing an error response and returning false when it is not.
func (h *Handler) findPendingRequest(c *gin.Context, circleID, requestID int) (*cModel.UserCircleDetail, bool) {
	log := logging.FromContext(c)

	pendingRequests, err := h.circleRepo.GetPendingJoinRequests(c, circleID)
	if err != nil {
		log.Error("Error getting pending circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting pending circle members",
		})
		return nil, false
	}

	for _, request := range pendingRequests {
		if request.ID == requestID {
			return request, true
		}
	}

	c.JSON(400, gin.H{
		"error": "Invalid request",
	})
	return nil, false
}

// AcceptJoinRequest godoc
//
//	@Summary		Accept a join request
//	@Description	Accepts a pending circle join request (admin only)
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			requestId	query		int					true	"Request ID"
//	@Success		200			{object}	map[string]string	"res: Join request accepted successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid request / Circle is full"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		403			{object}	map[string]string	"error: You are not an admin of this circle"
//	@Failure		500			{object}	map[string]string	"error: Error accepting join request"
//	@Router			/circles/members/requests/accept [put]
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

	currentMembers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	// filter to the active members only:
	activeMembers := make([]*cModel.UserCircleDetail, 0)
	for _, member := range currentMembers {
		if member.IsActive {
			activeMembers = append(activeMembers, member)
		}
	}
	if err != nil {
		log.Error("Error getting circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle members",
		})
		return
	}
	if h.isDonetickDotCom {
		maxMembers := h.maxCircleMembers
		if currentUser.IsPlusMember() {
			maxMembers = h.plusMaxCircleMembers
		}
		if len(activeMembers) >= maxMembers {
			log.Error("Circle is full")
			c.JSON(400, gin.H{
				"error": "Circle is full, you can only have " + strconv.Itoa(maxMembers) + " members in a circle",
			})
			return
		}
	}
	if !h.requireCircleAdmin(c, currentMembers, currentUser.ID) {
		return
	}

	requestedCircle, ok := h.findPendingRequest(c, currentUser.CircleID, requestID)
	if !ok {
		return
	}

	// Capture the circle the user is in before accepting: UpdateUserCircle
	// below repoints them, and the cleanup afterwards needs the old value.
	previousCircleID := 0
	if requester, err := h.userRepo.GetUserByID(c, requestedCircle.UserID); err != nil {
		log.Error("Error getting requester, skipping previous membership cleanup:", err)
	} else {
		previousCircleID = requester.CircleID
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

	h.removeStaleMembership(c, requestedCircle.UserID, previousCircleID, currentUser.CircleID)

	h.notifyMemberOfAcceptedRequest(c, currentUser.CircleID, requestedCircle.UserID)

	c.JSON(200, gin.H{
		"res": "Join request accepted successfully",
	})

}

// removeStaleMembership drops the membership the accepted user held in the
// shared circle they just moved out of. Without this the old membership stays
// is_active, so the user keeps a seat in its member list and stays in its chore
// viewer set (which is what offline sync uses to decide what reaches a device).
//
// Memberships the user administers are left alone: that is their own circle,
// and LeaveCircle needs it to send them back. Failures are logged only, since
// the user has already been accepted into the new circle by this point.
func (h *Handler) removeStaleMembership(c *gin.Context, userID, previousCircleID, newCircleID int) {
	log := logging.FromContext(c)

	if previousCircleID == 0 || previousCircleID == newCircleID {
		return
	}

	previousRole, err := h.circleRepo.GetUserCircleRole(c, previousCircleID, userID)
	if err != nil {
		log.Error("Error getting previous circle role:", err)
		return
	}
	if previousRole == cModel.RoleAdmin {
		// Their own circle: keep the membership so they can return to it.
		return
	}
	if previousRole == "" {
		// No membership row there, nothing to clean up.
		return
	}

	// Chores the user created in the old circle follow them out, but they are
	// parked in the circle the user administers rather than the newly joined
	// one, so that a shared circle's tasks never surface in another one.
	destinationCircleID, err := h.circleRepo.GetUserOriginalCircle(c, userID)
	if err != nil || destinationCircleID == 0 {
		log.Warnw("Skipping previous membership cleanup: no circle to move the user's chores to",
			"userID", userID, "previousCircleID", previousCircleID, "error", err)
		return
	}

	leavingUser := &uModel.User{ID: userID, CircleID: previousCircleID}
	if err := reassignChoresForLeavingUser(h, c, leavingUser, destinationCircleID); err != nil {
		log.Error("Error reassigning chores from previous circle:", err)
		return
	}

	if err := h.circleRepo.LeaveCircleByUserID(c, previousCircleID, userID); err != nil {
		log.Error("Error removing previous circle membership:", err)
		return
	}

	log.Infow("Removed previous circle membership after join request acceptance",
		"userID", userID,
		"previousCircleID", previousCircleID,
		"newCircleID", newCircleID,
		"choresMovedTo", destinationCircleID)
}

// DeclineJoinRequest godoc
//
//	@Summary		Decline a join request
//	@Description	Declines a pending circle join request (admin only). The request is removed, so the user may request to join again later.
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			requestId	query		int					true	"Request ID"
//	@Success		200			{object}	map[string]string	"res: Join request declined successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid request"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		403			{object}	map[string]string	"error: You are not an admin of this circle"
//	@Failure		500			{object}	map[string]string	"error: Error declining join request"
//	@Router			/circles/members/requests/decline [delete]
func (h *Handler) DeclineJoinRequest(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	requestID, err := strconv.Atoi(c.Query("requestId"))
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	currentMembers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle members:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle members",
		})
		return
	}

	if !h.requireCircleAdmin(c, currentMembers, currentUser.ID) {
		return
	}

	requestedCircle, ok := h.findPendingRequest(c, currentUser.CircleID, requestID)
	if !ok {
		return
	}

	if err := h.circleRepo.DeclineJoinRequest(c, currentUser.CircleID, requestID); err != nil {
		log.Error("Error declining join request:", err)
		c.JSON(500, gin.H{
			"error": "Error declining join request",
		})
		return
	}

	h.notifyMemberOfDeclinedRequest(c, currentUser.CircleID, requestedCircle.UserID)

	c.JSON(200, gin.H{
		"res": "Join request declined successfully",
	})
}

// notifyMemberOfDeclinedRequest lets the requester know their join request was
// turned down. Failures are logged only: the request was already removed.
func (h *Handler) notifyMemberOfDeclinedRequest(c *gin.Context, circleID, memberID int) {
	log := logging.FromContext(c)

	circle, err := h.circleRepo.GetCircleByID(c, circleID)
	if err != nil {
		log.Error("Error getting circle for join decline notification:", err)
		return
	}

	payload := fcm.FCMNotificationPayload{
		Title: "Join Request Declined",
		Body:  fmt.Sprintf("Your request to join %s was declined.", circle.Name),
		Data: map[string]string{
			"type":        "circle_join_declined",
			"circle_id":   strconv.Itoa(circle.ID),
			"circle_name": circle.Name,
		},
	}

	if _, err := h.pushSender.SendToUser(c, memberID, payload); err != nil {
		log.Warn("Failed to send join decline notification", "user_id", memberID, "error", err)
	}
}

// notifyMemberOfAcceptedRequest welcomes the newly accepted member on their
// devices. Failures are logged only: the member was already added.
func (h *Handler) notifyMemberOfAcceptedRequest(c *gin.Context, circleID, memberID int) {
	log := logging.FromContext(c)

	circle, err := h.circleRepo.GetCircleByID(c, circleID)
	if err != nil {
		log.Error("Error getting circle for join acceptance notification:", err)
		return
	}

	payload := fcm.FCMNotificationPayload{
		Title: "Welcome to " + circle.Name,
		Body:  fmt.Sprintf("Your request to join %s was approved. You can now see and complete its tasks.", circle.Name),
		Data: map[string]string{
			"type":        "circle_join_approved",
			"circle_id":   strconv.Itoa(circle.ID),
			"circle_name": circle.Name,
		},
	}

	if _, err := h.pushSender.SendToUser(c, memberID, payload); err != nil {
		log.Warn("Failed to send join acceptance notification", "user_id", memberID, "error", err)
	}
}

// RedeemPoints godoc
//
//	@Summary		Redeem member points
//	@Description	Redeems points for a circle member (admin only)
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int								true	"Circle ID"
//	@Param			points	body		object{points=int,userId=int}	true	"Points redemption request"
//	@Success		200		{object}	map[string]string				"res: Points redeemed successfully"
//	@Failure		400		{object}	map[string]string				"error: Invalid request / User does not have enough points / User is not a member of this circle"
//	@Failure		401		{object}	map[string]string				"error: Authentication failed"
//	@Failure		403		{object}	map[string]string				"error: You are not an admin of this circle"
//	@Failure		500		{object}	map[string]string				"error: Error redeeming points"
//	@Router			/circles/{id}/members/points/redeem [post]
func (h *Handler) RedeemPoints(c *gin.Context) {
	type RedeemPointsRequest struct {
		Points int `json:"points"`
		UserID int `json:"userId"`
	}

	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	// parse body:
	var redeemReq RedeemPointsRequest

	if err := c.ShouldBindJSON(&redeemReq); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return

	}

	if redeemReq.Points <= 0 {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	circleIdRaw := c.Param("id")

	circleID, err := strconv.Atoi(circleIdRaw)
	if err != nil {
		log.Error("Error redeeming points: invalid circle id")
		c.JSON(400, gin.H{
			"error": "Invalid request: invalid circle id",
		})
		return
	}
	if circleID != currentUser.CircleID {
		log.Error("You are not a member of this circle")
		c.JSON(400, gin.H{
			"error": "You are not a member of this circle",
		})
		return
	}
	members, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle admins:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle admins",
		})
		return
	}
	isAdmin := false
	isValidMember := false
	var member *cModel.UserCircleDetail
	for _, user := range members {
		if user.UserID == currentUser.ID && user.Role == "admin" {
			isAdmin = true
		}
		if user.UserID == redeemReq.UserID {
			isValidMember = true
			member = user
		}

	}

	if !isAdmin {
		log.Error("Error redeeming points: user is not an admin of this circle")
		c.JSON(403, gin.H{
			"error": "You are not an admin of this circle",
		})
		return
	}
	if !isValidMember {
		log.Error("Error redeeming points: user is not a member of this circle")
		c.JSON(400, gin.H{
			"error": "User is not a member of this circle",
		})
		return
	}
	if member.Points-redeemReq.Points < 0 {
		log.Error("Error redeeming points: user does not have enough points")
		c.JSON(400, gin.H{
			"error": "User does not have enough points",
		})
		return
	}

	err = h.circleRepo.RedeemPoints(c, currentUser.CircleID, redeemReq.UserID, redeemReq.Points, currentUser.ID)
	if err != nil {
		log.Error("Error redeeming points:", err)
		c.JSON(500, gin.H{
			"error": "Error redeeming points",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Points redeemed successfully",
	})
}

// ChangeMemberRole godoc
//
//	@Summary		Change member role
//	@Description	Changes the role of a circle member (admin only)
//	@Tags			circles
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			role	body		object{memberId=int,role=cModel.Role}	true	"Role change request"
//	@Success		200		{object}	map[string]string						"res: Member role changed successfully"
//	@Failure		400		{object}	map[string]string						"error: Invalid request / Invalid role / User is not a member of this circle"
//	@Failure		401		{object}	map[string]string						"error: Authentication failed"
//	@Failure		403		{object}	map[string]string						"error: You are not an admin of this circle"
//	@Failure		500		{object}	map[string]string						"error: Error changing member role"
//	@Router			/circles/members/role [put]
func (h *Handler) ChangeMemberRole(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	type changeRoleRequest struct {
		MemberID int         `json:"memberId"`
		Role     cModel.Role `json:"role"`
	}
	var req changeRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error("Error changing member role:", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	if !cModel.IsValidRole(req.Role) {
		log.Error("Error changing member role: invalid role")
		c.JSON(400, gin.H{
			"error": "Invalid role",
		})
		return
	}

	users, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Error("Error getting circle admins:", err)
		c.JSON(500, gin.H{
			"error": "Error getting circle admins",
		})
		return
	}
	isAdmin := false
	memberFound := false
	adminCount := 0
	for _, user := range users {
		if user.Role == "admin" {
			adminCount++
			if user.UserID == currentUser.ID {
				isAdmin = true
			}
		}
		if user.UserID == req.MemberID {
			memberFound = true
		}
	}
	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "You are not an admin of this circle",
		})
		return
	}
	if !memberFound {
		c.JSON(400, gin.H{
			"error": "User is not a member of this circle",
		})
		return
	}

	err = h.circleRepo.ChangeUserRole(c, currentUser.CircleID, req.MemberID, req.Role)
	if err != nil {
		log.Error("Error changing member role:", err)
		c.JSON(500, gin.H{
			"error": "Error changing member role",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Member role changed successfully",
	})

}

func Routes(router *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	log.Println("Registering circle routes")

	circleRoutes := router.Group("api/v1/circles")
	circleRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		circleRoutes.GET("/members", h.GetCircleMembers)
		circleRoutes.GET("/members/requests", h.GetPendingCircleMembers)
		circleRoutes.PUT("/members/role", h.ChangeMemberRole)
		circleRoutes.GET("/", h.GetUserCircles)
		circleRoutes.POST("/:id/members/points/redeem", h.RedeemPoints)

		if !h.singleCircleInstance {
			circleRoutes.PUT("/members/requests/accept", h.AcceptJoinRequest)
			circleRoutes.DELETE("/members/requests/decline", h.DeclineJoinRequest)
			circleRoutes.POST("/join", h.JoinCircle)
			circleRoutes.DELETE("/leave", h.LeaveCircle)
			circleRoutes.DELETE("/:id/members/delete", h.DeleteCircleMember)
		}
	}
}
