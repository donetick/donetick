package thing

import (
	"strconv"
	"time"

	auth "donetick.com/core/internal/authorization"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	tModel "donetick.com/core/internal/thing/model"
	tRepo "donetick.com/core/internal/thing/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	choreRepo  *chRepo.ChoreRepository
	circleRepo *cRepo.CircleRepository
	nPlanner   *nps.NotificationPlanner
	nRepo      *nRepo.NotificationRepository
	tRepo      *tRepo.ThingRepository
}

type ThingRequest struct {
	ID    int    `json:"id"`
	Name  string `json:"name" binding:"required"`
	Type  string `json:"type" binding:"required"`
	State string `json:"state"`
}

func NewHandler(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	np *nps.NotificationPlanner, nRepo *nRepo.NotificationRepository, tRepo *tRepo.ThingRepository) *Handler {
	return &Handler{
		choreRepo:  cr,
		circleRepo: circleRepo,
		nPlanner:   np,
		nRepo:      nRepo,
		tRepo:      tRepo,
	}
}

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
			h.choreRepo.SetDueDateIfNotExisted(c, tc.ChoreID, time.Now().UTC())
		}
	}
	return false
}

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
	//  confirm there are no chores associated with the thing:
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
