package points

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	"gorm.io/gorm"
)

var (
	ErrInvalidXP         = errors.New("XP must be a positive integer within the supported limit")
	ErrCategoryRequired  = errors.New("choose an XP category")
	ErrInvalidCategory   = errors.New("XP category is not available in this circle")
	ErrInsufficientXP    = errors.New("insufficient category or total balance")
	ErrBalanceLimit      = errors.New("XP counter limit reached")
	ErrMembershipChanged = errors.New("membership changed since this request was created")
)

const MaxXP = 1000000000

// UserCircle's existing counters are int, including on supported ARMv6/7
// installations. Keep updates within their common representable range.
const MaxXPBalance = 2147483647
const MinXPBalance = -2147483648

func CategoryName(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	key := strings.ToLower(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || key == "uncategorized" || strings.ContainsAny(name, "\r\n\x00") {
		return "", "", errors.New("category name must contain 1–80 characters and cannot be Uncategorized")
	}
	return name, key, nil
}

// Category zero denotes the legacy Uncategorized balance in API requests.
func ValidateCategory(db *gorm.DB, circleID int, id *int, activeOnly bool) error {
	if id == nil || *id == 0 {
		return nil
	}
	if *id < 0 {
		return ErrInvalidCategory
	}
	var category pModel.XPCategory
	q := db.Where("id = ? AND circle_id = ?", *id, circleID)
	if activeOnly {
		q = q.Where("archived = ?", false)
	}
	if err := q.First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidCategory
		}
		return err
	}
	return nil
}

func (r *PointsRepository) GetCategories(ctx context.Context, circleID int) ([]pModel.XPCategory, error) {
	rows := make([]pModel.XPCategory, 0)
	err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("id asc").Find(&rows).Error
	return rows, err
}

// Stats uses original member counters as the authoritative total. All values
// predating categories remain Uncategorized, without guessing from task history.
func Stats(db *gorm.DB, circleID, userID int) (*pModel.XPStats, error) {
	// One SQL snapshot keeps the authoritative counters and category sums from
	// different sides of a concurrent award/redemption from being mixed.
	var rows []struct {
		Points         int64
		PointsRedeemed int64
		CategoryID     *int
		Name           string
		Archived       bool
		Earned         int64
		Redeemed       int64
	}
	err := db.Table("user_circles AS uc").Select(`uc.points, uc.points_redeemed, cat.id AS category_id, COALESCE(cat.name, '') AS name,
		COALESCE(cat.archived, false) AS archived, COALESCE(SUM(entry.earned), 0) AS earned, COALESCE(SUM(entry.redeemed), 0) AS redeemed`).
		Joins("LEFT JOIN xp_categories AS cat ON cat.circle_id = uc.circle_id").
		Joins("LEFT JOIN xp_entries AS entry ON entry.membership_id = uc.id AND entry.category_id = cat.id").
		Where("uc.circle_id = ? AND uc.user_id = ? AND uc.is_active = ?", circleID, userID, true).
		Group("uc.id, uc.points, uc.points_redeemed, cat.id, cat.name, cat.archived").Order("cat.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	result := &pModel.XPStats{UserID: userID, CircleID: circleID, TotalXP: rows[0].Points - rows[0].PointsRedeemed, LifetimeXP: rows[0].Points, RedeemedXP: rows[0].PointsRedeemed, Categories: map[string]int64{}, CategoryDetails: make([]pModel.XPCategoryTotal, 0, len(rows)+1)}
	legacy := pModel.XPCategoryTotal{ID: 0, Name: "Uncategorized", Earned: rows[0].Points, Redeemed: rows[0].PointsRedeemed}
	for _, cat := range rows {
		if cat.CategoryID == nil {
			continue
		}
		t := pModel.XPCategoryTotal{ID: *cat.CategoryID, Name: cat.Name, Archived: cat.Archived, Earned: cat.Earned, Redeemed: cat.Redeemed, Balance: cat.Earned - cat.Redeemed}
		legacy.Earned -= t.Earned
		legacy.Redeemed -= t.Redeemed
		result.Categories[strconv.Itoa(t.ID)] = t.Balance
		result.CategoryDetails = append(result.CategoryDetails, t)
	}
	legacy.Balance = legacy.Earned - legacy.Redeemed
	result.Categories["0"] = legacy.Balance
	result.CategoryDetails = append([]pModel.XPCategoryTotal{legacy}, result.CategoryDetails...)
	return result, nil
}

func (r *PointsRepository) GetStats(ctx context.Context, circleID, userID int) (*pModel.XPStats, error) {
	var result *pModel.XPStats
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; result, err = Stats(tx, circleID, userID); return err })
	return result, err
}

// RecordDelta must run in the SAME transaction as the corresponding update of
// user_circles. Unique event keys make repeated award/reversal operations fail
// and roll back their counter changes instead of silently awarding twice.
func RecordDelta(tx *gorm.DB, circleID, userID int, categoryID *int, earned, redeemed int, eventKey string) error {
	if err := ValidateCategory(tx, circleID, categoryID, false); err != nil {
		return err
	}
	if categoryID != nil && *categoryID == 0 {
		categoryID = nil
	}
	var member cModel.UserCircle
	if err := tx.Where("circle_id = ? AND user_id = ? AND is_active = ?", circleID, userID, true).First(&member).Error; err != nil {
		return err
	}
	return tx.Create(&pModel.XPEntry{MembershipID: member.ID, CircleID: circleID, UserID: userID, CategoryID: categoryID, Earned: earned, Redeemed: redeemed, EventKey: eventKey}).Error
}

// LockMember serializes all spends for a member, including distinct reward
// requests. A write lock also works on SQLite, where SELECT FOR UPDATE does not.
func LockMember(tx *gorm.DB, circleID, userID int, expectedIDs ...int) error {
	q := tx.Model(&cModel.UserCircle{}).Where("circle_id = ? AND user_id = ? AND is_active = ?", circleID, userID, true)
	if len(expectedIDs) > 0 && expectedIDs[0] != 0 {
		q = q.Where("id = ?", expectedIDs[0])
	}
	result := q.UpdateColumn("points", gorm.Expr("points"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func CheckSpend(tx *gorm.DB, circleID, userID, amount int, categoryID *int) error {
	return checkSpend(tx, circleID, userID, amount, categoryID, true)
}

func checkSpend(tx *gorm.DB, circleID, userID, amount int, categoryID *int, enforceMode bool) error {
	if amount <= 0 || amount > MaxXPBalance {
		return ErrInvalidXP
	}
	var circle cModel.Circle
	if err := tx.First(&circle, circleID).Error; err != nil {
		return err
	}
	if enforceMode && circle.XPMode && categoryID == nil {
		return ErrCategoryRequired
	}
	if err := ValidateCategory(tx, circleID, categoryID, false); err != nil {
		return err
	}
	stats, err := Stats(tx, circleID, userID)
	if err != nil {
		return err
	}
	if stats.TotalXP < int64(amount) {
		return ErrInsufficientXP
	}
	if categoryID != nil && stats.Categories[strconv.Itoa(*categoryID)] < int64(amount) {
		return ErrInsufficientXP
	}
	return nil
}

func Spend(tx *gorm.DB, circleID, userID, amount int, categoryID *int, eventKey string) error {
	return SpendWithPolicy(tx, circleID, userID, amount, categoryID, eventKey, true)
}

// Pending rewards requested in Points mode retain its deterministic allocation
// if the circle switches mode before approval. New XP requests choose a category.
func SpendWithPolicy(tx *gorm.DB, circleID, userID, amount int, categoryID *int, eventKey string, enforceMode bool) error {
	if err := LockMember(tx, circleID, userID); err != nil {
		return err
	}
	if err := checkSpend(tx, circleID, userID, amount, categoryID, enforceMode); err != nil {
		return err
	}
	stats, err := Stats(tx, circleID, userID)
	if err != nil {
		return err
	}
	result := tx.Model(&cModel.UserCircle{}).Where("circle_id = ? AND user_id = ? AND points - points_redeemed >= ? AND points_redeemed <= ?", circleID, userID, amount, MaxXPBalance-amount).UpdateColumn("points_redeemed", gorm.Expr("points_redeemed + ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInsufficientXP
	}
	if categoryID != nil {
		return RecordDelta(tx, circleID, userID, categoryID, 0, amount, eventKey)
	}
	// Old Points-mode clients do not supply a category. Consume Uncategorized
	// first, then named balances in ID order, rather than leaving spent XP in a
	// named category that can be spent a second time after earning elsewhere.
	left, part := int64(amount), 0
	for _, category := range stats.CategoryDetails {
		if left == 0 {
			break
		}
		if category.Balance <= 0 {
			continue
		}
		debit := category.Balance
		if debit > left {
			debit = left
		}
		key := eventKey
		if part > 0 {
			key = eventKey + ":part:" + strconv.Itoa(part)
		}
		id := category.ID
		if err := RecordDelta(tx, circleID, userID, &id, 0, int(debit), key); err != nil {
			return err
		}
		left -= debit
		part++
	}
	if left != 0 {
		return ErrInsufficientXP
	}
	return nil
}

func ChangeEarned(tx *gorm.DB, circleID, userID, delta int, categoryID *int, eventKey string, expectedMembershipID int) error {
	if delta == 0 || delta > MaxXPBalance || delta < -MaxXPBalance {
		return ErrInvalidXP
	}
	if err := LockMember(tx, circleID, userID, expectedMembershipID); err != nil {
		return err
	}
	q := tx.Model(&cModel.UserCircle{}).Where("circle_id = ? AND user_id = ?", circleID, userID)
	if delta > 0 {
		q = q.Where("points <= ?", MaxXPBalance-delta)
	} else {
		q = q.Where("points >= ?", MinXPBalance-delta)
	}
	res := q.UpdateColumn("points", gorm.Expr("points + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrBalanceLimit
	}
	return RecordDelta(tx, circleID, userID, categoryID, delta, 0, eventKey)
}
