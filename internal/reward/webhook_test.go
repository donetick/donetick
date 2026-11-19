package reward

import (
	"context"
	"database/sql"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/events"
	pModel "donetick.com/core/internal/points"
	rModel "donetick.com/core/internal/reward/model"
	rRepo "donetick.com/core/internal/reward/repo"
	uModel "donetick.com/core/internal/user/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Simulate a database refusing commit, after all approval statements succeeded.
type failedCommitPool struct {
	*sql.DB
}

func (p failedCommitPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &failedCommitTx{tx}, nil
}

type failedCommitTx struct{ *sql.Tx }

func (tx failedCommitTx) Commit() error {
	_ = tx.Tx.Rollback()
	return errors.New("injected commit failure")
}

type rewardEventRecorder struct {
	onApproved func(*string, events.RewardRedemptionData)
}

func (r rewardEventRecorder) RewardRedeemApproved(_ context.Context, url *string, data events.RewardRedemptionData) {
	r.onApproved(url, data)
}

func TestApprovedRedemptionWebhook(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		role     cModel.UserRole
		circleID int
		points   int
		status   rModel.RedemptionStatus
		rollback bool
		want     int
	}{
		{"admin", cModel.UserRoleAdmin, 1, 100, rModel.RedemptionStatusPending, false, 200},
		{"manager", cModel.UserRoleManager, 1, 100, rModel.RedemptionStatusPending, false, 200},
		{"member", cModel.UserRoleMember, 1, 100, rModel.RedemptionStatusPending, false, 403},
		{"other circle", cModel.UserRoleAdmin, 2, 100, rModel.RedemptionStatusPending, false, 500},
		{"insufficient points", cModel.UserRoleAdmin, 1, 0, rModel.RedemptionStatusPending, false, 400},
		{"already approved", cModel.UserRoleAdmin, 1, 100, rModel.RedemptionStatusApproved, false, 400},
		{"rejected", cModel.UserRoleAdmin, 1, 100, rModel.RedemptionStatusRejected, false, 400},
		{"transaction rollback", cModel.UserRoleAdmin, 1, 100, rModel.RedemptionStatusPending, true, 500},
		{"commit failure", cModel.UserRoleAdmin, 1, 100, rModel.RedemptionStatusPending, false, 500},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "webhook.db")), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			require.NoError(t, db.AutoMigrate(&uModel.User{}, &uModel.UserNotificationTarget{},
				&cModel.UserCircle{}, &rModel.RewardRedemption{}, &pModel.PointsHistory{}))
			require.NoError(t, db.Create(&uModel.User{ID: 1, Username: "approver", CircleID: scenario.circleID}).Error)
			require.NoError(t, db.Create(&cModel.UserCircle{UserID: 1, CircleID: scenario.circleID, Role: scenario.role, IsActive: true}).Error)
			require.NoError(t, db.Create(&cModel.UserCircle{UserID: 2, CircleID: 1, Points: scenario.points, IsActive: true}).Error)
			require.NoError(t, db.Create(&rModel.RewardRedemption{ID: 7, RewardID: 42, CircleID: 1, UserID: 2, PointsCost: 25, Status: scenario.status}).Error)
			if scenario.rollback {
				require.NoError(t, db.Migrator().DropTable(&pModel.PointsHistory{}))
			}
			url := "https://ha.example/api/webhook/reward"
			calls := 0
			publisher := rewardEventRecorder{onApproved: func(gotURL *string, data events.RewardRedemptionData) {
				calls++
				require.Equal(t, &url, gotURL)
				require.Equal(t, events.RewardRedemptionData{RedemptionID: 7, RewardID: 42, RecipientUserID: 2}, data)
				// Read outside the approval transaction to check that publication follows commit.
				var stored rModel.RewardRedemption
				require.NoError(t, db.First(&stored, 7).Error)
				require.Equal(t, rModel.RedemptionStatusApproved, stored.Status)
				require.Equal(t, 1, *stored.ResolvedBy)
				var member cModel.UserCircle
				require.NoError(t, db.Where("user_id = ?", 2).First(&member).Error)
				require.Equal(t, 25, member.PointsRedeemed)
			}}
			approvalDB := db
			if scenario.name == "commit failure" {
				approvalDB = db.Session(&gorm.Session{NewDB: true})
				// Clone the statement before changing the connection pool.
				approvalDB = approvalDB.WithContext(context.Background())
				approvalDB.Statement.ConnPool = failedCommitPool{sqlDB}
			}
			h := &Handler{rRepo: rRepo.NewRewardRepository(approvalDB, nil), circleRepo: cRepo.NewCircleRepository(db), eventProducer: publisher}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("id", &uModel.UserDetails{User: uModel.User{ID: 1, CircleID: scenario.circleID}, WebhookURL: &url})
			})
			router.POST("/redemptions/:id/approve", h.approveRedemption)
			request := func() int {
				res := httptest.NewRecorder()
				router.ServeHTTP(res, httptest.NewRequest("POST", "/redemptions/7/approve", nil))
				return res.Code
			}
			require.Equal(t, scenario.want, request())
			if scenario.want == 200 {
				require.Equal(t, 1, calls)
				require.Equal(t, 400, request())
				require.Equal(t, 1, calls, "repeated approval must not publish again")
			} else {
				require.Zero(t, calls)
				var stored rModel.RewardRedemption
				require.NoError(t, db.First(&stored, 7).Error)
				require.Equal(t, scenario.status, stored.Status)
				var member cModel.UserCircle
				require.NoError(t, db.Where("user_id = ?", 2).First(&member).Error)
				require.Zero(t, member.PointsRedeemed, "failed approval must roll back the debit")
				if !scenario.rollback {
					var count int64
					require.NoError(t, db.Model(&pModel.PointsHistory{}).Count(&count).Error)
					require.Zero(t, count, "failed approval must not leave points history")
				}
			}
		})
	}
}
