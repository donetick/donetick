package points

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func xpFixture(t *testing.T, legacy int) (*gorm.DB, *cModel.UserCircle, int, int) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err = db.AutoMigrate(&cModel.Circle{}, &cModel.UserCircle{}, &pModel.XPCategory{}, &pModel.XPEntry{}); err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&cModel.Circle{ID: 1, XPMode: true}).Error; err != nil {
		t.Fatal(err)
	}
	member := &cModel.UserCircle{CircleID: 1, UserID: 1, IsActive: true, Points: legacy}
	if err = db.Create(member).Error; err != nil {
		t.Fatal(err)
	}
	catA := pModel.XPCategory{CircleID: 1, Name: "Learning", NameKey: "learning"}
	catB := pModel.XPCategory{CircleID: 1, Name: "Fitness", NameKey: "fitness"}
	if err = db.Create(&catA).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&catB).Error; err != nil {
		t.Fatal(err)
	}
	return db, member, catA.ID, catB.ID
}

func mustXP(t *testing.T, db *gorm.DB) *pModel.XPStats {
	t.Helper()
	stats, err := Stats(db, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, amount := range stats.Categories {
		sum += amount
	}
	if sum != stats.TotalXP {
		t.Fatalf("category sum %d != total %d", sum, stats.TotalXP)
	}
	return stats
}

func TestXPOriginalAwardsAndLegacyBalance(t *testing.T) {
	db, member, a, b := xpFixture(t, 75)
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 50, &a, "award", member.ID) }); err != nil {
		t.Fatal(err)
	}
	// Renaming/archiving a category never moves a past award.
	if err := db.Model(&pModel.XPCategory{}).Where("id = ?", a).Updates(map[string]interface{}{"name": "Study", "archived": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 20, &b, "later", member.ID) }); err != nil {
		t.Fatal(err)
	}
	stats := mustXP(t, db)
	if stats.TotalXP != 145 || stats.Categories["0"] != 75 || stats.Categories[strconv.Itoa(a)] != 50 {
		t.Fatalf("wrong original balances: %+v", stats)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, -50, &a, "undo", member.ID) }); err != nil {
		t.Fatal(err)
	}
	stats = mustXP(t, db)
	if stats.TotalXP != 95 || stats.Categories[strconv.Itoa(a)] != 0 || stats.Categories[strconv.Itoa(b)] != 20 {
		t.Fatalf("wrong reversal: %+v", stats)
	}
}

func TestXPExplicitCategoryAndAtomicRollback(t *testing.T) {
	db, m, a, b := xpFixture(t, 10)
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 50, &a, "award", m.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 20, &b, "wrong-category") }); !errors.Is(err, ErrInsufficientXP) {
		t.Fatalf("expected category limit, got %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 20, nil, "missing") }); !errors.Is(err, ErrCategoryRequired) {
		t.Fatalf("expected selected category, got %v", err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 20, &a, "spend") }); err != nil {
		t.Fatal(err)
	}
	// Same event must roll back its counter update as well as its ledger insert.
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 20, &a, "spend") }); err == nil {
		t.Fatal("duplicate spend accepted")
	}
	stats := mustXP(t, db)
	if stats.TotalXP != 40 || stats.RedeemedXP != 20 || stats.Categories[strconv.Itoa(a)] != 30 {
		t.Fatalf("duplicate/failed spend changed balance: %+v", stats)
	}
}

func TestXPLegacySpendConsumesNamedBalances(t *testing.T) {
	db, m, a, b := xpFixture(t, 10)
	if err := db.Model(&cModel.Circle{}).Where("id = ?", 1).Update("xp_mode", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 100, &a, "award-a", m.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 110, nil, "legacy-spend") }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 100, &b, "award-b", m.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&cModel.Circle{}).Where("id = ?", 1).Update("xp_mode", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 1, &a, "respend-a") }); !errors.Is(err, ErrInsufficientXP) {
		t.Fatalf("spent category resurrected: %v", err)
	}
	stats := mustXP(t, db)
	if stats.Categories["0"] != 0 || stats.Categories[strconv.Itoa(a)] != 0 || stats.Categories[strconv.Itoa(b)] != 100 {
		t.Fatalf("wrong allocation: %+v", stats)
	}
}

func TestXPRejoinedMembershipCannotResurrectAwards(t *testing.T) {
	db, m, a, b := xpFixture(t, 0)
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 100, &a, "old", m.ID) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(m).Error; err != nil {
		t.Fatal(err)
	}
	fresh := cModel.UserCircle{CircleID: 1, UserID: 1, IsActive: true}
	if err := db.Create(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.ID == m.ID {
		t.Fatal("membership fixture must have a new incarnation")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 100, &b, "new", fresh.ID) }); err != nil {
		t.Fatal(err)
	}
	stats := mustXP(t, db)
	if stats.Categories[strconv.Itoa(a)] != 0 || stats.Categories["0"] != 0 || stats.TotalXP != 100 {
		t.Fatalf("old membership leaked: %+v", stats)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 1, &a, "old-pending", m.ID) }); err == nil {
		t.Fatal("old pending award credited new membership")
	}
}

func TestXPCounterBoundsAndLargeCategorySums(t *testing.T) {
	db, m, a, _ := xpFixture(t, 0)
	for _, key := range []string{"one", "two"} {
		if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, MaxXP, &a, key, m.ID) }); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, MaxXP, &a, "three", m.ID) }); !errors.Is(err, ErrBalanceLimit) {
		t.Fatalf("counter overflow was not rejected: %v", err)
	}
	if stats := mustXP(t, db); stats.TotalXP != 2000000000 {
		t.Fatalf("failed update changed total: %+v", stats)
	}
	// Category aggregates remain 64-bit even on supported 32-bit builds.
	if err := db.Create(&pModel.XPEntry{MembershipID: m.ID, CircleID: 1, UserID: 1, CategoryID: &a, Earned: MaxXP, EventKey: "legacy-large-sum"}).Error; err != nil {
		t.Fatal(err)
	}
	if stats := mustXP(t, db); stats.Categories[strconv.Itoa(a)] != 3000000000 {
		t.Fatal("category sum overflowed")
	}
}

func TestXPConcurrentSpendsCannotOverdraw(t *testing.T) {
	db, m, a, _ := xpFixture(t, 0)
	if err := db.Transaction(func(tx *gorm.DB) error { return ChangeEarned(tx, 1, 1, 100, &a, "seed", m.ID) }); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- db.Transaction(func(tx *gorm.DB) error { return Spend(tx, 1, 1, 80, &a, "concurrent-"+strconv.Itoa(i)) })
		}(i)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly one successful spend, got %d", success)
	}
	stats := mustXP(t, db)
	if stats.TotalXP != 20 || stats.RedeemedXP != 80 {
		t.Fatalf("concurrent overspend: %+v", stats)
	}
}
