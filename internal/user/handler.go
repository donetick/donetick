package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"time"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/authorization"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/email"
	nModel "donetick.com/core/internal/notifier/model"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/oauth2/v1"
)

type Handler struct {
	userRepo               *uRepo.UserRepository
	circleRepo             *cRepo.CircleRepository
	jwtAuth                *jwt.GinJWTMiddleware
	email                  *email.EmailSender
	isDonetickDotCom       bool
	IsUserCreationDisabled bool
}

func NewHandler(ur *uRepo.UserRepository, cr *cRepo.CircleRepository, jwtAuth *jwt.GinJWTMiddleware, email *email.EmailSender, config *config.Config) *Handler {
	return &Handler{
		userRepo:               ur,
		circleRepo:             cr,
		jwtAuth:                jwtAuth,
		email:                  email,
		isDonetickDotCom:       config.IsDoneTickDotCom,
		IsUserCreationDisabled: config.IsUserCreationDisabled,
	}
}

func (h *Handler) GetAllUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUser, ok := auth.CurrentUser(c)
		if !ok {
			c.JSON(500, gin.H{
				"error": "Error getting current user",
			})
			return
		}

		users, err := h.userRepo.GetAllUsers(c, currentUser.CircleID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting users",
			})
			return
		}

		c.JSON(200, gin.H{
			"res": users,
		})
	}
}

func (h *Handler) signUp(c *gin.Context) {
	if h.IsUserCreationDisabled {
		c.JSON(403, gin.H{
			"error": "User creation is disabled",
		})
		return
	}

	type SignUpReq struct {
		Username    string `json:"username" binding:"required,min=4,max=20"`
		Password    string `json:"password" binding:"required,min=8,max=45"`
		Email       string `json:"email" binding:"required,email"`
		DisplayName string `json:"displayName"`
	}
	var signupReq SignUpReq
	if err := c.BindJSON(&signupReq); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	if signupReq.DisplayName == "" {
		signupReq.DisplayName = signupReq.Username
	}
	password, err := auth.EncodePassword(signupReq.Password)
	signupReq.Username = html.EscapeString(signupReq.Username)
	signupReq.DisplayName = html.EscapeString(signupReq.DisplayName)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error encoding password",
		})
		return
	}
	var insertedUser *uModel.User
	if insertedUser, err = h.userRepo.CreateUser(c, &uModel.User{
		Username:    signupReq.Username,
		Password:    password,
		DisplayName: signupReq.DisplayName,
		Email:       signupReq.Email,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating user, email already exists or username is taken",
		})
		return
	}
	// var userCircle *circle.Circle
	// var userRole string
	userCircle, err := h.circleRepo.CreateCircle(c, &cModel.Circle{
		Name:       signupReq.DisplayName + "'s circle",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		InviteCode: utils.GenerateInviteCode(c),
	})

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating circle",
		})
		return
	}

	if err := h.circleRepo.AddUserToCircle(c, &cModel.UserCircle{
		UserID:    insertedUser.ID,
		CircleID:  userCircle.ID,
		Role:      "admin",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		c.JSON(500, gin.H{
			"error": "Error adding user to circle",
		})
		return
	}
	insertedUser.CircleID = userCircle.ID
	if err := h.userRepo.UpdateUser(c, insertedUser); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating user",
		})
		return
	}

	c.JSON(201, gin.H{})
}

func (h *Handler) GetUserProfile(c *gin.Context) {
	user, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting user",
		})
		return
	}
	c.JSON(200, gin.H{
		"res": user,
	})
}

func (h *Handler) thirdPartyAuthCallback(c *gin.Context) {

	// read :provider from path param, if param is google check the token with google if it's valid and fetch the user details:
	logger := logging.FromContext(c)
	provider := c.Param("provider")
	logger.Infow("account.handler.thirdPartyAuthCallback", "provider", provider)

	if provider == "google" {
		c.Set("auth_provider", "3rdPartyAuth")
		type OAuthRequest struct {
			Token    string `json:"token" binding:"required"`
			Provider string `json:"provider" binding:"required"`
		}
		var body OAuthRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback failed to bind", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request",
			})
			return
		}

		// logger.Infow("account.handler.thirdPartyAuthCallback", "token", token)
		service, err := oauth2.New(http.DefaultClient)

		// tokenInfo, err := service.Tokeninfo().AccessToken(token).Do()
		userinfo, err := service.Userinfo.Get().Do(googleapi.QueryParameter("access_token", body.Token))
		logger.Infow("account.handler.thirdPartyAuthCallback", "tokenInfo", userinfo)
		if err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback failed to get token info", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid token",
			})
			return
		}

		acc, err := h.userRepo.FindByEmail(c, userinfo.Email)

		if err != nil {
			// create a random password for the user using crypto/rand:
			password := auth.GenerateRandomPassword(12)
			encodedPassword, err := auth.EncodePassword(password)
			acc = &uModel.User{
				Username:    userinfo.Id,
				Email:       userinfo.Email,
				Image:       userinfo.Picture,
				Password:    encodedPassword,
				DisplayName: userinfo.GivenName,
				Provider:    2,
			}
			createdUser, err := h.userRepo.CreateUser(c, acc)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Unable to create user",
				})
				return

			}
			// Create Circle for the user:
			userCircle, err := h.circleRepo.CreateCircle(c, &cModel.Circle{
				Name:       userinfo.GivenName + "'s circle",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				InviteCode: utils.GenerateInviteCode(c),
			})

			if err != nil {
				c.JSON(500, gin.H{
					"error": "Error creating circle",
				})
				return
			}

			if err := h.circleRepo.AddUserToCircle(c, &cModel.UserCircle{
				UserID:    createdUser.ID,
				CircleID:  userCircle.ID,
				Role:      "admin",
				IsActive:  true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}); err != nil {
				c.JSON(500, gin.H{
					"error": "Error adding user to circle",
				})
				return
			}
			createdUser.CircleID = userCircle.ID
			if err := h.userRepo.UpdateUser(c, createdUser); err != nil {
				c.JSON(500, gin.H{
					"error": "Error updating user",
				})
				return
			}
		}
		// use auth to generate a token for the user:
		c.Set("user_account", acc)
		h.jwtAuth.Authenticator(c)
		tokenString, expire, err := h.jwtAuth.TokenGenerator(acc)
		if err != nil {
			logger.Errorw("Unable to Generate a Token")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Unable to Generate a Token",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": tokenString, "expire": expire})
		return
	}
}

func (h *Handler) resetPassword(c *gin.Context) {
	log := logging.FromContext(c)
	type ResetPasswordReq struct {
		Email string `json:"email" binding:"required,email"`
	}
	var req ResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}
	user, err := h.userRepo.FindByEmail(c, req.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		log.Error("account.handler.resetPassword failed to find user")
		return
	}
	if user.Provider != 0 {
		// user create account thought login with Gmail. they can reset the password they just need to login with google again
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": "User account created with google login. Please login with google",
			},
		)
		return
	}
	// generate a random password:
	token, err := auth.GenerateEmailResetToken(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to generate token",
		})
		return
	}

	err = h.userRepo.SetPasswordResetToken(c, req.Email, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to generate password",
		})
		return
	}
	// send an email to the user with the new password:
	err = h.email.SendResetPasswordEmail(c, req.Email, token)
	if err != nil {
		log.Errorw("account.handler.resetPassword failed to send email", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to send email",
		})
		return
	}

	// send an email to the user with the new password:
	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) updateUserPassword(c *gin.Context) {
	logger := logging.FromContext(c)
	// read the code from query param:
	code := c.Query("c")
	email, code, err := email.DecodeEmailAndCode(code)
	if err != nil {
		logger.Errorw("account.handler.verify failed to decode email and code", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid code",
		})
		return

	}
	// read password from body:
	type RequestBody struct {
		Password string `json:"password" binding:"required,min=8,max=32"`
	}
	var body RequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		logger.Errorw("user.handler.resetAccountPassword failed to bind", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return

	}
	password, err := auth.EncodePassword(body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to process password",
		})
		return
	}

	err = h.userRepo.UpdatePasswordByToken(c.Request.Context(), email, code, password)
	if err != nil {
		logger.Errorw("account.handler.resetAccountPassword failed to reset password", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to reset password",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{})

}

func (h *Handler) UpdateUserDetails(c *gin.Context) {
	type UpdateUserReq struct {
		DisplayName *string `json:"displayName" binding:"omitempty"`
		ChatID      *int64  `json:"chatID" binding:"omitempty"`
		Image       *string `json:"image" binding:"omitempty"`
	}
	user, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting user",
		})
		return
	}
	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	// update non-nil fields:
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.ChatID != nil {
		user.ChatID = *req.ChatID
	}
	if req.Image != nil {
		user.Image = *req.Image
	}

	if err := h.userRepo.UpdateUser(c, user); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating user",
		})
		return
	}
	c.JSON(200, user)
}

func (h *Handler) CreateLongLivedToken(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}
	type TokenRequest struct {
		Name string `json:"name" binding:"required"`
	}
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Step 1: Generate a secure random number
	randomBytes := make([]byte, 16) // 128 bits are enough for strong randomness
	_, err := rand.Read(randomBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate random part of the token"})
		return
	}

	timestamp := time.Now().Unix()
	hashInput := fmt.Sprintf("%s:%d:%x", currentUser.Username, timestamp, randomBytes)
	hash := sha256.Sum256([]byte(hashInput))

	token := hex.EncodeToString(hash[:])

	tokenModel, err := h.userRepo.StoreAPIToken(c, currentUser.ID, req.Name, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store the token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"res": tokenModel})
}

func (h *Handler) GetAllUserToken(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	tokens, err := h.userRepo.GetAllUserTokens(c, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"res": tokens})

}

func (h *Handler) DeleteUserToken(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	tokenID := c.Param("id")

	err := h.userRepo.DeleteAPIToken(c, currentUser.ID, tokenID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete the token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) UpdateNotificationTarget(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	type Request struct {
		Type   nModel.NotificationType `json:"type"`
		Target string                  `json:"target"`
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.Type == nModel.NotificationTypeNone {
		err := h.userRepo.DeleteNotificationTarget(c, currentUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification target"})
			return
		}
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	err := h.userRepo.UpdateNotificationTarget(c, currentUser.ID, req.Target, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification target"})
		return
	}

	err = h.userRepo.UpdateNotificationTargetForAllNotifications(c, currentUser.ID, req.Target, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification target for all notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) updateUserPasswordLoggedInOnly(c *gin.Context) {
	if h.isDonetickDotCom {
		// only enable this feature for self-hosted instances
		c.JSON(http.StatusForbidden, gin.H{"error": "This action is not allowed on donetick.com"})
		return
	}
	logger := logging.FromContext(c)
	type RequestBody struct {
		Password string `json:"password" binding:"required,min=8,max=32"`
	}
	var body RequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		logger.Errorw("user.handler.resetAccountPassword failed to bind", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request",
		})
		return
	}

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	password, err := auth.EncodePassword(body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to process password",
		})
		return
	}

	err = h.userRepo.UpdatePasswordByUserId(c.Request.Context(), currentUser.ID, password)
	if err != nil {
		logger.Errorw("account.handler.resetAccountPassword failed to reset password", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Unable to reset password",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

func Routes(router *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter) {

	userRoutes := router.Group("api/v1/users")
	userRoutes.Use(auth.MiddlewareFunc(), utils.RateLimitMiddleware(limiter))
	{
		userRoutes.GET("/", h.GetAllUsers())
		userRoutes.GET("/profile", h.GetUserProfile)
		userRoutes.PUT("", h.UpdateUserDetails)
		userRoutes.POST("/tokens", h.CreateLongLivedToken)
		userRoutes.GET("/tokens", h.GetAllUserToken)
		userRoutes.DELETE("/tokens/:id", h.DeleteUserToken)
		userRoutes.PUT("/targets", h.UpdateNotificationTarget)
		userRoutes.PUT("change_password", h.updateUserPasswordLoggedInOnly)

	}

	authRoutes := router.Group("api/v1/auth")
	authRoutes.Use(utils.RateLimitMiddleware(limiter))
	{
		authRoutes.POST("/:provider/callback", h.thirdPartyAuthCallback)
		authRoutes.POST("/", h.signUp)
		authRoutes.POST("login", auth.LoginHandler)
		authRoutes.GET("refresh", auth.RefreshHandler)
		authRoutes.POST("reset", h.resetPassword)
		authRoutes.POST("password", h.updateUserPassword)
	}
	pingRoutes := router.Group("api/v1/ping")
	pingRoutes.Use(utils.RateLimitMiddleware(limiter))
	{
		pingRoutes.GET("/", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
	}
}
