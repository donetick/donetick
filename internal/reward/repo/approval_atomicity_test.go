package repo

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	rModel "donetick.com/core/internal/reward/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func approvalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "approval.db")+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&cModel.UserCircle{}, &pModel.PointsHistory{}, &rModel.RewardRedemption{}))
	seedMember(t, db, 1, 1, 100, 0)
	return db
}

func TestConcurrentRedemptionResolution(t *testing.T) {
	for _, scenario := range []string{"same redemption", "competing balance", "approval versus rejection"} {
		t.Run(scenario, func(t *testing.T) {
			db := approvalTestDB(t)
			r := NewRewardRepository(db, nil)
			require.NoError(t, db.Create(&rModel.RewardRedemption{ID: 1, CircleID: 1, UserID: 1, PointsCost: 75}).Error)
			if scenario == "competing balance" {
				require.NoError(t, db.Create(&rModel.RewardRedemption{ID: 2, CircleID: 1, UserID: 1, PointsCost: 75}).Error)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					if scenario == "approval versus rejection" && i == 1 {
						results <- r.RejectRedemption(context.Background(), 1, 1, 999, nil)
						return
					}
					id := 1
					if scenario == "competing balance" {
						id += i
					}
					_, err := r.ApproveRedemption(context.Background(), id, 1, 999)
					results <- err
				}(i)
			}
			close(start)
			wg.Wait()
			close(results)
			successes := 0
			for err := range results {
				if err == nil {
					successes++
				} else {
					// SQLite can reject a competing transaction's snapshot upgrade.
					busy := strings.Contains(strings.ToLower(err.Error()), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY")
					require.True(t, busy || errors.Is(err, ErrRedemptionNotPending) || errors.Is(err, ErrInsufficientPoints), "unexpected competing resolution error: %v", err)
				}
			}
			require.Equal(t, 1, successes, "only one competing resolution may succeed")
			var approved, histories int64
			require.NoError(t, db.Model(&rModel.RewardRedemption{}).Where("status = ?", rModel.RedemptionStatusApproved).Count(&approved).Error)
			require.NoError(t, db.Model(&pModel.PointsHistory{}).Count(&histories).Error)
			require.Equal(t, approved, histories)
			var member cModel.UserCircle
			require.NoError(t, db.Where("user_id = ?", 1).First(&member).Error)
			require.Equal(t, int(approved)*75, member.PointsRedeemed)
		})
	}
}

// Inject a competing write after the snapshot read. Unlike a timing-based race,
// this exercises both conditional writes on every run, including SQLite.
func TestApprovalRejectsStaleStateAndBalance(t *testing.T) {
	for _, table := range []string{"reward_redemptions", "user_circles"} {
		t.Run(table, func(t *testing.T) {
			db := approvalTestDB(t)
			require.NoError(t, db.Create(&rModel.RewardRedemption{ID: 1, CircleID: 1, UserID: 1, PointsCost: 75}).Error)
			injected := false
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:competing_write", func(tx *gorm.DB) {
				if injected || tx.Statement.Table != table {
					return
				}
				injected = true
				var err error
				writer := tx.Session(&gorm.Session{NewDB: true})
				if table == "reward_redemptions" {
					err = writer.Exec("UPDATE reward_redemptions SET status = ? WHERE id = 1", rModel.RedemptionStatusRejected).Error
				} else {
					err = writer.Exec("UPDATE user_circles SET points_redeemed = 75 WHERE user_id = 1").Error
				}
				if err != nil {
					tx.AddError(err)
				}
			}))
			redemption, err := NewRewardRepository(db, nil).ApproveRedemption(context.Background(), 1, 1, 999)
			want := ErrRedemptionNotPending
			if table == "user_circles" {
				want = ErrInsufficientPoints
			}
			require.ErrorIs(t, err, want)
			require.Nil(t, redemption)
			require.True(t, injected)
			var stored rModel.RewardRedemption
			require.NoError(t, db.First(&stored, 1).Error)
			require.Equal(t, rModel.RedemptionStatusPending, stored.Status)
			var member cModel.UserCircle
			require.NoError(t, db.Where("user_id = ?", 1).First(&member).Error)
			require.Zero(t, member.PointsRedeemed)
			var histories int64
			require.NoError(t, db.Model(&pModel.PointsHistory{}).Count(&histories).Error)
			require.Zero(t, histories)
		})
	}
}
