package thing

import (
	"strconv"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	authMiddleware "donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	tModel "donetick.com/core/internal/thing/model"
	tRepo "donetick.com/core/internal/thing/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type API struct {
	choreRepo  *chRepo.ChoreRepository
	circleRepo *cRepo.CircleRepository
	thingRepo  *tRepo.ThingRepository
	userRepo   *uRepo.UserRepository
	tRepo      *tRepo.ThingRepository
}

func NewAPI(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	thingRepo *tRepo.ThingRepository, userRepo *uRepo.UserRepository, tRepo *tRepo.ThingRepository) *API {
	return &API{
		choreRepo:  cr,
		circleRepo: circleRepo,
		thingRepo:  thingRepo,
		userRepo:   userRepo,
		tRepo:      tRepo,
	}
}

func (h *API) UpdateThingState(c *gin.Context) {
	thing, shouldReturn := validateUserAndThing(c, h)
	if shouldReturn {
		return
	}

	state := c.Query("state")
	if state == "" {
		c.JSON(400, gin.H{"error": "Invalid state value"})
		return
	}

	thing.State = state
	if !isValidThingState(thing) {
		c.JSON(400, gin.H{"error": "Invalid state for thing"})
		return
	}

	if err := h.thingRepo.UpdateThingState(c, thing); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{})
}

func (h *API) ChangeThingState(c *gin.Context) {
	thing, shouldReturn := validateUserAndThing(c, h)
	if shouldReturn {
		return
	}
	addRemoveRaw := c.Query("op")
	setRaw := c.Query("set")

	if addRemoveRaw == "" && setRaw == "" {
		c.JSON(400, gin.H{"error": "Invalid increment value"})
		return
	}
	var xValue int
	var err error
	if addRemoveRaw != "" {
		xValue, err = strconv.Atoi(addRemoveRaw)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid increment value"})
			return
		}
		currentState, err := strconv.Atoi(thing.State)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid state for thing"})
			return
		}
		newState := currentState + xValue
		thing.State = strconv.Itoa(newState)
	}
	if setRaw != "" {
		thing.State = setRaw
	}

	if !isValidThingState(thing) {
		c.JSON(400, gin.H{"error": "Invalid state for thing"})
		return
	}
	if err := h.thingRepo.UpdateThingState(c, thing); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	shouldReturn1 := WebhookEvaluateTriggerAndScheduleDueDate(h, c, thing)
	if shouldReturn1 {
		return
	}

	c.JSON(200, gin.H{"state": thing.State})
}

func WebhookEvaluateTriggerAndScheduleDueDate(h *API, c *gin.Context, thing *tModel.Thing) bool {
	// handler should be interface to not duplicate both WebhookEvaluateTriggerAndScheduleDueDate and EvaluateTriggerAndScheduleDueDate
	// this is bad code written Saturday at 2:25 AM

	log := logging.FromContext(c)

	thingChores, err := h.tRepo.GetThingChoresByThingId(c, thing.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return true
	}
	for _, tc := range thingChores {
		triggered := EvaluateThingChore(tc, thing.State)
		if triggered {
			errSave := h.choreRepo.SetDueDate(c, tc.ChoreID, time.Now().UTC())
			if errSave != nil {
				log.Error("Error setting due date for chore ", errSave)
				log.Error("Chore ID ", tc.ChoreID, " Thing ID ", thing.ID, " State ", thing.State)
			}
		}

	}
	return false
}

func validateUserAndThing(c *gin.Context, h *API) (*tModel.Thing, bool) {
	thingID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return nil, true
	}
	user := auth.MustCurrentUser(c)
	thing, err := h.thingRepo.GetThingByID(c, thingID)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid thing id"})
		return nil, true
	}
	if thing.UserID != user.ID {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return nil, true
	}
	return thing, false
}

func APIs(cfg *config.Config, w *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, userRepo *uRepo.UserRepository) {

	thingsAPI := r.Group("eapi/v1/things")

	thingsAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		thingsAPI.GET("/:id/state/change", w.ChangeThingState)
		thingsAPI.GET("/:id/state", w.UpdateThingState)
		thingsAPI.GET("/:id", w.GetThingByID)
		thingsAPI.GET("/", w.GetAllThings)
	}

}

// GetThingByID returns a single thing by its ID for the authenticated user
func (h *API) GetThingByID(c *gin.Context) {
	thing, shouldReturn := validateUserAndThing(c, h)
	if shouldReturn {
		return
	}
	c.JSON(200, gin.H{"thing": thing})
}

// GetAllThings returns all things for the authenticated user
func (h *API) GetAllThings(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	things, err := h.thingRepo.GetThingsByUserID(c, user.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, things)
}
