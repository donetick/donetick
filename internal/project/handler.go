package project

import (
	"errors"
	"sort"
	"strconv"

	"donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	nps "donetick.com/core/internal/notifier/service"
	pModel "donetick.com/core/internal/project/model"
	pRepo "donetick.com/core/internal/project/repo"
	"donetick.com/core/internal/realtime"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	pRepo           *pRepo.ProjectRepository
	choreRepo       *chRepo.ChoreRepository
	nPlanner        *nps.NotificationPlanner
	realTimeService *realtime.RealTimeService
}

func NewHandler(pRepo *pRepo.ProjectRepository, choreRepo *chRepo.ChoreRepository, planner *nps.NotificationPlanner, rts *realtime.RealTimeService) *Handler {
	return &Handler{
		pRepo:           pRepo,
		choreRepo:       choreRepo,
		nPlanner:        planner,
		realTimeService: rts,
	}
}

// statusFor maps repository errors to HTTP status codes.
func statusFor(err error) int {
	switch {
	case errors.Is(err, pRepo.ErrProjectNotFound):
		return 404
	case errors.Is(err, pRepo.ErrInvalidVisibility), errors.Is(err, pRepo.ErrMemberNotInCircle):
		return 400
	case err != nil && (err.Error() == "user does not have permission to update this project" ||
		err.Error() == "user does not have permission to delete this project"):
		return 403
	default:
		return 500
	}
}

// getProjects godoc
//
//	@Summary		Get all projects
//	@Description	Retrieves the projects of the current user's circle the user can see: circle-wide ones, their own private ones and the ones shared with them
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Success		200	{array}		pModel.Project		"array of projects"
//	@Failure		401	{object}	map[string]string	"error: Error getting current user"
//	@Failure		500	{object}	map[string]string	"error: Error getting projects"
//	@Router			/projects [get]
func (h *Handler) getProjects(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projects, err := h.pRepo.GetCircleProjects(c, currentUser.CircleID, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting projects",
		})
		return
	}

	c.JSON(200, projects)
}

// createProject godoc
//
//	@Summary		Create a new project
//	@Description	Creates a new project for the current user's circle. visibility is "private" (only me), "members" (me and memberIds) or "circle" (everyone in the circle, the default).
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			project	body		pModel.ProjectReq			true	"Project creation request"
//	@Success		200		{object}	map[string]pModel.Project	"res: created project object"
//	@Failure		400		{object}	map[string]string			"error: Error binding project data"
//	@Failure		401		{object}	map[string]string			"error: Error getting current user"
//	@Failure		500		{object}	map[string]string			"error: Error creating project"
//	@Router			/projects [post]
func (h *Handler) createProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	var req pModel.ProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Error binding project data",
		})
		return
	}
	if req.Visibility != nil && !req.Visibility.Valid() {
		c.JSON(400, gin.H{"error": pRepo.ErrInvalidVisibility.Error()})
		return
	}
	visibility, members := req.ResolveAccess(pModel.ProjectVisibilityCircle, nil)

	project := &pModel.Project{
		Name:              req.Name,
		Description:       req.Description,
		Color:             req.Color,
		CircleID:          currentUser.CircleID,
		CreatedBy:         currentUser.ID,
		Icon:              req.Icon,
		IsPrivate:         visibility != pModel.ProjectVisibilityCircle,
		SharedWithMembers: visibility == pModel.ProjectVisibilityMembers,
		MemberIDs:         members,
	}

	if err := h.pRepo.CreateProject(c, project); err != nil {
		status := statusFor(err)
		msg := "Error creating project"
		if status == 400 {
			msg = err.Error()
		}
		c.JSON(status, gin.H{
			"error": msg,
		})
		return
	}

	c.JSON(200, gin.H{
		"res": project,
	})
}

// updateProject godoc
//
//	@Summary		Update a project
//	@Description	Updates name, description, color and icon of a project owned by the current user and, optionally, who can see it (visibility, memberIds). Fields that are not sent keep their current value. Changing who can see the project also adjusts the assignees of its tasks; use POST /projects/{id}/access-preview to see the effect first.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int							true	"Project ID"
//	@Param			project	body		pModel.ProjectReq			true	"Project update request"
//	@Success		200		{object}	map[string]pModel.Project	"res: updated project object"
//	@Failure		400		{object}	map[string]string			"error: Invalid project ID, Error binding project data or invalid members"
//	@Failure		401		{object}	map[string]string			"error: Error getting current user"
//	@Failure		403		{object}	map[string]string			"error: not the project owner"
//	@Failure		404		{object}	map[string]string			"error: project not found"
//	@Failure		500		{object}	map[string]string			"error: Error updating project or Error getting updated project"
//	@Router			/projects/{id} [put]
func (h *Handler) updateProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid project ID",
		})
		return
	}

	var req pModel.ProjectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Error binding project data",
		})
		return
	}

	project := &pModel.Project{
		ID:          projectID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		Icon:        req.Icon,
	}

	result, err := h.pRepo.UpdateProjectWithAccess(c, project, &req, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(statusFor(err), gin.H{
			"error": err.Error(),
		})
		return
	}

	updatedProject, err := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting updated project",
		})
		return
	}

	h.afterAccessChange(c, currentUser.CircleID, updatedProject, result, &currentUser.User)

	c.JSON(200, gin.H{
		"res": updatedProject,
	})
}

// AccessPreviewReq is the audience a client is about to save.
type AccessPreviewReq struct {
	IsPrivate  *bool                     `json:"isPrivate"`
	Visibility *pModel.ProjectVisibility `json:"visibility"`
	MemberIDs  *[]int                    `json:"memberIds"`
}

// previewProjectAccess godoc
//
//	@Summary		Preview a change of who can see a project
//	@Description	Returns, without changing anything, which tasks of the project would change assignees and who would gain or lose access if the given visibility and members were saved. Only the project owner may call it.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int					true	"Project ID"
//	@Param			access	body		AccessPreviewReq	true	"Target visibility and members"
//	@Success		200		{object}	map[string]interface{}	"res: chRepo.ProjectAccessPlan"
//	@Failure		400		{object}	map[string]string	"error"
//	@Failure		403		{object}	map[string]string	"error: not the project owner"
//	@Failure		404		{object}	map[string]string	"error: project not found"
//	@Router			/projects/{id}/access-preview [post]
func (h *Handler) previewProjectAccess(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Error getting current user"})
		return
	}
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid project ID"})
		return
	}
	var req AccessPreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Error binding access data"})
		return
	}
	plan, err := h.pRepo.PreviewProjectAccess(c, projectID, &pModel.ProjectReq{
		IsPrivate:  req.IsPrivate,
		Visibility: req.Visibility,
		MemberIDs:  req.MemberIDs,
	}, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"res": gin.H{
			"before":       plan.Before,
			"after":        plan.After,
			"ownerId":      plan.OwnerID,
			"totalChores":  len(plan.Chores),
			"changes":      plan.ChangedChores(),
			"revokedUsers": plan.RevokedUsers,
			"grantedUsers": plan.GrantedUsers,
		},
	})
}

// deleteProject godoc
//
//	@Summary		Delete a project
//	@Description	Deletes a project by ID; restricted to the current user's circle. Its tasks are kept without a project and follow their own privacy from then on.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id	path		int					true	"Project ID"
//	@Success		200	{object}	map[string]string	"res: Project deleted successfully"
//	@Failure		400	{object}	map[string]string	"error: Invalid project ID"
//	@Failure		401	{object}	map[string]string	"error: Error getting current user"
//	@Failure		500	{object}	map[string]string	"error: Error deleting project"
//	@Router			/projects/{id} [delete]
func (h *Handler) deleteProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid project ID",
		})
		return
	}

	// Remember who could see the project, to tell their clients it is gone.
	before, _ := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)

	if err := h.pRepo.DeleteProject(c, projectID, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(statusFor(err), gin.H{
			"error": err.Error(),
		})
		return
	}

	if before != nil && h.realTimeService != nil {
		recipients, _ := before.ViewerIDs()
		h.realTimeService.GetEventBroadcaster().BroadcastProjectUpdated(currentUser.CircleID, projectID, true, &currentUser.User, recipients)
	}

	c.JSON(200, gin.H{
		"res": "Project deleted successfully",
	})
}

// afterAccessChange does the post-commit work of a project update: it regenerates
// the reminders of chores whose audience or assignees changed (they were removed
// inside the transaction), tells users who lost a chore to drop it, and tells
// everybody who could see the project before or after to refresh.
func (h *Handler) afterAccessChange(c *gin.Context, circleID int, project *pModel.Project, result *chRepo.ProjectAccessResult, user *uModel.User) {
	log := logging.FromContext(c)

	if result != nil && h.nPlanner != nil {
		for _, choreID := range result.NotificationChoreIDs {
			chore, err := h.choreRepo.GetChoreForSystem(c, choreID)
			if err != nil {
				log.Error("Error reloading chore for notifications", "error", err, "choreID", choreID)
				continue
			}
			h.nPlanner.GenerateNotifications(c, chore)
		}
	}

	if h.realTimeService == nil {
		return
	}
	broadcaster := h.realTimeService.GetEventBroadcaster()

	accessChanged := result != nil
	if result != nil {
		names := make(map[int]string, len(result.Plan.Chores))
		for _, change := range result.Plan.Chores {
			names[change.ChoreID] = change.Name
		}
		choreIDs := make([]int, 0, len(result.Revocations))
		for choreID := range result.Revocations {
			choreIDs = append(choreIDs, choreID)
		}
		sort.Ints(choreIDs)
		for _, choreID := range choreIDs {
			broadcaster.BroadcastChoreAccessRevoked(circleID, choreID, names[choreID], user, result.Revocations[choreID])
		}
	}

	// A circle-wide project before or after the change concerns the whole circle;
	// otherwise only its owner and the members on either side of the change.
	var recipients []int
	circleWide := project.Visibility() == pModel.ProjectVisibilityCircle
	if result != nil && result.Plan.Before.Visibility == pModel.ProjectVisibilityCircle {
		circleWide = true
	}
	if !circleWide {
		set := map[int]struct{}{project.CreatedBy: {}}
		for _, id := range project.MemberIDs {
			set[id] = struct{}{}
		}
		if result != nil {
			for _, id := range result.Plan.Before.MemberIDs {
				set[id] = struct{}{}
			}
		}
		recipients = make([]int, 0, len(set))
		for id := range set {
			recipients = append(recipients, id)
		}
		sort.Ints(recipients)
	}
	broadcaster.BroadcastProjectUpdated(circleID, project.ID, accessChanged, user, recipients)
}

func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	projectRoutes := r.Group("api/v1/projects")
	projectRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		projectRoutes.GET("", h.getProjects)
		projectRoutes.POST("", h.createProject)
		projectRoutes.PUT("/:id", h.updateProject)
		projectRoutes.POST("/:id/access-preview", h.previewProjectAccess)
		projectRoutes.DELETE("/:id", h.deleteProject)
	}
}
