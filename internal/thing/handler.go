package thing

import (
	"strconv"
	"time"

	auth "donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/events"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	tModel "donetick.com/core/internal/thing/model"
	tRepo "donetick.com/core/internal/thing/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	choreRepo      *chRepo.ChoreRepository
	circleRepo     *cRepo.CircleRepository
	nPlanner       *nps.NotificationPlanner
	nRepo          *nRepo.NotificationRepository
	tRepo          *tRepo.ThingRepository
	uRepo          *uRepo.UserRepository
	eventsProducer *events.EventsProducer
}

type ThingRequest struct {
	ID    int    `json:"id"`
	Name  string `json:"name" binding:"required"`
	Type  string `json:"type" binding:"required"`
	State string `json:"state"`
}

func NewHandler(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	np *nps.NotificationPlanner, nRepo *nRepo.NotificationRepository, tRepo *tRepo.ThingRepository, eventsProducer *events.EventsProducer) *Handler {
	return &Handler{
		choreRepo:      cr,
		circleRepo:     circleRepo,
		nPlanner:       np,
		nRepo:          nRepo,
		tRepo:          tRepo,
		eventsProducer: eventsProducer,
	}
}

// CreateThing godoc
//
//	@Summary		Create a new thing
//	@Description	Creates a new thing for the current user with the given name, type, and optional state
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			thing	body		ThingRequest			true	"Thing creation request"
//	@Success		201		{object}	map[string]model.Thing	"res: created thing object"
//	@Failure		400		{object}	map[string]string		"error: Invalid request | Invalid state"
//	@Failure		401		{object}	map[string]string		"error: Unauthorized"
//	@Failure		500		{object}	map[string]string		"error: Failed to create thing"
//	@Router			/things [post]
func (h *Handler) CreateThing(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var req ThingRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	thing := &tModel.Thing{
		Name:   req.Name,
		UserID: currentUser.ID,
		Type:   req.Type,
		State:  req.State,
	}
	if !isValidThingState(thing) {
		c.JSON(400, gin.H{"error": "Invalid state"})
		return
	}
	log.Debug("Creating thing", thing)
	if err := h.tRepo.UpsertThing(c, thing); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{
		"res": thing,
	})
}

// UpdateThingState godoc
//
//	@Summary		Update thing state
//	@Description	Updates the state of a thing by ID and triggers any associated chore due dates
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int					true	"Thing ID"
//	@Param			value	query		string				true	"New state value"
//	@Success		200		{object}	map[string]model.Thing	"res: updated thing object"
//	@Failure		400		{object}	map[string]string		"error: Invalid thing id | state or increment query param is required | Invalid state"
//	@Failure		401		{object}	map[string]string		"error: Unauthorized"
//	@Failure		403		{object}	map[string]string		"error: Forbidden"
//	@Failure		500		{object}	map[string]string		"error: Unable to find thing | Failed to update state"
//	@Router			/things/{id}/state [put]
func (h *Handler) UpdateThingState(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid thing id"})
		return
	}

	val := c.Query("value")
	if val == "" {
		c.JSON(400, gin.H{"error": "state or increment query param is required"})
		return
	}
	thing, err := h.tRepo.GetThingByID(c, thingID)
	old_state := thing.State
	if thing.UserID != currentUser.ID {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to find thing"})
		return
	}
	thing.State = val
	if !isValidThingState(thing) {
		c.JSON(400, gin.H{"error": "Invalid state"})
		return
	}

	if err := h.tRepo.UpdateThingState(c, thing); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	shouldReturn := EvaluateTriggerAndScheduleDueDate(h, c, thing)
	if shouldReturn {
		return
	}
	h.eventsProducer.ThingsUpdated(c.Request.Context(), currentUser.WebhookURL, map[string]interface{}{
		"id":         thing.ID,
		"name":       thing.Name,
		"type":       thing.Type,
		"from_state": old_state,
		"to_state":   val,
	})

	c.JSON(200, gin.H{
		"res": thing,
	})
}

func EvaluateTriggerAndScheduleDueDate(h *Handler, c *gin.Context, thing *tModel.Thing) bool {
	thingChores, err := h.tRepo.GetThingChoresByThingId(c, thing.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return true
	}
	for _, tc := range thingChores {
		triggered := EvaluateThingChore(tc, thing.State)
		if triggered {
			err := h.choreRepo.SetDueDateIfNotExisted(c, tc.ChoreID, time.Now().UTC())
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return true
			}
		}
	}
	return false
}

// UpdateThing godoc
//
//	@Summary		Update a thing
//	@Description	Updates the name, type, and optionally the state of an existing thing
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			thing	body		ThingRequest			true	"Thing update request"
//	@Success		200		{object}	map[string]model.Thing	"res: updated thing object"
//	@Failure		400		{object}	map[string]string		"error: Invalid request | Invalid state"
//	@Failure		401		{object}	map[string]string		"error: Unauthorized"
//	@Failure		403		{object}	map[string]string		"error: Forbidden"
//	@Failure		500		{object}	map[string]string		"error: Unable to find thing | Failed to update thing"
//	@Router			/things [put]
func (h *Handler) UpdateThing(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var req ThingRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	thing, err := h.tRepo.GetThingByID(c, req.ID)

	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to find thing"})
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	thing.Name = req.Name
	thing.Type = req.Type
	if req.State != "" {
		thing.State = req.State
		if !isValidThingState(thing) {
			c.JSON(400, gin.H{"error": "Invalid state"})
			return
		}
	}

	if err := h.tRepo.UpsertThing(c, thing); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"res": thing,
	})
}

// GetAllThings godoc
//
//	@Summary		Get all things
//	@Description	Retrieves all things belonging to the current user
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{object}	map[string][]model.Thing	"res: array of things"
//	@Failure		401	{object}	map[string]string			"error: Unauthorized"
//	@Failure		500	{object}	map[string]string			"error: Failed to retrieve things"
//	@Router			/things [get]
func (h *Handler) GetAllThings(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	things, err := h.tRepo.GetUserThings(c, currentUser.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"res": things,
	})
}

// GetThingHistory godoc
//
//	@Summary		Get thing history
//	@Description	Retrieves the state change history for a specific thing with pagination offset
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int						true	"Thing ID"
//	@Param			offset	query		int						true	"Pagination offset"
//	@Success		200		{object}	map[string]interface{}	"res: thing history entries"
//	@Failure		400		{object}	map[string]string		"error: Invalid thing id | Invalid offset"
//	@Failure		401		{object}	map[string]string		"error: Unauthorized"
//	@Failure		403		{object}	map[string]string		"error: Forbidden"
//	@Failure		500		{object}	map[string]string		"error: Unable to find thing | Failed to retrieve history"
//	@Router			/things/{id}/history [get]
func (h *Handler) GetThingHistory(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid thing id"})
		return
	}

	thing, err := h.tRepo.GetThingByID(c, thingID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to find thing"})
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	offsetRaw := c.Query("offset")
	offset, err := strconv.Atoi(offsetRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid offset"})
		return
	}

	history, err := h.tRepo.GetThingHistoryWithOffset(c, thingID, offset)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"res": history,
	})
}

// DeleteThing godoc
//
//	@Summary		Delete a thing
//	@Description	Deletes a thing by ID; fails if there are chores still associated with it
//	@Tags			things
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Thing ID"
//	@Success		200	{object}	map[string]interface{}	"empty response on success"
//	@Failure		400	{object}	map[string]string		"error: Invalid thing id"
//	@Failure		401	{object}	map[string]string		"error: Unauthorized"
//	@Failure		403	{object}	map[string]string		"error: Forbidden"
//	@Failure		405	{object}	map[string]string		"error: Unable to delete thing with associated tasks"
//	@Failure		500	{object}	map[string]string		"error: Unable to find thing | Unable to find tasks linked to this thing | Failed to delete thing"
//	@Router			/things/{id} [delete]
func (h *Handler) DeleteThing(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid thing id"})
		return
	}

	thing, err := h.tRepo.GetThingByID(c, thingID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to find thing"})
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(403, gin.H{"error": "Forbidden"})
		return
	}
	thingChores, err := h.tRepo.GetThingChoresByThingId(c, thing.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to find tasks linked to this thing"})
		return
	}
	if len(thingChores) > 0 {
		c.JSON(405, gin.H{"error": "Unable to delete thing with associated tasks"})
		return
	}
	if err := h.tRepo.DeleteThing(c, thingID); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{})
}

func Routes(r *gin.Engine, h *Handler, ginJWTMiddleware *jwt.GinJWTMiddleware) {
	thingRoutes := r.Group("api/v1/things")
	thingRoutes.Use(auth.MultiAuthMiddleware(ginJWTMiddleware, h.uRepo))
	{
		thingRoutes.POST("", h.CreateThing)
		thingRoutes.PUT("/:id/state", h.UpdateThingState)
		thingRoutes.PUT("", h.UpdateThing)
		thingRoutes.GET("", h.GetAllThings)
		thingRoutes.GET("/:id/history", h.GetThingHistory)
		thingRoutes.DELETE("/:id", h.DeleteThing)
	}
}
