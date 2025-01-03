package thing

import (
	"strconv"
	"time"

	"donetick.com/core/config"
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

type Webhook struct {
	choreRepo  *chRepo.ChoreRepository
	circleRepo *cRepo.CircleRepository
	thingRepo  *tRepo.ThingRepository
	userRepo   *uRepo.UserRepository
	tRepo      *tRepo.ThingRepository
}

func NewWebhook(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository,
	thingRepo *tRepo.ThingRepository, userRepo *uRepo.UserRepository, tRepo *tRepo.ThingRepository) *Webhook {
	return &Webhook{
		choreRepo:  cr,
		circleRepo: circleRepo,
		thingRepo:  thingRepo,
		userRepo:   userRepo,
		tRepo:      tRepo,
	}
}

func (h *Webhook) UpdateThingState(c *gin.Context) {
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

func (h *Webhook) ChangeThingState(c *gin.Context) {
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

func WebhookEvaluateTriggerAndScheduleDueDate(h *Webhook, c *gin.Context, thing *tModel.Thing) bool {
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

func validateUserAndThing(c *gin.Context, h *Webhook) (*tModel.Thing, bool) {
	apiToken := c.GetHeader("secretkey")
	if apiToken == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return nil, true
	}
	thingID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return nil, true
	}
	user, err := h.userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return nil, true
	}
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

func Webhooks(cfg *config.Config, w *Webhook, r *gin.Engine, auth *jwt.GinJWTMiddleware) {

	thingsAPI := r.Group("eapi/v1/things")

	thingsAPI.Use(utils.TimeoutMiddleware(cfg.Server.WriteTimeout))
	{
		thingsAPI.GET("/:id/state/change", w.ChangeThingState)
		thingsAPI.GET("/:id/state", w.UpdateThingState)
	}

}
