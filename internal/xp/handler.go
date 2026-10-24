package xp

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"donetick.com/core/internal/auth"
	circle "donetick.com/core/internal/circle/model"
	user "donetick.com/core/internal/user/model"
	xp "donetick.com/core/internal/xp/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct{ db *gorm.DB }

func NewHandler(db *gorm.DB) *Handler { return &Handler{db: db} }

// Membership is checked from storage on every request, rather than trusting a
// stale active-circle or role claim. No endpoint accepts another circle in a body.
func (h *Handler) member(c *gin.Context, manage bool) (*user.UserDetails, bool) {
	u, ok := auth.CurrentUser(c)
	if !ok || u.Disabled {
		c.JSON(401, gin.H{"error": "Authentication required"})
		return nil, false
	}
	var m circle.UserCircle
	err := h.db.WithContext(c).Where("circle_id = ? AND user_id = ? AND is_active = ?", u.CircleID, u.ID, true).First(&m).Error
	if err != nil {
		c.JSON(403, gin.H{"error": "Active circle membership required"})
		return nil, false
	}
	if manage && m.Role != circle.UserRoleAdmin && m.Role != circle.UserRoleManager {
		c.JSON(403, gin.H{"error": "Admin or manager required"})
		return nil, false
	}
	return u, true
}

func positiveID(c *gin.Context, name string) (int, bool) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return 0, false
	}
	return id, true
}

func (h *Handler) categories(c *gin.Context) {
	u, ok := h.member(c, false)
	if !ok {
		return
	}
	categories := []xp.Category{}
	if err := h.db.WithContext(c).Where("circle_id = ?", u.CircleID).Order("id").Find(&categories).Error; err != nil {
		c.JSON(500, gin.H{"error": "Unable to load categories"})
		return
	}
	c.JSON(200, gin.H{"res": categories})
}

func (h *Handler) saveCategory(c *gin.Context) {
	u, ok := h.member(c, true)
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "Invalid category"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 80 {
		c.JSON(400, gin.H{"error": "Category name must contain 1 to 80 characters"})
		return
	}
	category := xp.Category{CircleID: u.CircleID, Name: req.Name, NormalizedName: strings.ToLower(req.Name), Archived: req.Archived}
	if c.Request.Method == http.MethodPut {
		id, valid := positiveID(c, "id")
		if !valid {
			return
		}
		category.ID = id
		var existing xp.Category
		if err := h.db.WithContext(c).Where("id = ? AND circle_id = ?", id, u.CircleID).First(&existing).Error; err != nil {
			c.JSON(404, gin.H{"error": "Category not found"})
			return
		}
	}
	// The unique index arbitrates simultaneous creates/renames, not a racy count.
	var err error
	if category.ID == 0 {
		err = h.db.WithContext(c).Create(&category).Error
	} else {
		err = h.db.WithContext(c).Model(&xp.Category{}).Where("id = ? AND circle_id = ?", category.ID, u.CircleID).Updates(map[string]interface{}{"name": category.Name, "normalized_name": category.NormalizedName, "archived": category.Archived}).Error
	}
	if err != nil {
		c.JSON(409, gin.H{"error": "Category could not be saved; names must be unique within the circle"})
		return
	}
	c.JSON(200, gin.H{"res": category})
}

func (h *Handler) preference(c *gin.Context) {
	u, ok := h.member(c, false)
	if !ok {
		return
	}
	var req struct {
		XPMode *bool `json:"xpMode"`
	}
	if c.ShouldBindJSON(&req) != nil || req.XPMode == nil {
		c.JSON(400, gin.H{"error": "xpMode must be a boolean"})
		return
	}
	if err := h.db.WithContext(c).Model(&user.User{}).Where("id = ?", u.ID).Update("xp_mode", *req.XPMode).Error; err != nil {
		c.JSON(500, gin.H{"error": "Unable to save preference"})
		return
	}
	c.JSON(200, gin.H{"res": gin.H{"xpMode": *req.XPMode}})
}

type Statistics struct {
	UserID     int              `json:"user_id"`
	TotalXP    int64            `json:"total_xp"`
	Categories map[string]int64 `json:"categories"`
}

// One SQL statement reads balances and attribution from the same snapshot.
// Existing points with no ledger entry remain explicitly uncategorized. Rewards
// reduce points_redeemed only and must not reduce lifetime XP.
func (h *Handler) statistics(c *gin.Context, circleID int, userID *int) ([]Statistics, error) {
	var rows []struct {
		UserID     int
		TotalXP    int64
		CategoryID *int
		Earned     int64
	}
	q := h.db.WithContext(c).Table("user_circles AS m").
		Select("m.user_id, m.points AS total_xp, a.category_id, COALESCE(SUM(a.points), 0) AS earned").
		Joins("LEFT JOIN xp_awards AS a ON a.membership_id = m.id AND a.circle_id = m.circle_id AND a.user_id = m.user_id").
		Where("m.circle_id = ? AND m.is_active = ?", circleID, true)
	if userID != nil {
		q = q.Where("m.user_id = ?", *userID)
	}
	if err := q.Group("m.user_id, m.points, a.category_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := []Statistics{}
	indices := map[int]int{}
	for _, row := range rows {
		index, exists := indices[row.UserID]
		if !exists {
			index = len(result)
			indices[row.UserID] = index
			result = append(result, Statistics{UserID: row.UserID, TotalXP: row.TotalXP, Categories: map[string]int64{"uncategorized": row.TotalXP}})
		}
		if row.CategoryID != nil {
			key := fmt.Sprintf("category_%d", *row.CategoryID)
			result[index].Categories[key] += row.Earned
			result[index].Categories["uncategorized"] -= row.Earned
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].TotalXP == result[j].TotalXP {
			return result[i].UserID < result[j].UserID
		}
		return result[i].TotalXP > result[j].TotalXP
	})
	return result, nil
}
func (h *Handler) userXP(c *gin.Context) {
	u, ok := h.member(c, false)
	if !ok {
		return
	}
	id, ok := positiveID(c, "userID")
	if !ok {
		return
	}
	stats, err := h.statistics(c, u.CircleID, &id)
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to load XP"})
		return
	}
	if len(stats) == 0 {
		c.JSON(404, gin.H{"error": "Active circle member not found"})
		return
	}
	c.JSON(200, stats[0])
}
func (h *Handler) circleXP(c *gin.Context) {
	u, ok := h.member(c, false)
	if !ok {
		return
	}
	id, ok := positiveID(c, "circleID")
	if !ok {
		return
	}
	if id != u.CircleID {
		c.JSON(403, gin.H{"error": "Current circle required"})
		return
	}
	stats, err := h.statistics(c, id, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": "Unable to load XP"})
		return
	}
	c.JSON(200, gin.H{"circle_id": id, "leaderboard": stats})
}
func Routes(r *gin.Engine, h *Handler, middleware *auth.MultiAuthMiddleware) {
	group := r.Group("/api/v1", middleware.MiddlewareFunc())
	group.GET("/users/:userID/xp", h.userXP)
	group.GET("/circles/:circleID/xp", h.circleXP)
	group.GET("/xp/categories", h.categories)
	group.POST("/xp/categories", h.saveCategory)
	group.PUT("/xp/categories/:id", h.saveCategory)
	group.PUT("/xp/preference", h.preference)
}
