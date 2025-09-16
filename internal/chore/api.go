package chore

import (
	"strconv"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	authMiddleware "donetick.com/core/internal/auth"
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
	user := auth.MustCurrentUser(c)
	chores, err := h.choreRepo.GetChores(c, user.CircleID, user.ID, false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chores)
}

func (h *API) CreateChore(c *gin.Context) {
	log := logging.FromContext(c)
	var choreRequest chModel.ChoreLiteReq
	user := auth.MustCurrentUser(c)

	if err := c.BindJSON(&choreRequest); err != nil {
		log.Debugw("chore.api.CreateChore failed to bind JSON", "error", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate required fields
	if choreRequest.Name == "" {
		c.JSON(400, gin.H{"error": "Chore name is required"})
		return
	}

	// Parse due date if provided
	var nextDueDate *time.Time
	if choreRequest.DueDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, choreRequest.DueDate)
		if err != nil {
			parsedDateSimple, errSimple := time.Parse("2006-01-02", choreRequest.DueDate)
			if errSimple != nil {
				c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 or YYYY-MM-DD"})
				return
			}
			// Set time to now UTC
			now := time.Now().UTC()
			parsedDate = time.Date(parsedDateSimple.Year(), parsedDateSimple.Month(), parsedDateSimple.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
			err = nil
		}
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 format"})
			return
		}
		nextDueDate = &parsedDate
	}
	// get all circle members:
	circleUsers, err := h.circleRepo.GetCircleUsers(c, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to get circle users", "error", err)
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	createdBy := user.ID
	if choreRequest.CreatedBy != nil {
		// Check if the specified user exists in the circle
		var found bool
		for _, u := range circleUsers {
			if u.UserID == *choreRequest.CreatedBy {
				found = true
				createdBy = u.UserID
				break
			}
		}
		if !found {
			log.Errorw("chore.api.CreateChore specified user not found in circle", "userID", *choreRequest.CreatedBy)
			c.JSON(400, gin.H{"error": "Specified user not found in circle"})
			return
		}
	}

	chore := &chModel.Chore{
		CreatedBy:     createdBy,
		CircleID:      user.CircleID,
		Name:          choreRequest.Name,
		IsActive:      true,
		FrequencyType: chModel.FrequencyTypeOnce,
		// Frequency:                choreRequest.Frequency,
		AssignStrategy: chModel.AssignmentStrategyRandom,
		AssignedTo:     createdBy,
		Assignees:      []chModel.ChoreAssignees{{UserID: createdBy}},
		Description:    choreRequest.Description,
		NextDueDate:    nextDueDate,
		CreatedAt:      time.Now().UTC(),
	}

	id, err := h.choreRepo.CreateChore(c, chore)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to create chore", "error", err)
		c.JSON(500, gin.H{"error": "Error creating chore"})
		return
	}

	// Fetch the created chore with all relations
	createdChore, err := h.choreRepo.GetChore(c, id, user.ID)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to fetch created chore", "error", err)
		c.JSON(500, gin.H{"error": "Error fetching created chore"})
		return
	}

	c.JSON(201, createdChore)
}

func (h *API) UpdateChore(c *gin.Context) {
	log := logging.FromContext(c)
	var choreRequest chModel.ChoreLiteReq
	user := auth.MustCurrentUser(c)

	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.UpdateChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}

	if err := c.BindJSON(&choreRequest); err != nil {
		log.Debugw("chore.api.UpdateChore failed to bind JSON", "error", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing chore
	existingChore, err := h.choreRepo.GetChore(c, choreID, user.ID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to get chore", "error", err)
		c.JSON(404, gin.H{"error": "Chore not found"})
		return
	}
	// get circle members:
	circleUsers, err := h.circleRepo.GetCircleUsers(c, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to get circle users", "error", err)
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	// Check if user owns this chore
	now := time.Now().UTC()
	if err := existingChore.CanEdit(user.ID, circleUsers, &now); err != nil {
		log.Debugw("chore.api.UpdateChore user does not own chore", "userID", user.ID, "choreCreatedBy", existingChore.CreatedBy)
		c.JSON(403, gin.H{"error": "You can only update your own chores"})
		return
	}

	// Validate required fields
	if choreRequest.Name == "" {
		c.JSON(400, gin.H{"error": "Chore name is required"})
		return
	}

	// Parse due date if provided
	var nextDueDate *time.Time
	if choreRequest.DueDate != "" {

		parsedDate, err := time.Parse(time.RFC3339, choreRequest.DueDate)
		if err != nil {
			parsedDateSimple, errSimple := time.Parse("2006-01-02", choreRequest.DueDate)
			if errSimple != nil {
				c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 or YYYY-MM-DD"})
				return
			}
			// Set time to now UTC
			now := time.Now().UTC()
			parsedDate = time.Date(parsedDateSimple.Year(), parsedDateSimple.Month(), parsedDateSimple.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
			err = nil
		}
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 format"})
			return
		}
		nextDueDate = &parsedDate
	}

	// Update only name and due date
	updates := map[string]interface{}{
		"name":          choreRequest.Name,
		"description":   choreRequest.Description,
		"next_due_date": nextDueDate,
		"updated_by":    user.ID,
		"updated_at":    time.Now().UTC(),
	}

	err = h.choreRepo.UpdateChoreFields(c, choreID, updates)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to update chore", "error", err)
		c.JSON(500, gin.H{"error": "Error updating chore"})
		return
	}

	// Fetch the updated chore
	updatedChore, err := h.choreRepo.GetChore(c, choreID, user.ID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to fetch updated chore", "error", err)
		c.JSON(500, gin.H{"error": "Error fetching updated chore"})
		return
	}

	c.JSON(200, updatedChore)
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

	currentUser := auth.MustCurrentUser(c)
	performer := currentUser.ID
	if completedBy != 0 {
		log.Debugw("chore.api.CompleteChore completedBy is set", "completedBy", completedBy)
		performer = completedBy
	}
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		log.Errorw("chore.api.CompleteChore failed to get chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Errorw("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(performer, circleUsers) {
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

	updatedChore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
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
	currentUser := auth.MustCurrentUser(c)
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
func (h *API) DeleteChore(c *gin.Context) {
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}
	currentUser := auth.MustCurrentUser(c)
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Chore not found"})
		return
	}
	if chore.CreatedBy != currentUser.ID {
		c.JSON(403, gin.H{"error": "You can only delete your own chores"})
		return
	}
	if err := h.choreRepo.DeleteChore(c, choreID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete chore"})
		return
	}
	c.JSON(200, gin.H{"message": "Chore deleted successfully"})
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter, userRepo *uRepo.UserRepository) {

	tasksAPI := r.Group("eapi/v1/chore")
	tasksAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		tasksAPI.GET("", api.GetAllChores)
		tasksAPI.POST("", api.CreateChore)
		tasksAPI.DELETE("/:id", api.DeleteChore)
	}

	// Plus member only endpoints
	tasksPlusAPI := r.Group("eapi/v1/chore")
	tasksPlusAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
		authMiddleware.RequirePlusMemberMiddleware(),
	)
	{
		tasksPlusAPI.POST("/:id/complete", api.CompleteChore)
		tasksPlusAPI.PUT("/:id", api.UpdateChore)
	}

	circleAPI := r.Group("eapi/v1/circle")
	circleAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
		authMiddleware.RequirePlusMemberMiddleware(),
	)
	{
		circleAPI.GET("/members", api.GetCircleMembers)
	}

}
