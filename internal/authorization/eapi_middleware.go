package auth

import (
	cRepo "donetick.com/core/internal/circle/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"github.com/gin-gonic/gin"
	"log"
	"strconv"
)

var AuthorizationHeader = "secretKey"
var ImpersonationHeader = "actionUserId"

func GetUserFromTokenRequest(c *gin.Context, userRepo *uRepo.UserRepository, circleRepo *cRepo.CircleRepository) (*uModel.UserDetails, bool) {
	apiToken := c.GetHeader(AuthorizationHeader)
	if apiToken == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return nil, false
	}

	user, err := userRepo.GetUserByToken(c, apiToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return nil, false
	}

	userToImpersonateStr := c.GetHeader(ImpersonationHeader)
	if userToImpersonateStr != "" {
		userRole, err := circleRepo.GetUserRoleInCircle(c, user.ID, user.CircleID)
		if err != nil {
			log.Printf("Error getting user role in circle: %s", err)
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return nil, false
		}
		if *userRole != "admin" {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return nil, false
		}

		userIdToImpersonate, err := strconv.Atoi(userToImpersonateStr)
		if err != nil {
			log.Printf("Error impersonating user by id: %s", err)
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return nil, false
		}
		originatingUser := user
		user, err = userRepo.GetUser(c, userIdToImpersonate)
		if err != nil {
			log.Printf("Error impersonating user by username: %s", err)
			c.JSON(401, gin.H{"error": "Unauthorized"})
			return nil, false
		}
		log.Printf(
			"User %s (%d) made an impersonated call as %s (%d)",
			originatingUser.Username, originatingUser.ID, user.Username, user.ID,
		)
	}
	return user, true
}

func TokenValidation(userRepo *uRepo.UserRepository, circleRepo *cRepo.CircleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, isOk := GetUserFromTokenRequest(c, userRepo, circleRepo)
		if !isOk {
			c.Abort()
			return
		}
		c.Set(identityKey, user)
		c.Next()
	}
}
