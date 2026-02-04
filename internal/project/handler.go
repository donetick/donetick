package project

import (
	"strconv"

	auth "donetick.com/core/internal/auth"
	pModel "donetick.com/core/internal/project/model"
	pRepo "donetick.com/core/internal/project/repo"
	jwt "github.com/appleboy/gin-jwt/v2"
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

func (h *Handler) getProjects(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
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

	c.JSON(200, projects)
}

func (h *Handler) createProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
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

func (h *Handler) updateProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projectIDRaw := c.Param("id")
	if projectIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Project ID is required",
		})
		return
	}

	projectID, err := strconv.Atoi(projectIDRaw)
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

	// Get updated project to return
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

func (h *Handler) deleteProject(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	projectIDRaw := c.Param("id")
	if projectIDRaw == "" {
		c.JSON(400, gin.H{
			"error": "Project ID is required",
		})
		return
	}

	projectID, err := strconv.Atoi(projectIDRaw)
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

func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {
	projectRoutes := r.Group("api/v1/projects")
	projectRoutes.Use(auth.MiddlewareFunc())
	{
		projectRoutes.GET("", h.getProjects)
		projectRoutes.POST("", h.createProject)
		projectRoutes.PUT("/:id", h.updateProject)
		projectRoutes.DELETE("/:id", h.deleteProject)
	}
}
