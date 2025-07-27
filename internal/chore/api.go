package chore

import (
	"strconv"
	"time"

	"donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/events"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
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

	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	user, err := h.userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	chores, err := h.choreRepo.GetChores(c, user.CircleID, user.ID, false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chores)
}

func (h *API) CreateChore(c *gin.Context) {
	var choreRequest chModel.ChoreReq

	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	user, err := h.userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

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
	}

	_, err = h.choreRepo.CreateChore(c, chore)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chore)

}

func (h *API) CompleteChore(c *gin.Context) {
	log := logging.FromContext(c)
	completedDate := time.Now().UTC()
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.CompleteChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	completedByRaw := c.Query("completedBy")
	completedBy, err := strconv.Atoi(completedByRaw)

	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		log.Debugw("chore.api.CompleteChore no secret key provided")
		c.JSON(401, gin.H{"error": "No secret key provided"})
		return
	}
	currentUser, err := h.userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	performer := currentUser.ID
	if completedBy != 0 {
		log.Debugw("chore.api.CompleteChore completedBy is set", "completedBy", completedBy)
		performer = completedBy
	}
	chore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		log.Errorw("chore.api.CompleteChore failed to get chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	if !chore.CanComplete(performer) {
		log.Debugw("chore.api.CompleteChore user is not assigned to chore", "userID", performer, "choreID", choreID)
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}

	// confirm that the chore in completion window:
	if chore.CompletionWindow != nil {
		if completedDate.Before(chore.NextDueDate.Add(time.Hour * time.Duration(*chore.CompletionWindow))) {
			log.Debugw("chore.api.CompleteChore chore is in completion window", "choreID", choreID, "completionWindow", chore.CompletionWindow)
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
			log.Debugw("chore.api.CompleteChore failed to schedule adaptive next due date", "error", err)

			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}

	} else {
		nextDueDate, err = scheduleNextDueDate(c, chore, completedDate.UTC())
		if err != nil {
			log.Debugw("chore.api.CompleteChore failed to schedule next due date", "error", err)
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

	nextAssignedTo, err := checkNextAssignee(chore, choreHistory, performer)
	if err != nil {
		log.Debugw("chore.api.CompleteChore failed to check next assignee", "error", err)
		c.JSON(500, gin.H{
			"error": "Error checking next assignee",
		})
		return
	}

	if err := h.choreRepo.CompleteChore(c, chore, nil, performer, nextDueDate, &completedDate, nextAssignedTo, true); err != nil {
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

func (h *API) GetCircleMembers(c *gin.Context) {
	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	currentUser, err := h.userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}
	users, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	if len(users) == 0 {
		c.JSON(404, gin.H{"error": "No members found in the circle"})
		return
	}
	c.JSON(200, users)
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {

	tasksAPI := r.Group("eapi/v1/chore")

	// tasksAPI.Use(utils.TimeoutMiddleware(cfg.Server.WriteTimeout), utils.RateLimitMiddleware(limiter))
	{
		tasksAPI.GET("", api.GetAllChores)
		tasksAPI.POST("/:id/complete", api.CompleteChore)
		// tasksAPI.POST("", api.CreateChore)
		// tasksAPI.PUT(":id", api.UpdateChore)
	}

	circleAPI := r.Group("eapi/v1/circle")
	circleAPI.Use(utils.TimeoutMiddleware(cfg.Server.WriteTimeout), utils.RateLimitMiddleware(limiter))
	{
		circleAPI.GET("/members", api.GetCircleMembers)
	}

}
