package auth

import (
	"net/http"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/mfa"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var identityKey = "id"

type signIn struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

func CurrentUser(c *gin.Context) (*uModel.UserDetails, bool) {
	data, ok := c.Get(identityKey)
	if !ok {
		return nil, false
	}
	acc, ok := data.(*uModel.UserDetails)
	return acc, ok
}

func MustCurrentUser(c *gin.Context) *uModel.UserDetails {
	acc, ok := CurrentUser(c)
	if ok {
		return acc
	}
	panic("no account in gin.Context")
}

func NewAuthMiddleware(cfg *config.Config, userRepo *uRepo.UserRepository) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "test zone",
		Key:         []byte(cfg.Jwt.Secret),
		Timeout:     cfg.Jwt.SessionTime,
		MaxRefresh:  cfg.Jwt.MaxRefresh, // 7 days as long as their token is valid they can refresh it
		IdentityKey: identityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if u, ok := data.(*uModel.UserDetails); ok {
				return jwt.MapClaims{
					identityKey: u.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			username, ok := claims[identityKey].(string)
			if !ok {
				return nil
			}
			user, err := userRepo.GetUserByUsername(c.Request.Context(), username)
			if err != nil {
				return nil
			}
			return user
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			provider := c.Value("auth_provider")
			switch provider {
			case nil:
				var req signIn
				if err := c.ShouldBindJSON(&req); err != nil {
					return "", jwt.ErrMissingLoginValues
				}

				// ctx := cache.WithCacheSkip(c.Request.Context(), true)
				user, err := userRepo.GetUserByUsername(c.Request.Context(), req.Username)
				if err != nil || user.Disabled {
					return nil, jwt.ErrFailedAuthentication
				}
				err = Matches(user.Password, req.Password)
				if err != nil {
					if err != bcrypt.ErrMismatchedHashAndPassword {
						logging.FromContext(c).Warnw("middleware.jwt.Authenticator found unknown error when matches password", "err", err)
					}
					return nil, jwt.ErrFailedAuthentication
				}

				// Check if MFA is enabled for this user
				if user.MFAEnabled {
					// Create MFA session for verification
					mfaService := mfa.NewMFAService("Donetick")
					sessionToken, err := mfaService.GenerateSessionToken()
					if err != nil {
						logging.FromContext(c).Errorw("Failed to generate MFA session token", "error", err)
						return nil, jwt.ErrFailedAuthentication
					}

					mfaSession := &uModel.MFASession{
						SessionToken: sessionToken,
						UserID:       user.ID,
						AuthMethod:   "local",
						Verified:     false,
						CreatedAt:    time.Now(),
						ExpiresAt:    time.Now().Add(10 * time.Minute), // 10-minute expiry
						UserData:     user.Username,                    // Store username for later verification
					}

					if err := userRepo.CreateMFASession(c.Request.Context(), mfaSession); err != nil {
						logging.FromContext(c).Errorw("Failed to create MFA session", "error", err)
						return nil, jwt.ErrFailedAuthentication
					}

					// Return special error to indicate MFA is required
					c.Set("mfa_required", true)
					c.Set("mfa_session_token", sessionToken)
					return nil, NewMFARequiredError()
				}

				return &uModel.UserDetails{
					User: uModel.User{
						ID:        user.ID,
						Username:  user.Username,
						Password:  "",
						Image:     user.Image,
						CreatedAt: user.CreatedAt,
						UpdatedAt: user.UpdatedAt,
						Disabled:  user.Disabled,
						CircleID:  user.CircleID,
					},
					WebhookURL: user.WebhookURL,
				}, nil
			case "3rdPartyAuth":
				// we should only reach this stage if a handler mannually call authenticator with it's context:

				var authObject *uModel.UserDetails
				v := c.Value("user_account")
				authObject = v.(*uModel.UserDetails)

				return authObject, nil

			default:
				return nil, jwt.ErrFailedAuthentication
			}
		},

		Authorizator: func(data interface{}, c *gin.Context) bool {

			if _, ok := data.(*uModel.UserDetails); ok {
				return true
			}
			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			logging.FromContext(c).Info("middleware.jwt.Unauthorized", "code", code, "message", message)

			// Check if MFA is required
			if mfaRequired, exists := c.Get("mfa_required"); exists && mfaRequired.(bool) {
				sessionToken, _ := c.Get("mfa_session_token")
				c.JSON(http.StatusOK, gin.H{
					"mfaRequired":  true,
					"sessionToken": sessionToken,
				})
				return
			}

			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		LoginResponse: func(c *gin.Context, code int, token string, expire time.Time) {
			// Check if MFA is required
			if mfaRequired, exists := c.Get("mfa_required"); exists && mfaRequired.(bool) {
				sessionToken, _ := c.Get("mfa_session_token")
				c.JSON(http.StatusOK, gin.H{
					"mfaRequired":  true,
					"sessionToken": sessionToken,
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"code":   code,
				"token":  token,
				"expire": expire,
			})
		},
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})
}
