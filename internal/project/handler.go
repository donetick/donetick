package project

import (
	"strconv"

	"donetick.com/core/internal/auth"
	pModel "donetick.com/core/internal/project/model"
	pRepo "donetick.com/core/internal/project/repo"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	pRepo *pRepo.ProjectRepository
}

func NewHandler(pRepo *pRepo.ProjectRepository) *Handler {
	return &Handler{
		pRepo: pRepo,
	}
}

// getProjects godoc
//
//	@Summary		Get all projects
//	@Description	Retrieves all projects for the current user's circle
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{object}	map[string][]model.Project	"res: array of projects"
//	@Failure		401	{object}	map[string]string			"error: Error getting current user"
//	@Failure		500	{object}	map[string]string			"error: Error getting projects"
//	@Router			/projects [get]
func (h *Handler) getProjects(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projects, err := h.pRepo.GetCircleProjects(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting projects",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": projects,
	})
}

// createProject godoc
//
//	@Summary		Create a new project
//	@Description	Creates a new project for the current user's circle
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			project	body		model.ProjectReq			true	"Project creation request"
//	@Success		200		{object}	map[string]model.Project	"res: created project object"
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

	project := &pModel.Project{
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		CircleID:    currentUser.CircleID,
		CreatedBy:   currentUser.ID,
		Icon:        req.Icon,
	}

	if err := h.pRepo.CreateProject(c, project); err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating project",
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
//	@Description	Updates the name, description, color, and icon of an existing project by ID
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int							true	"Project ID"
//	@Param			project	body		model.ProjectReq			true	"Project update request"
//	@Success		200		{object}	map[string]model.Project	"res: updated project object"
//	@Failure		400		{object}	map[string]string			"error: Invalid project ID | Error binding project data"
//	@Failure		401		{object}	map[string]string			"error: Error getting current user"
//	@Failure		500		{object}	map[string]string			"error: Error updating project | Error getting updated project"
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

	if err := h.pRepo.UpdateProject(c, project, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(500, gin.H{
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

	c.JSON(200, gin.H{
		"res": updatedProject,
	})
}

// deleteProject godoc
//
//	@Summary		Delete a project
//	@Description	Deletes a project by ID; restricted to the current user's circle
//	@Tags			projects
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
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

	if err := h.pRepo.DeleteProject(c, projectID, currentUser.ID, currentUser.CircleID); err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"res": "Project deleted successfully",
	})
}

func Routes(r *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	projectRoutes := r.Group("api/v1/projects")
	projectRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		projectRoutes.GET("", h.getProjects)
		projectRoutes.POST("", h.createProject)
		projectRoutes.PUT("/:id", h.updateProject)
		projectRoutes.DELETE("/:id", h.deleteProject)
	}
}
