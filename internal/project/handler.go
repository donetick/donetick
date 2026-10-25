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
	case errors.Is(err, pRepo.ErrProjectNotFound), errors.Is(err, pRepo.ErrNotProjectViewer),
		errors.Is(err, pRepo.ErrSectionNotFound), errors.Is(err, pRepo.ErrChoreNotFound):
		return 404
	case errors.Is(err, pRepo.ErrInvalidVisibility), errors.Is(err, pRepo.ErrMemberNotInCircle),
		errors.Is(err, pRepo.ErrLastSection), errors.Is(err, pRepo.ErrInvalidView):
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

	ids := make([]int, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	views, err := h.pRepo.GetViewPreferences(c, currentUser.ID, ids)
	if err == nil {
		for i := range projects {
			projects[i].View = views[projects[i].ID]
		}
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
	if req.Name == "" {
		c.JSON(400, gin.H{"error": "Project name is required"})
		return
	}
	if req.Visibility != nil && !req.Visibility.Valid() {
		c.JSON(400, gin.H{"error": pRepo.ErrInvalidVisibility.Error()})
		return
	}
	if req.View != nil && !req.View.Valid() {
		c.JSON(400, gin.H{"error": pRepo.ErrInvalidView.Error()})
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

	if req.View != nil {
		if err := h.pRepo.SetViewPreference(c, project.ID, currentUser.CircleID, currentUser.ID, *req.View); err == nil {
			project.View = *req.View
		}
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

	// View is a private, per-user preference: anyone who can see the project
	// may set their own, independent of who owns it. Applied before the
	// owner-only fields below so a view-only request (no other field sent)
	// never has to pass the ownership check that guards the rest of this
	// endpoint.
	if req.View != nil {
		if !req.View.Valid() {
			c.JSON(400, gin.H{"error": pRepo.ErrInvalidView.Error()})
			return
		}
		if err := h.pRepo.SetViewPreference(c, projectID, currentUser.CircleID, currentUser.ID, *req.View); err != nil {
			c.JSON(statusFor(err), gin.H{"error": err.Error()})
			return
		}
	}

	var result *chRepo.ProjectAccessResult
	if req.HasProjectFields() {
		if req.Name == "" {
			c.JSON(400, gin.H{"error": "Project name is required"})
			return
		}
		project := &pModel.Project{
			ID:          projectID,
			Name:        req.Name,
			Description: req.Description,
			Color:       req.Color,
			Icon:        req.Icon,
		}

		var err error
		result, err = h.pRepo.UpdateProjectWithAccess(c, project, &req, currentUser.ID, currentUser.CircleID)
		if err != nil {
			c.JSON(statusFor(err), gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	updatedProject, err := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting updated project",
		})
		return
	}
	if view, err := h.pRepo.GetViewPreference(c, projectID, currentUser.CircleID, currentUser.ID); err == nil {
		updatedProject.View = view
	}

	if result != nil {
		h.afterAccessChange(c, currentUser.CircleID, updatedProject, result, &currentUser.User)
	}

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

	notificationChores, err := h.pRepo.DeleteProjectWithResult(c, projectID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(statusFor(err), gin.H{
			"error": err.Error(),
		})
		return
	}
	// Reminders of detached chores were removed inside the transaction because
	// somebody lost sight of them; plan them again for whoever still may see them.
	if h.nPlanner != nil {
		for _, choreID := range notificationChores {
			chore, err := h.choreRepo.GetChoreForSystem(c, choreID)
			if err != nil {
				logging.FromContext(c).Error("Error reloading chore for notifications", "error", err, "choreID", choreID)
				continue
			}
			h.nPlanner.GenerateNotifications(c, chore)
		}
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

// broadcastBoardUpdated tells everyone who can see the project (owner and
// members only, for a restricted project) that its shared board changed.
func (h *Handler) broadcastBoardUpdated(c *gin.Context, circleID, projectID int, user *uModel.User) {
	if h.realTimeService == nil {
		return
	}
	project, err := h.pRepo.GetProjectByID(c, projectID, circleID)
	if err != nil {
		return
	}
	var recipients []int
	if ids, restricted := project.ViewerIDs(); restricted {
		recipients = ids
	}
	h.realTimeService.GetEventBroadcaster().BroadcastProjectBoardUpdated(circleID, projectID, user, recipients)
}

// getSections godoc
//
//	@Summary		Get a project's board sections
//	@Description	Lists the sections (board columns) of a project, ordered by position. Shared by every user who can see the project.
//	@Tags			projects
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id	path		int						true	"Project ID"
//	@Success		200	{object}	map[string]interface{}	"res: array of pModel.ProjectSection"
//	@Failure		404	{object}	map[string]string		"error: project not found"
//	@Router			/projects/{id}/sections [get]
func (h *Handler) getSections(c *gin.Context) {
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
	project, err := h.pRepo.GetProjectByID(c, projectID, currentUser.CircleID)
	if err != nil || !project.CanView(currentUser.ID) {
		c.JSON(404, gin.H{"error": pRepo.ErrProjectNotFound.Error()})
		return
	}
	sections, err := h.pRepo.GetSections(c, projectID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting sections"})
		return
	}
	c.JSON(200, gin.H{"res": sections})
}

// SectionReq is the body of a section create/rename request.
type SectionReq struct {
	Name string `json:"name" binding:"required"`
}

// createSection godoc
//
//	@Summary		Create a board section
//	@Description	Adds a column to a project's shared board. Any user who can see the project may add one.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int						true	"Project ID"
//	@Param			section	body		SectionReq				true	"Section name"
//	@Success		200		{object}	map[string]interface{}	"res: created pModel.ProjectSection"
//	@Failure		404		{object}	map[string]string		"error: project not found"
//	@Router			/projects/{id}/sections [post]
func (h *Handler) createSection(c *gin.Context) {
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
	var req SectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Error binding section data"})
		return
	}
	section, err := h.pRepo.CreateSection(c, projectID, currentUser.CircleID, currentUser.ID, req.Name)
	if err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	h.broadcastBoardUpdated(c, currentUser.CircleID, projectID, &currentUser.User)
	c.JSON(200, gin.H{"res": section})
}

// renameSection godoc
//
//	@Summary		Rename a board section
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id			path		int					true	"Project ID"
//	@Param			sectionId	path		int					true	"Section ID"
//	@Param			section		body		SectionReq			true	"New section name"
//	@Success		200			{object}	map[string]string	"res: ok"
//	@Failure		404			{object}	map[string]string	"error: project or section not found"
//	@Router			/projects/{id}/sections/{sectionId} [put]
func (h *Handler) renameSection(c *gin.Context) {
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
	sectionID, err := strconv.Atoi(c.Param("sectionId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid section ID"})
		return
	}
	var req SectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Error binding section data"})
		return
	}
	if err := h.pRepo.RenameSection(c, projectID, sectionID, currentUser.CircleID, currentUser.ID, req.Name); err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	h.broadcastBoardUpdated(c, currentUser.CircleID, projectID, &currentUser.User)
	c.JSON(200, gin.H{"res": "ok"})
}

// deleteSection godoc
//
//	@Summary		Delete a board section
//	@Description	Deletes a section, moving its chores into the project's remaining section with the lowest position. Fails if it is the only section.
//	@Tags			projects
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id			path		int					true	"Project ID"
//	@Param			sectionId	path		int					true	"Section ID"
//	@Success		200			{object}	map[string]string	"res: ok"
//	@Failure		400			{object}	map[string]string	"error: cannot delete the last section"
//	@Failure		404			{object}	map[string]string	"error: project or section not found"
//	@Router			/projects/{id}/sections/{sectionId} [delete]
func (h *Handler) deleteSection(c *gin.Context) {
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
	sectionID, err := strconv.Atoi(c.Param("sectionId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid section ID"})
		return
	}
	if err := h.pRepo.DeleteSection(c, projectID, sectionID, currentUser.CircleID, currentUser.ID); err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	h.broadcastBoardUpdated(c, currentUser.CircleID, projectID, &currentUser.User)
	c.JSON(200, gin.H{"res": "ok"})
}

// MoveSectionReq places a section right after afterSectionId, or first when
// afterSectionId is omitted.
type MoveSectionReq struct {
	AfterSectionID *int `json:"afterSectionId"`
}

// moveSection godoc
//
//	@Summary		Move a board section
//	@Description	Repositions one section to sort right after afterSectionId (first, when omitted). Computed from the current server-side order, so two users reordering columns at the same time cannot clobber each other.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id			path		int					true	"Project ID"
//	@Param			sectionId	path		int					true	"Section ID"
//	@Param			move		body		MoveSectionReq		true	"Section to place it after"
//	@Success		200			{object}	map[string]string	"res: ok"
//	@Failure		404			{object}	map[string]string	"error: project or section not found"
//	@Router			/projects/{id}/sections/{sectionId}/move [put]
func (h *Handler) moveSection(c *gin.Context) {
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
	sectionID, err := strconv.Atoi(c.Param("sectionId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid section ID"})
		return
	}
	var req MoveSectionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Error binding move data"})
		return
	}
	if err := h.pRepo.MoveSection(c, projectID, sectionID, req.AfterSectionID, currentUser.CircleID, currentUser.ID); err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	h.broadcastBoardUpdated(c, currentUser.CircleID, projectID, &currentUser.User)
	c.JSON(200, gin.H{"res": "ok"})
}

// MoveChoreReq places a chore into sectionId, right after afterChoreId, or
// first when afterChoreId is omitted.
type MoveChoreReq struct {
	SectionID    int  `json:"sectionId" binding:"required"`
	AfterChoreID *int `json:"afterChoreId"`
}

// moveChore godoc
//
//	@Summary		Move a chore within a project's board
//	@Description	Places one chore into sectionId, sorted right after afterChoreId (first, when omitted), moving it in from a different section of the same project if needed. Computed from the current server-side order of that section, so two users dragging different cards at the same time cannot clobber each other.
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			id		path		int					true	"Project ID"
//	@Param			choreId	path		int					true	"Chore ID"
//	@Param			move	body		MoveChoreReq		true	"Destination section and chore to place it after"
//	@Success		200		{object}	map[string]string	"res: ok"
//	@Failure		404		{object}	map[string]string	"error: project, section or chore not found"
//	@Router			/projects/{id}/board/chores/{choreId}/move [put]
func (h *Handler) moveChore(c *gin.Context) {
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
	choreID, err := strconv.Atoi(c.Param("choreId"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}
	var req MoveChoreReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Error binding move data"})
		return
	}
	if err := h.pRepo.MoveChore(c, projectID, choreID, req.SectionID, req.AfterChoreID, currentUser.CircleID, currentUser.ID); err != nil {
		c.JSON(statusFor(err), gin.H{"error": err.Error()})
		return
	}
	h.broadcastBoardUpdated(c, currentUser.CircleID, projectID, &currentUser.User)
	c.JSON(200, gin.H{"res": "ok"})
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

		projectRoutes.GET("/:id/sections", h.getSections)
		projectRoutes.POST("/:id/sections", h.createSection)
		projectRoutes.PUT("/:id/sections/:sectionId", h.renameSection)
		projectRoutes.PUT("/:id/sections/:sectionId/move", h.moveSection)
		projectRoutes.DELETE("/:id/sections/:sectionId", h.deleteSection)

		projectRoutes.PUT("/:id/board/chores/:choreId/move", h.moveChore)
	}
}
