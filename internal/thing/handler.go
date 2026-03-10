package thing

import (
	"net/http"
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

func (h *Handler) CreateThing(c *gin.Context) {
	log := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	var req ThingRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	thing := &tModel.Thing{
		Name:   req.Name,
		UserID: currentUser.ID,
		Type:   req.Type,
		State:  req.State,
	}
	if !isValidThingState(thing) {
		c.JSON(http.StatusBadRequest, "Invalid state")
		return
	}
	log.Debug("Creating thing", thing)
	if err := h.tRepo.UpsertThing(c, thing); err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, thing)
}

func (h *Handler) UpdateThingState(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, "Invalid thing id")
		return
	}

	val := c.Query("value")
	if val == "" {
		c.JSON(http.StatusBadRequest, "state or increment query param is required")
		return
	}
	thing, err := h.tRepo.GetThingByID(c, thingID)
	old_state := thing.State
	if thing.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Unable to find thing")
		return
	}
	thing.State = val
	if !isValidThingState(thing) {
		c.JSON(http.StatusBadRequest, "Invalid state")
		return
	}

	if err := h.tRepo.UpdateThingState(c, thing); err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
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

	c.JSON(http.StatusOK, thing)
}

func EvaluateTriggerAndScheduleDueDate(h *Handler, c *gin.Context, thing *tModel.Thing) bool {
	thingChores, err := h.tRepo.GetThingChoresByThingId(c, thing.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return true
	}
	for _, tc := range thingChores {
		triggered := EvaluateThingChore(tc, thing.State)
		if triggered {
			err := h.choreRepo.SetDueDateIfNotExisted(c, tc.ChoreID, time.Now().UTC())
			if err != nil {
				c.JSON(http.StatusInternalServerError, err.Error())
				return true
			}
		}
	}
	return false
}

func (h *Handler) UpdateThing(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	var req ThingRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	thing, err := h.tRepo.GetThingByID(c, req.ID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, "Unable to find thing")
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}
	thing.Name = req.Name
	thing.Type = req.Type
	if req.State != "" {
		thing.State = req.State
		if !isValidThingState(thing) {
			c.JSON(http.StatusBadRequest, "Invalid state")
			return
		}
	}

	if err := h.tRepo.UpsertThing(c, thing); err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, thing)
}

func (h *Handler) GetAllThings(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	things, err := h.tRepo.GetUserThings(c, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, things)
}

func (h *Handler) GetThingHistory(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, "Invalid thing id")
		return
	}

	thing, err := h.tRepo.GetThingByID(c, thingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Unable to find thing")
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}
	offsetRaw := c.Query("offset")
	offset, err := strconv.Atoi(offsetRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, "Invalid offset")
		return
	}

	history, err := h.tRepo.GetThingHistoryWithOffset(c, thingID, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, history)
}

func (h *Handler) DeleteThing(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusForbidden, "Unauthorized")
		return
	}

	thingIDRaw := c.Param("id")
	thingID, err := strconv.Atoi(thingIDRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, "Invalid thing id")
		return
	}

	thing, err := h.tRepo.GetThingByID(c, thingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Unable to find thing")
		return
	}
	if thing.UserID != currentUser.ID {
		c.JSON(http.StatusForbidden, "Forbidden")
		return
	}
	//  confirm there are no chores associated with the thing:
	thingChores, err := h.tRepo.GetThingChoresByThingId(c, thing.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, "Unable to find tasks linked to this thing")
		return
	}
	if len(thingChores) > 0 {
		c.JSON(http.StatusMethodNotAllowed, "Unable to delete thing with associated tasks")
		return
	}
	if err := h.tRepo.DeleteThing(c, thingID); err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
func Routes(r *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {

	thingRoutes := r.Group("api/v1/things")
	thingRoutes.Use(auth.MiddlewareFunc())
	{
		thingRoutes.POST("", h.CreateThing)
		thingRoutes.PUT("/:id/state", h.UpdateThingState)
		thingRoutes.PUT("", h.UpdateThing)
		thingRoutes.GET("", h.GetAllThings)
		thingRoutes.GET("/:id/history", h.GetThingHistory)
		thingRoutes.DELETE("/:id", h.DeleteThing)
	}
}
