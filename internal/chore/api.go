package chore

import (
	"donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	"donetick.com/core/internal/utils"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	limiter "github.com/ulule/limiter/v3"

	chModel "donetick.com/core/internal/chore/model"
	uRepo "donetick.com/core/internal/user/repo"
)

type API struct {
	choreRepo *chRepo.ChoreRepository
	userRepo  *uRepo.UserRepository
}

func NewAPI(cr *chRepo.ChoreRepository, userRepo *uRepo.UserRepository) *API {
	return &API{
		choreRepo: cr,
		userRepo:  userRepo,
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

func APIs(cfg *config.Config, api *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {

	thingsAPI := r.Group("eapi/v1/chore")

	thingsAPI.Use(utils.TimeoutMiddleware(cfg.Server.WriteTimeout), utils.RateLimitMiddleware(limiter))
	{
		thingsAPI.GET("", api.GetAllChores)
	}

}
