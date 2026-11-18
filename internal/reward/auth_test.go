package reward

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	points "donetick.com/core/internal/points"
	rModel "donetick.com/core/internal/reward/model"
	rRepo "donetick.com/core/internal/reward/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type rewardAuthTest struct {
	router *gin.Engine
	db     *gorm.DB
	jwt    *jwt.GinJWTMiddleware
}

func newRewardAuthTest(t *testing.T) *rewardAuthTest {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "rewards.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.APIToken{}, &uModel.UserNotificationTarget{},
		&cModel.Circle{}, &cModel.UserCircle{}, &rModel.Reward{}, &rModel.RewardRedemption{}, &points.PointsHistory{}))
	for id := 1; id <= 4; id++ {
		circleID, role := 1, cModel.UserRoleMember
		if id == 1 {
			role = cModel.UserRoleAdmin
		}
		if id == 3 {
			circleID, role = 2, cModel.UserRoleAdmin
		}
		require.NoError(t, db.Create(&uModel.User{ID: id, Username: fmt.Sprintf("user%d", id), CircleID: circleID, Disabled: id == 4}).Error)
		require.NoError(t, db.Create(&uModel.APIToken{UserID: id, Name: "test", Token: fmt.Sprintf("token-%d", id)}).Error)
		require.NoError(t, db.Create(&cModel.UserCircle{UserID: id, CircleID: circleID, Role: role, IsActive: true, Points: 100}).Error)
	}
	cfg := &config.Config{}
	cfg.Jwt.Secret = "reward-auth-test-secret"
	cfg.Jwt.SessionTime = time.Hour
	users := uRepo.NewUserRepository(db, cfg)
	jwtMiddleware, err := auth.NewAuthMiddleware(cfg, users, nil)
	require.NoError(t, err)
	router := gin.New()
	Routes(router, NewHandler(rRepo.NewRewardRepository(db, cfg), cRepo.NewCircleRepository(db), nil), auth.NewMultiAuthMiddleware(jwtMiddleware, users))
	return &rewardAuthTest{router: router, db: db, jwt: jwtMiddleware}
}

func (s *rewardAuthTest) request(t *testing.T, method, path, mode string, userID int, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/rewards"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	switch mode {
	case "api":
		req.Header.Set("secretkey", fmt.Sprintf("token-%d", userID))
	case "jwt":
		token, _, err := s.jwt.TokenGenerator(&uModel.UserDetails{User: uModel.User{Username: fmt.Sprintf("user%d", userID)}})
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	case "invalid-jwt":
		req.Header.Set("Authorization", "Bearer invalid-token")
	}
	res := httptest.NewRecorder()
	s.router.ServeHTTP(res, req)
	return res
}

func TestRewardRoutesAcceptAPIKeyAndJWT(t *testing.T) {
	for _, mode := range []string{"api", "jwt"} {
		for _, role := range []cModel.UserRole{cModel.UserRoleAdmin, cModel.UserRoleManager} {
			t.Run(fmt.Sprintf("%s/%s", mode, role), func(t *testing.T) {
				s := newRewardAuthTest(t)
				require.NoError(t, s.db.Model(&cModel.UserCircle{}).Where("user_id = ?", 1).Update("role", role).Error)
				request := func(method, path, body string) *httptest.ResponseRecorder {
					res := s.request(t, method, path, mode, 1, body)
					require.Equal(t, http.StatusOK, res.Code, res.Body.String())
					return res
				}
				created := request("POST", "", `{"name":"Game time","pointsCost":25}`)
				var reward struct {
					Res rModel.Reward `json:"res"`
				}
				require.NoError(t, json.Unmarshal(created.Body.Bytes(), &reward))
				require.Equal(t, 1, reward.Res.CreatedBy)
				path := fmt.Sprintf("/%d", reward.Res.ID)
				request("GET", "", "")
				request("PUT", path, `{"name":"Bonus game time","pointsCost":30}`)
				// The requester is the only approver, so no notification delivery is needed.
				for _, action := range []string{"approve", "reject"} {
					res := request("POST", path+"/redeem", `{"note":"Please"}`)
					var redemption struct {
						Res rModel.RewardRedemption `json:"res"`
					}
					require.NoError(t, json.Unmarshal(res.Body.Bytes(), &redemption))
					require.Equal(t, 1, redemption.Res.UserID)
					request("POST", fmt.Sprintf("/redemptions/%d/%s", redemption.Res.ID, action), "")
					var stored rModel.RewardRedemption
					require.NoError(t, s.db.First(&stored, redemption.Res.ID).Error)
					want := rModel.RedemptionStatusApproved
					if action == "reject" {
						want = rModel.RedemptionStatusRejected
					}
					require.Equal(t, want, stored.Status)
					require.NotNil(t, stored.ResolvedBy)
					require.Equal(t, 1, *stored.ResolvedBy)
				}
				request("GET", "/redemptions", "")
				request("DELETE", path, "")
				var member cModel.UserCircle
				require.NoError(t, s.db.Where("user_id = ?", 1).First(&member).Error)
				require.Equal(t, 30, member.PointsRedeemed)
			})
		}
	}
}

func TestRewardRoutesRejectUnauthorizedRequests(t *testing.T) {
	s := newRewardAuthTest(t)
	for _, route := range []struct{ method, path string }{
		{"GET", ""}, {"POST", ""}, {"PUT", "/1"}, {"DELETE", "/1"},
		{"POST", "/1/redeem"}, {"GET", "/redemptions"},
		{"POST", "/redemptions/1/approve"}, {"POST", "/redemptions/1/reject"},
	} {
		for _, mode := range []string{"missing", "api", "invalid-jwt"} {
			t.Run(route.method+route.path+"/"+mode, func(t *testing.T) {
				// No user 99 exists; user 4 has a valid token but is disabled.
				for _, id := range []int{99, 4} {
					res := s.request(t, route.method, route.path, mode, id, "")
					require.Equal(t, http.StatusUnauthorized, res.Code, res.Body.String())
				}
			})
		}
	}
}

func TestRewardAPITokenPreservesMemberPermissionsAndCircleIsolation(t *testing.T) {
	s := newRewardAuthTest(t)
	for _, route := range []struct{ method, path string }{
		{"POST", ""}, {"PUT", "/1"}, {"DELETE", "/1"},
		{"POST", "/redemptions/1/approve"}, {"POST", "/redemptions/1/reject"},
	} {
		res := s.request(t, route.method, route.path, "api", 2, `{"name":"Forbidden","pointsCost":10}`)
		require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	}
	for id := 1; id <= 3; id++ {
		circleID := 1
		if id == 3 {
			circleID = 2
		}
		require.NoError(t, s.db.Create(&rModel.Reward{ID: id, CircleID: circleID, Name: fmt.Sprintf("Reward%d", id), PointsCost: 10}).Error)
		require.NoError(t, s.db.Create(&rModel.RewardRedemption{ID: id, RewardID: id, CircleID: circleID, UserID: id}).Error)
	}
	res := s.request(t, "GET", "", "api", 2, "")
	require.Equal(t, http.StatusOK, res.Code)
	var rewards []rModel.Reward
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &rewards))
	require.Len(t, rewards, 2)
	for _, reward := range rewards {
		require.Equal(t, 1, reward.CircleID)
	}
	for _, id := range []int{1, 2, 3} {
		res = s.request(t, "GET", "/redemptions", "api", id, "")
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		var redemptions []rModel.RewardRedemption
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &redemptions))
		if id == 1 {
			require.Len(t, redemptions, 2)
		} else {
			require.Len(t, redemptions, 1)
			require.Equal(t, id, redemptions[0].UserID)
		}
	}
	// An administrator in circle 2 cannot resolve a circle 1 redemption.
	res = s.request(t, "POST", "/redemptions/1/approve", "api", 3, "")
	require.NotEqual(t, http.StatusOK, res.Code)
	var redemption rModel.RewardRedemption
	require.NoError(t, s.db.First(&redemption, 1).Error)
	require.Equal(t, rModel.RedemptionStatusPending, redemption.Status)
}
