package chore

import (
	"log"
	"strconv"
	"time"

	"donetick.com/core/config"
	dtAuth "donetick.com/core/internal/authorization"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/events"
	nps "donetick.com/core/internal/notifier/service"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	limiter "github.com/ulule/limiter/v3"

	chModel "donetick.com/core/internal/chore/model"
	cRepo "donetick.com/core/internal/circle/repo"
	stRepo "donetick.com/core/internal/subtask/repo"
	uRepo "donetick.com/core/internal/user/repo"
)

type API struct {
	choreRepo     *chRepo.ChoreRepository
	userRepo      *uRepo.UserRepository
	circleRepo    *cRepo.CircleRepository
	nPlanner      *nps.NotificationPlanner
	eventProducer *events.EventsProducer
	stRepo        *stRepo.SubTasksRepository
}

func NewAPI(cr *chRepo.ChoreRepository, userRepo *uRepo.UserRepository, circleRepo *cRepo.CircleRepository, nPlanner *nps.NotificationPlanner, eventProducer *events.EventsProducer, stRepo *stRepo.SubTasksRepository) *API {
	return &API{
		choreRepo:     cr,
		userRepo:      userRepo,
		circleRepo:    circleRepo,
		nPlanner:      nPlanner,
		eventProducer: eventProducer,
		stRepo:        stRepo,
	}
}

func (h *API) GetAllChores(c *gin.Context) {
	user, _ := dtAuth.CurrentUser(c)
	chores, err := h.choreRepo.GetChores(c, user.CircleID, user.ID, false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chores)
}

func (h *API) CreateChore(c *gin.Context) {
	var choreRequest chModel.ChoreReq

	user, _ := dtAuth.CurrentUser(c)

	if err := c.BindJSON(&choreRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	chore := &chModel.Chore{
		CreatedBy:      user.ID,
		CircleID:       user.CircleID,
		Name:           choreRequest.Name,
		IsRolling:      choreRequest.IsRolling,
		FrequencyType:  choreRequest.FrequencyType,
		Frequency:      choreRequest.Frequency,
		AssignStrategy: choreRequest.AssignStrategy,
		AssignedTo:     user.ID,
		Assignees:      []chModel.ChoreAssignees{{UserID: user.ID}},
		Description:    choreRequest.Description,
		IsActive:       true,
	}

	_, err := h.choreRepo.CreateChore(c, chore)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chore)

}

func (h *API) CompleteChore(c *gin.Context) {

	completedDate := time.Now().UTC()
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	currentUser, _ := dtAuth.CurrentUser(c)

	chore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	if !chore.CanComplete(currentUser.ID) {
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}

	// confirm that the chore in completion window:
	if chore.CompletionWindow != nil {
		if completedDate.Before(chore.NextDueDate.Add(time.Hour * time.Duration(*chore.CompletionWindow))) {
			c.JSON(400, gin.H{
				"error": "Chore is out of completion window",
			})
			return
		}
	}

	var nextDueDate *time.Time
	if chore.FrequencyType == "adaptive" {
		history, err := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 5)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting chore history",
			})
			return
		}
		nextDueDate, err = scheduleAdaptiveNextDueDate(chore, completedDate, history)
		if err != nil {
			log.Printf("Error scheduling next due date: %s", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}

	} else {
		nextDueDate, err = scheduleNextDueDate(chore, completedDate.UTC())
		if err != nil {
			log.Printf("Error scheduling next due date: %s", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}
	}
	choreHistory, err := h.choreRepo.GetChoreHistory(c, chore.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	nextAssignedTo, err := checkNextAssignee(chore, choreHistory, currentUser.ID)
	if err != nil {
		log.Printf("Error checking next assignee: %s", err)
		c.JSON(500, gin.H{
			"error": "Error checking next assignee",
		})
		return
	}

	if err := h.choreRepo.CompleteChore(c, chore, nil, currentUser.ID, nextDueDate, &completedDate, nextAssignedTo, true); err != nil {
		c.JSON(500, gin.H{
			"error": "Error completing chore",
		})
		return
	}
	if chore.SubTasks != nil && chore.FrequencyType != chModel.FrequencyTypeOnce {
		h.stRepo.ResetSubtasksCompletion(c, chore.ID)
	}

	updatedChore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	h.nPlanner.GenerateNotifications(c, updatedChore)
	h.eventProducer.ChoreCompleted(c, currentUser.WebhookURL, chore, &currentUser.User)
	c.JSON(200,
		updatedChore,
	)
}

func (h *API) UpdateChore(c *gin.Context) {
	user, _ := dtAuth.CurrentUser(c)
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	var choreRequest chModel.ChoreReq
	if err := c.BindJSON(&choreRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	chore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	chore.Name = choreRequest.Name
	chore.IsRolling = choreRequest.IsRolling
	chore.FrequencyType = choreRequest.FrequencyType
	chore.Frequency = choreRequest.Frequency
	chore.AssignStrategy = choreRequest.AssignStrategy
	chore.Description = choreRequest.Description
	chore.IsActive = choreRequest.IsActive
	chore.UpdatedBy = user.ID

	if err := h.choreRepo.UpsertChore(c, chore); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating chore",
		})
		return
	}
	c.JSON(200, chore)
}

func (h *API) DeleteChore(c *gin.Context) {
	user, _ := dtAuth.CurrentUser(c)
	chore := h.TryGetAndValidateChoreFromContext(c)
	if err := h.choreRepo.SoftDelete(c, chore.ID, user.ID); err != nil {
		c.JSON(500, gin.H{
			"error": "Error deleting chore",
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "Chore deleted",
	})
}

func (h *API) GetChore(c *gin.Context) {
	chore := h.TryGetAndValidateChoreFromContext(c)
	c.JSON(200, chore)
}

func (h *API) TryGetAndValidateChoreFromContext(c *gin.Context) *chModel.Chore {
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		c.Abort()
	}
	chore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		c.Abort()
	}
	if chore == nil {
		c.JSON(404, gin.H{
			"error": "Chore not found",
		})
		c.Abort()
	}
	return chore
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {

	choresAPI := r.Group("eapi/v1/chore")
	choresAPI.Use(dtAuth.TokenValidation(api.userRepo, api.circleRepo))

	// choresAPI.Use(utils.TimeoutMiddleware(cfg.Server.WriteTimeout), utils.RateLimitMiddleware(limiter))
	{
		choresAPI.GET("", api.GetAllChores)
		choresAPI.POST("", api.CreateChore)
		choresAPI.GET("/:id", api.GetChore)
		choresAPI.PUT("/:id", api.UpdateChore)
		choresAPI.DELETE("/:id", api.DeleteChore)
		choresAPI.POST("/:id/complete", api.CompleteChore)
	}
}
