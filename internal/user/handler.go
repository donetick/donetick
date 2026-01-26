package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"donetick.com/core/config"
	auth "donetick.com/core/internal/auth"
	"donetick.com/core/internal/auth/apple"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/email"
	"donetick.com/core/internal/mfa"
	nModel "donetick.com/core/internal/notifier/model"
	storage "donetick.com/core/internal/storage"
	storageRepo "donetick.com/core/internal/storage/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	limiter "github.com/ulule/limiter/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/oauth2/v1"
	"google.golang.org/api/option"
)

type Handler struct {
	userRepo               *uRepo.UserRepository
	circleRepo             *cRepo.CircleRepository
	jwtAuth                *jwt.GinJWTMiddleware
	email                  *email.EmailSender
	identityProvider       *auth.IdentityProvider
	isDonetickDotCom       bool
	IsUserCreationDisabled bool
	DonetickCloudConfig    config.DonetickCloudConfig
	storage                storage.Storage
	storageRepo            *storageRepo.StorageRepository
	signer                 storage.URLSigner
	deletionService        *DeletionService
	appleService           *apple.AppleService
	maxSubaccounts         int
	plusMaxSubaccounts     int
}

func NewHandler(ur *uRepo.UserRepository, cr *cRepo.CircleRepository,
	jwtAuth *jwt.GinJWTMiddleware, email *email.EmailSender,
	idp *auth.IdentityProvider,
	storage storage.Storage,
	signer storage.URLSigner,
	storageRepo *storageRepo.StorageRepository,
	appleService *apple.AppleService,
	deletionService *DeletionService, config *config.Config) *Handler {
	return &Handler{
		userRepo:               ur,
		circleRepo:             cr,
		jwtAuth:                jwtAuth,
		email:                  email,
		identityProvider:       idp,
		isDonetickDotCom:       config.IsDoneTickDotCom,
		IsUserCreationDisabled: config.IsUserCreationDisabled,
		DonetickCloudConfig:    config.DonetickCloudConfig,
		storage:                storage,
		storageRepo:            storageRepo,
		signer:                 signer,
		deletionService:        deletionService,
		appleService:           appleService,
		maxSubaccounts:         config.FeatureLimits.MaxSubaccounts,
		plusMaxSubaccounts:     config.FeatureLimits.PlusMaxSubaccounts,
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

	// Validate username format
	if !utils.IsValidUsername(signupReq.Username) {
		c.JSON(400, gin.H{
			"error": "Username can only contain lowercase letters (a-z), numbers (0-9), dots (.), and hyphens (-)",
		})
		return
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
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
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

	switch provider {
	case "google":
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
		service, err := oauth2.NewService(c, option.WithHTTPClient(http.DefaultClient))
		if err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback failed to create oauth2 service", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication service unavailable",
			})
			return
		}

		tokenInfo, err := service.Tokeninfo().AccessToken(body.Token).Do()
		if err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback failed to get token info", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid token",
			})
			return
		}
		logger.Infow("account.handler.thirdPartyAuthCallback", "tokenInfo", tokenInfo)
		if tokenInfo.Audience != h.DonetickCloudConfig.GoogleClientID && tokenInfo.Audience != h.DonetickCloudConfig.GoogleIOSClientID && tokenInfo.Audience != h.DonetickCloudConfig.GoogleAndroidClientID {
			logger.Errorw("account.handler.thirdPartyAuthCallback token audience mismatch", "audience", tokenInfo.Audience)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid token",
			})
			return
		}
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
			encodedPassword, err := auth.EncodePassword(password) //nolint:ineffassign
			if err != nil {
				logger.Errorw("account.handler.thirdPartyAuthCallback failed to encode password", "err", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Unable to create user account",
				})
				return
			}
			account := &uModel.User{
				Username:    userinfo.Id,
				Email:       userinfo.Email,
				Image:       userinfo.Picture,
				Password:    encodedPassword,
				DisplayName: userinfo.GivenName,
				Provider:    uModel.AuthProviderGoogle,
			}
			createdUser, err := h.userRepo.CreateUser(c, account)
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
		// Check if user has MFA enabled
		if acc.MFAEnabled {
			// Create MFA session for third-party auth
			mfaService := mfa.NewMFAService("Donetick")
			sessionToken, err := mfaService.GenerateSessionToken()
			if err != nil {
				logger.Errorw("Failed to generate MFA session token", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			mfaSession := &uModel.MFASession{
				SessionToken: sessionToken,
				UserID:       acc.ID,
				AuthMethod:   "google",
				Verified:     false,
				CreatedAt:    time.Now(),
				ExpiresAt:    time.Now().Add(10 * time.Minute),
				UserData:     acc.Username,
			}

			if err := h.userRepo.CreateMFASession(c, mfaSession); err != nil {
				logger.Errorw("Failed to create MFA session", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"mfaRequired":  true,
				"sessionToken": sessionToken,
			})
			return
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
	case "apple":
		c.Set("auth_provider", "3rdPartyAuth")
		// AppleAuthRequest matches the structure of the incoming Apple auth payload
		type AppleAuthRequest struct {
			Provider string `json:"provider" binding:"required"`
			Data     struct {
				Provider string `json:"provider"`
				Result   struct {
					IDToken     string `json:"idToken"`
					AccessToken struct {
						Token string `json:"token"`
					} `json:"accessToken"`
					Profile struct {
						User       string `json:"user"`
						GivenName  string `json:"givenName"`
						FamilyName string `json:"familyName"`
						Email      string `json:"email"`
					} `json:"profile"`
				} `json:"result"`
			} `json:"data"`
		}

		var body AppleAuthRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to bind", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request",
			})
			return
		}

		// Validate the ID token - use the JWT from accessToken.token, not the short idToken
		idToken := body.Data.Result.IDToken
		if idToken == "" || len(idToken) < 100 { // JWT tokens are much longer
			// Fallback to accessToken.token which contains the actual JWT
			idToken = body.Data.Result.AccessToken.Token
		}
		userInfo, err := h.appleService.ValidateIDToken(c.Request.Context(), idToken)
		if err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to validate token", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid Apple ID token",
			})
			return
		}

		logger.Infow("account.handler.thirdPartyAuthCallback (apple)", "userInfo", userInfo)

		// Check if user exists
		acc, err := h.userRepo.FindByEmail(c, userInfo.Email)

		if err != nil {
			// Create user account
			password := auth.GenerateRandomPassword(12)
			encodedPassword, err := auth.EncodePassword(password)
			if err != nil {
				logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to encode password", "err", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Unable to create user account",
				})
				return
			}

			// Use provided names from profile or fallback to email
			displayName := body.Data.Result.Profile.GivenName
			if displayName == "" {
				displayName = userInfo.Email
			}

			account := &uModel.User{
				Username:    userInfo.Sub,
				Email:       userInfo.Email,
				Password:    encodedPassword,
				DisplayName: displayName,
				Provider:    uModel.AuthProviderApple,
			}

			createdUser, err := h.userRepo.CreateUser(c, account)
			if err != nil {
				logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to create user", "err", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Unable to create user",
				})
				return
			}

			// Create Circle for the user
			userCircle, err := h.circleRepo.CreateCircle(c, &cModel.Circle{
				Name:       displayName + "'s circle",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				InviteCode: utils.GenerateInviteCode(c),
			})

			if err != nil {
				logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to create circle", "err", err)
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
				logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to add user to circle", "err", err)
				c.JSON(500, gin.H{
					"error": "Error adding user to circle",
				})
				return
			}

			createdUser.CircleID = userCircle.ID
			if err := h.userRepo.UpdateUser(c, createdUser); err != nil {
				logger.Errorw("account.handler.thirdPartyAuthCallback (apple) failed to update user", "err", err)
				c.JSON(500, gin.H{
					"error": "Error updating user",
				})
				return
			}

			acc = &uModel.UserDetails{User: *createdUser}
		}

		// Check if user has MFA enabled
		if acc.MFAEnabled {
			// Create MFA session for Apple auth
			mfaService := mfa.NewMFAService("Donetick")
			sessionToken, err := mfaService.GenerateSessionToken()
			if err != nil {
				logger.Errorw("Failed to generate MFA session token", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			mfaSession := &uModel.MFASession{
				SessionToken: sessionToken,
				UserID:       acc.ID,
				AuthMethod:   "apple",
				Verified:     false,
				CreatedAt:    time.Now(),
				ExpiresAt:    time.Now().Add(10 * time.Minute),
				UserData:     acc.Username,
			}

			if err := h.userRepo.CreateMFASession(c, mfaSession); err != nil {
				logger.Errorw("Failed to create MFA session", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"mfaRequired":  true,
				"sessionToken": sessionToken,
			})
			return
		}

		// Generate JWT token for the user
		c.Set("user_account", acc)
		h.jwtAuth.Authenticator(c)
		tokenString, expire, err := h.jwtAuth.TokenGenerator(acc)
		if err != nil {
			logger.Errorw("Unable to Generate a Token for Apple user")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Unable to Generate a Token",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": tokenString, "expire": expire})
		return
	case "oauth2":
		c.Set("auth_provider", "3rdPartyAuth")
		// Read the ID token from the request bod
		type Request struct {
			Code string `json:"code"`
		}
		var req Request
		if err := c.BindJSON(&req); err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback (oauth2) failed to bind request", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		// Validate that the code is not empty
		if req.Code == "" {
			logger.Errorw("account.handler.thirdPartyAuthCallback (oauth2) empty authorization code")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code is required"})
			return
		}

		logger.Infow("account.handler.thirdPartyAuthCallback (oauth2) attempting to exchange code", "codeLength", len(req.Code))

		token, err := h.identityProvider.ExchangeToken(c, req.Code)

		if err != nil {
			logger.Errorw("account.handler.thirdPartyAuthCallback (oauth2) failed to exchange token", "err", err, "code", req.Code[:min(len(req.Code), 10)]+"...")
			// Return a more specific error message based on the OAuth2 error
			if strings.Contains(err.Error(), "invalid_grant") {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Authorization code is invalid, expired, or already used. Please try the authentication process again.",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to exchange authorization code for token",
				})
			}
			return
		}

		claims, err := h.identityProvider.GetUserInfo(c, token)
		if err != nil {
			logger.Error("account.handler.thirdPartyAuthCallback (oauth2) failed to get claims", "err", err)
		}

		acc, err := h.userRepo.FindByEmail(c, claims.Email)
		if err != nil {
			// Create user
			password := auth.GenerateRandomPassword(12)
			encodedPassword, err := auth.EncodePassword(password)
			if err != nil {
				logger.Error("account.handler.thirdPartyAuthCallback (oauth2) password encoding failed", "err", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Password encoding failed"})
				return
			}
			account := &uModel.User{
				Username:    claims.Email,
				Email:       claims.Email,
				Password:    encodedPassword,
				DisplayName: claims.DisplayName,
				Provider:    uModel.AuthProviderOAuth2,
			}
			createdUser, err := h.userRepo.CreateUser(c, account)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Unable to create user",
				})
				return

			}
			// Create Circle for the user:
			userCircle, err := h.circleRepo.CreateCircle(c, &cModel.Circle{
				Name:       claims.DisplayName + "'s circle",
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
			acc = &uModel.UserDetails{User: *createdUser}
		}
		// Check if user has MFA enabled
		if acc.MFAEnabled {
			// Create MFA session for OAuth2 auth
			mfaService := mfa.NewMFAService("Donetick")
			sessionToken, err := mfaService.GenerateSessionToken()
			if err != nil {
				logger.Error("Failed to generate MFA session token", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			mfaSession := &uModel.MFASession{
				SessionToken: sessionToken,
				UserID:       acc.ID,
				AuthMethod:   "oauth2",
				Verified:     false,
				CreatedAt:    time.Now(),
				ExpiresAt:    time.Now().Add(10 * time.Minute),
				UserData:     acc.Username,
			}

			if err := h.userRepo.CreateMFASession(c, mfaSession); err != nil {
				logger.Error("Failed to create MFA session", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"mfaRequired":  true,
				"sessionToken": sessionToken,
			})
			return
		}
		// ... (JWT generation and response)
		c.Set("user_account", acc)
		h.jwtAuth.Authenticator(c)
		tokenString, expire, err := h.jwtAuth.TokenGenerator(acc)
		if err != nil {
			logger.Error("Unable to Generate a Token")
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
		Timezone    *string `json:"timezone" binding:"omitempty"`
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
	if req.Timezone != nil {
		if !utils.IsValidTimezone(*req.Timezone) {
			c.JSON(400, gin.H{
				"error": "Invalid timezone",
			})
			return
		}
		user.Timezone = *req.Timezone
	}

	if err := h.userRepo.UpdateUser(c, &user.User); err != nil {
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
		Name    string `json:"name" binding:"required"`
		MFACode string `json:"mfaCode"` // Optional MFA code for enhanced security
	}
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// If user has MFA enabled and provides an MFA code, verify it
	if currentUser.MFAEnabled && req.MFACode != "" {
		mfaService := mfa.NewMFAService("Donetick")
		valid, newUsedCodes, err := mfaService.IsCodeValid(
			currentUser.MFASecret,
			currentUser.MFABackupCodes,
			currentUser.MFARecoveryUsed,
			req.MFACode,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate MFA code"})
			return
		}

		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid MFA code"})
			return
		}

		// Update used codes if a backup code was used
		if newUsedCodes != currentUser.MFARecoveryUsed {
			if err := h.userRepo.UpdateMFARecoveryCodes(c, currentUser.ID, newUsedCodes); err != nil {
				logging.FromContext(c).Errorw("Failed to update recovery codes", "error", err)
			}
		}
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

	response := gin.H{"res": tokenModel}

	// If user has MFA enabled but didn't provide a code, suggest using MFA for enhanced security
	if currentUser.MFAEnabled && req.MFACode == "" {
		response["message"] = "API token created successfully. For enhanced security, consider providing an MFA code when creating API tokens."
	}

	c.JSON(http.StatusOK, response)
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
		Type   nModel.NotificationPlatform `json:"type"`
		Target string                      `json:"target"`
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.Type == nModel.NotificationPlatformNone {
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

func (h *Handler) setWebhook(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	type Request struct {
		URL *string `json:"url"`
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if !currentUser.IsPlusMember() {
		c.JSON(http.StatusForbidden, gin.H{"error": "This action is only available for Plus members"})
		return
	}

	// get circle admins
	admins, err := h.circleRepo.GetCircleAdmins(c, currentUser.CircleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get circle details"})
		return
	}

	// confirm that the user is an admin:
	isAdmin := false
	for _, admin := range admins {
		if admin.ID == currentUser.ID {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "You are not an admin"})
		return
	}

	err = h.circleRepo.SetWebhookURL(c, currentUser.CircleID, req.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set webhook URL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

func (h *Handler) updateProfilePhoto(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file"})
		return
	}
	fileExtension := file.Filename[strings.LastIndex(file.Filename, "."):]
	// validate file extension:
	if fileExtension != ".jpg" && fileExtension != ".jpeg" && fileExtension != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file extension"})
		return
	}

	// Generate a unique filename using the current timestamp and a random string

	hashFromUserName := sha256.Sum256([]byte(currentUser.Username))
	// use the first 8 bytes of the hash as a unique identifier
	id := fmt.Sprintf("%x", hashFromUserName[:20])
	filename := fmt.Sprintf("profiles/%s%s", id, fileExtension)

	openedFile, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer openedFile.Close()

	err = h.storage.Save(c, filename, openedFile)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to save profile photo", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	signedFileName, err := h.signer.Sign(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign URL"})
		return
	}
	err = h.userRepo.UpdateUserImage(c, currentUser.ID, signedFileName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile photo"})
		return
	}
	// create signed URL for the file:
	signedURL, err := h.signer.Sign(filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign URL"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sign": signedURL})
}

func (h *Handler) getStorageUsage(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current user"})
		return
	}

	used, available, err := h.storageRepo.GetStorageStats(c, currentUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get storage usage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"res": gin.H{
			"used":  used,
			"total": available,
		},
	})

}

// Account deletion request/response types
type AccountDeletionRequest struct {
	Password        string                 `json:"password" binding:"required"`
	TransferOptions []CircleTransferOption `json:"transferOptions,omitempty"`
	Confirmation    string                 `json:"confirmation" binding:"required"` // Must be "DELETE"
}

type AccountDeletionCheckRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *Handler) checkAccountDeletion(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req AccountDeletionCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify password
	if auth.Matches(currentUser.Password, req.Password) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid password"})
		return
	}

	// Check what would be deleted (dry run)
	result, err := h.deletionService.CheckUserAccountDeletion(c.Request.Context(), currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check account deletion: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) deleteAccount(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req AccountDeletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate confirmation text
	if req.Confirmation != "DELETE" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Confirmation text must be 'DELETE'"})
		return
	}

	// Verify password
	if auth.Matches(currentUser.Password, req.Password) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// Perform account deletion
	result, err := h.deletionService.DeleteUserAccount(c.Request.Context(), currentUser.ID, req.TransferOptions)
	if err != nil {
		logging.DefaultLogger().Errorf("Failed to delete account for user %d: %v", currentUser.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account: " + err.Error()})
		return
	}

	if !result.Success {
		c.JSON(http.StatusBadRequest, result)
		return
	}

	// Log the account deletion
	logging.DefaultLogger().Infof("Account deleted successfully for user %d (%s)", currentUser.ID, currentUser.Username)

	c.JSON(http.StatusOK, result)
}

// Child User Management endpoints

// CreateChildUserRequest represents the request to create a child user
type CreateChildUserRequest struct {
	ChildName   string `json:"childName" binding:"required,min=2,max=20"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password" binding:"required,min=8,max=45"`
}

// UpdateChildPasswordRequest represents the request to update a child user's password
type UpdateChildPasswordRequest struct {
	ChildUserID int    `json:"childUserId" binding:"required"`
	Password    string `json:"password" binding:"required,min=8,max=45"`
}

// ChildUserResponse represents the response for child user operations
type ChildUserResponse struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	UserType    string `json:"userType"`
	CreatedAt   string `json:"createdAt"`
}

func (h *Handler) createChildUser(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Only parent users can create child users
	if currentUser.UserType != uModel.UserTypeParent {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only parent users can create child accounts"})
		return
	}

	var req CreateChildUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check current child user count to enforce limit (only for donetick.com)
	if h.isDonetickDotCom {
		currentChildCount, err := h.userRepo.GetChildUserCount(c, currentUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing subaccounts"})
			return
		}

		maxSubaccounts := h.maxSubaccounts
		if currentUser.IsPlusMember() {
			maxSubaccounts = h.plusMaxSubaccounts
		}

		if int(currentChildCount) >= maxSubaccounts {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("Maximum of %d subaccounts allowed per account", maxSubaccounts),
			})
			return
		}
	}
	// Validate username and child username:
	if !utils.IsValidUsername(currentUser.Username) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parent username format"})
		return
	}
	if !utils.IsValidUsername(req.ChildName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid child username format"})
		return
	}

	// Generate child username
	childUsername := uModel.GenerateChildUsername(currentUser.Username, req.ChildName)

	// Set default display name if not provided
	if req.DisplayName == "" {
		req.DisplayName = req.ChildName
	}

	// Encode password
	encodedPassword, err := auth.EncodePassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Create child user
	childUser := &uModel.User{
		Username:     childUsername,
		DisplayName:  html.EscapeString(req.DisplayName),
		Password:     encodedPassword,
		CircleID:     currentUser.CircleID,
		ParentUserID: &currentUser.ID,
		UserType:     uModel.UserTypeChild,
		Provider:     uModel.AuthProviderDonetick,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Validate child user
	if err := childUser.ValidateChildUser(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdUser, err := h.userRepo.CreateUser(c, childUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create child user"})
		return
	}

	// Add child user to the same circle as parent
	if err := h.circleRepo.AddUserToCircle(c, &cModel.UserCircle{
		UserID:    createdUser.ID,
		CircleID:  currentUser.CircleID,
		Role:      "member", // Child users are members, not admins
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add child user to circle"})
		return
	}

	response := ChildUserResponse{
		ID:          createdUser.ID,
		Username:    createdUser.Username,
		DisplayName: createdUser.DisplayName,
		UserType:    "child",
		CreatedAt:   createdUser.CreatedAt.Format(time.RFC3339),
	}

	c.JSON(http.StatusCreated, gin.H{"res": response})
}

func (h *Handler) updateChildPassword(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Only parent users can update child passwords
	if currentUser.UserType != uModel.UserTypeParent {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only parent users can update child passwords"})
		return
	}

	var req UpdateChildPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify that the child user belongs to the current parent
	childUser, err := h.userRepo.GetUserByID(c, req.ChildUserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child user not found"})
		return
	}

	if childUser.ParentUserID == nil || *childUser.ParentUserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only update passwords for your own child users"})
		return
	}

	// Encode new password
	encodedPassword, err := auth.EncodePassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Update password
	err = h.userRepo.UpdatePasswordByUserId(c.Request.Context(), req.ChildUserID, encodedPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child user password updated successfully"})
}

func (h *Handler) deleteChildUser(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Only parent users can delete child users
	if currentUser.UserType != uModel.UserTypeParent {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only parent users can delete child accounts"})
		return
	}

	childUserID := c.Param("id")
	if childUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Child user ID is required"})
		return
	}

	// Parse child user ID
	childID := 0
	if _, err := fmt.Sscanf(childUserID, "%d", &childID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid child user ID"})
		return
	}

	// Verify that the child user belongs to the current parent
	childUser, err := h.userRepo.GetUserByID(c, childID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child user not found"})
		return
	}

	if childUser.ParentUserID == nil || *childUser.ParentUserID != currentUser.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own child users"})
		return
	}

	// Delete child user account (this will cascade delete all associated data)
	result, err := h.deletionService.DeleteUserAccount(c.Request.Context(), childID, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete child user: " + err.Error()})
		return
	}

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete child user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child user deleted successfully"})
}

func (h *Handler) getChildUsers(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Only parent users can view child users
	if currentUser.UserType != uModel.UserTypeParent {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only parent users can view child accounts"})
		return
	}

	childUsers, err := h.userRepo.GetChildUsersByParentID(c, currentUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get child users"})
		return
	}

	var response []ChildUserResponse
	for _, child := range childUsers {
		response = append(response, ChildUserResponse{
			ID:          child.ID,
			Username:    child.Username,
			DisplayName: child.DisplayName,
			UserType:    "child",
			CreatedAt:   child.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{"res": response})
}

// RefreshRequest represents a refresh token request
type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// logout handles user logout by clearing cookies and revoking refresh tokens
func (h *Handler) logout(c *gin.Context) {
	logger := logging.FromContext(c)

	// Try to get refresh token from cookie first (httpOnly), fallback to body
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		// Fallback to JSON body for backward compatibility
		var req RefreshRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			// No token provided, but logout should still succeed
			refreshToken = ""
		} else {
			refreshToken = req.RefreshToken
		}
	}

	// Revoke the refresh token if we have one
	if refreshToken != "" {
		// Hash the refresh token before looking it up (tokens are stored hashed)
		tokenHash := hashToken(refreshToken)

		// Get the session by token hash
		session, err := h.userRepo.GetUserSessionByTokenHash(c.Request.Context(), tokenHash)
		if err != nil {
			logger.Infow("Refresh token session not found during logout", "note", "Token may already be expired or invalid")
			// Don't return error - logout should always succeed from client perspective
		} else {
			// Revoke the session
			if err := h.userRepo.RevokeSession(c.Request.Context(), session.ID); err != nil {
				logger.Errorw("Failed to revoke session during logout", "error", err)
				// Don't return error - logout should always succeed from client perspective
			}
		}
	}

	// Clear the httpOnly cookie by setting it with negative max age
	c.SetCookie("refresh_token", "", 0, "/", "", true, true)

	// Also clear any access token cookie if it exists
	c.SetCookie("access_token", "", 0, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logged out successfully",
	})
}

// hashToken creates a SHA-256 hash of the token for database lookup
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func Routes(router *gin.Engine, h *Handler, jwtAuth *jwt.GinJWTMiddleware, limiter *limiter.Limiter, cfg *config.Config) {

	userRoutes := router.Group("api/v1/users")
	userRoutes.Use(jwtAuth.MiddlewareFunc(), utils.RateLimitMiddleware(limiter))
	{
		userRoutes.GET("/", h.GetAllUsers())
		userRoutes.GET("/profile", h.GetUserProfile)
		userRoutes.PUT("", h.UpdateUserDetails)
		userRoutes.POST("/tokens", h.CreateLongLivedToken)
		userRoutes.GET("/tokens", h.GetAllUserToken)
		userRoutes.DELETE("/tokens/:id", h.DeleteUserToken)
		userRoutes.PUT("/webhook", h.setWebhook)
		userRoutes.PUT("/targets", h.UpdateNotificationTarget)
		userRoutes.PUT("change_password", h.updateUserPasswordLoggedInOnly)
		userRoutes.POST("profile_photo", h.updateProfilePhoto)
		userRoutes.GET("storage", h.getStorageUsage)
		userRoutes.POST("/logout", h.logout) // Logout endpoint to clear cookies and expire refresh tokens

		// MFA endpoints
		userRoutes.GET("/mfa/status", h.getMFAStatus)
		userRoutes.POST("/mfa/setup", h.setupMFA)
		userRoutes.POST("/mfa/confirm", h.confirmMFA)
		userRoutes.POST("/mfa/disable", h.disableMFA)
		// userRoutes.POST("/mfa/regenerate-backup-codes", h.regenerateBackupCodes)

		// Account deletion endpoints
		userRoutes.POST("/delete/check", h.checkAccountDeletion)
		userRoutes.DELETE("/delete", h.deleteAccount)

		// Child user management endpoints
		userRoutes.POST("/subaccounts", h.createChildUser)
		userRoutes.GET("/subaccounts", h.getChildUsers)
		userRoutes.PUT("/subaccounts/password", h.updateChildPassword)
		userRoutes.DELETE("/subaccounts/:id", h.deleteChildUser)
	}

	// Create new auth handler for enhanced token management
	authHandler := auth.NewAuthHandler(h.userRepo, jwtAuth, cfg)

	authRoutes := router.Group("api/v1/auth")
	authRoutes.Use(utils.RateLimitMiddleware(limiter))
	{
		authRoutes.POST("/:provider/callback", h.thirdPartyAuthCallback)
		authRoutes.POST("/", h.signUp)
		authRoutes.POST("login", authHandler.EnhancedLoginHandler)  // Enhanced login with refresh tokens
		authRoutes.POST("login/legacy", jwtAuth.LoginHandler)       // Legacy login for backward compatibility
		authRoutes.POST("refresh", authHandler.RefreshTokenHandler) // Changed from GET to POST
		authRoutes.POST("logout", authHandler.LogoutHandler)        // New logout endpoint
		authRoutes.POST("reset", h.resetPassword)
		authRoutes.POST("password", h.updateUserPassword)
		authRoutes.POST("mfa/verify", h.verifyMFA) // Add MFA verification endpoint
	}

	// Protected auth routes (require JWT)
	protectedAuthRoutes := router.Group("api/v1/auth")
	protectedAuthRoutes.Use(jwtAuth.MiddlewareFunc(), utils.RateLimitMiddleware(limiter))
	{
		protectedAuthRoutes.POST("revoke-all", authHandler.RevokeAllHandler) // New revoke all endpoint
	}
}
