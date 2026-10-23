package handler

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"donetick.com/core/internal/auth"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	pRepo "donetick.com/core/internal/points/repo"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db   *gorm.DB
	repo *pRepo.PointsRepository
}

func NewHandler(db *gorm.DB, repo *pRepo.PointsRepository) *Handler {
	return &Handler{db: db, repo: repo}
}

// Check active membership on every request; a previously issued token must not
// expose circle statistics or retain management rights after membership changes.
func (h *Handler) member(c *gin.Context, manage bool) (*cModel.UserCircle, bool) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return nil, false
	}
	var member cModel.UserCircle
	err := h.db.WithContext(c).Where("user_id = ? AND circle_id = ? AND is_active = ?", u.ID, u.CircleID, true).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Active circle membership required"})
		} else {
			c.JSON(500, gin.H{"error": "Unable to verify membership"})
		}
		return nil, false
	}
	if manage && member.Role != cModel.UserRoleAdmin && member.Role != cModel.UserRoleManager {
		c.JSON(403, gin.H{"error": "Circle admin or manager required"})
		return nil, false
	}
	return &member, true
}

func pathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return 0, false
	}
	return id, true
}

func apiError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "Not found in this circle"})
	case errors.Is(err, pRepo.ErrCategoryRequired), errors.Is(err, pRepo.ErrInvalidCategory), errors.Is(err, pRepo.ErrInsufficientXP), errors.Is(err, pRepo.ErrInvalidXP):
		c.JSON(400, gin.H{"error": err.Error()})
	case errors.Is(err, pRepo.ErrBalanceLimit), errors.Is(err, pRepo.ErrMembershipChanged):
		c.JSON(409, gin.H{"error": err.Error()})
	default:
		c.JSON(500, gin.H{"error": "Unable to update XP"})
	}
}

func (h *Handler) settings(c *gin.Context) {
	m, ok := h.member(c, false)
	if !ok {
		return
	}
	var circle cModel.Circle
	if err := h.db.WithContext(c).First(&circle, m.CircleID).Error; err != nil {
		apiError(c, err)
		return
	}
	c.JSON(200, gin.H{"xpMode": circle.XPMode})
}

func (h *Handler) updateSettings(c *gin.Context) {
	m, ok := h.member(c, true)
	if !ok {
		return
	}
	var req struct {
		XPMode *bool `json:"xpMode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "xpMode must be a boolean"})
		return
	}
	if err := h.db.WithContext(c).Model(&cModel.Circle{}).Where("id = ?", m.CircleID).Update("xp_mode", *req.XPMode).Error; err != nil {
		apiError(c, err)
		return
	}
	c.JSON(200, gin.H{"xpMode": *req.XPMode})
}

func (h *Handler) categories(c *gin.Context) {
	m, ok := h.member(c, false)
	if !ok {
		return
	}
	cats, err := h.repo.GetCategories(c, m.CircleID)
	if err != nil {
		apiError(c, err)
		return
	}
	c.JSON(200, cats)
}

func (h *Handler) saveCategory(c *gin.Context) {
	m, ok := h.member(c, true)
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Archived bool   `json:"archived"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid category"})
		return
	}
	name, key, err := pRepo.CategoryName(req.Name)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	category := pModel.XPCategory{CircleID: m.CircleID, Name: name, NameKey: key, Archived: req.Archived, CreatedAt: time.Now().UTC()}
	if c.Param("id") != "" {
		id, ok := pathID(c)
		if !ok {
			return
		}
		if err = h.db.WithContext(c).Where("id = ? AND circle_id = ?", id, m.CircleID).First(&category).Error; err != nil {
			apiError(c, err)
			return
		}
		category.Name, category.NameKey, category.Archived = name, key, req.Archived
	}
	// A unique circle/name key protects against concurrent duplicate creates.
	if err = h.db.WithContext(c).Save(&category).Error; err != nil {
		c.JSON(409, gin.H{"error": "Unable to save category; names must be unique within the circle"})
		return
	}
	c.JSON(200, category)
}

func (h *Handler) userXP(c *gin.Context) {
	m, ok := h.member(c, false)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	stats, err := h.repo.GetStats(c, m.CircleID, id)
	if err != nil {
		apiError(c, err)
		return
	}
	c.JSON(200, stats)
}

func (h *Handler) circleXP(c *gin.Context) {
	m, ok := h.member(c, false)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if id != m.CircleID {
		c.JSON(403, gin.H{"error": "Circle does not match the active membership"})
		return
	}
	var members []cModel.UserCircle
	if err := h.db.WithContext(c).Where("circle_id = ? AND is_active = ?", id, true).Order("user_id asc").Find(&members).Error; err != nil {
		apiError(c, err)
		return
	}
	users := make([]*pModel.XPStats, 0, len(members))
	for _, member := range members {
		stats, err := h.repo.GetStats(c, id, member.UserID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			apiError(c, err)
			return
		}
		users = append(users, stats)
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].TotalXP > users[j].TotalXP })
	c.JSON(200, gin.H{"circle_id": id, "users": users})
}

func (h *Handler) adjust(c *gin.Context) {
	m, ok := h.member(c, true)
	if !ok {
		return
	}
	if m.Role != cModel.UserRoleAdmin {
		c.JSON(403, gin.H{"error": "Only circle admins may adjust balances"})
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req struct {
		Points     int    `json:"points"`
		CategoryID *int   `json:"categoryId"`
		Action     string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Points <= 0 || req.Points > pRepo.MaxXP || (req.Action != "add" && req.Action != "remove") {
		c.JSON(400, gin.H{"error": "Use add or remove with a positive integer amount up to 1000000000"})
		return
	}
	if req.CategoryID == nil || (req.Action == "add" && *req.CategoryID <= 0) {
		apiError(c, pRepo.ErrCategoryRequired)
		return
	}
	requestKey := c.GetHeader("Idempotency-Key")
	if len(requestKey) == 0 || len(requestKey) > 128 {
		c.JSON(400, gin.H{"error": "Supply an Idempotency-Key of 1–128 characters for this adjustment"})
		return
	}
	key := fmt.Sprintf("manual:%d:%d:%x", m.CircleID, m.UserID, sha256.Sum256([]byte(requestKey)))
	conflict := false
	err := h.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := pRepo.LockMember(tx, m.CircleID, id); err != nil {
			return err
		}
		var existing pModel.XPEntry
		lookupErr := tx.Where("event_key = ?", key).First(&existing).Error
		if lookupErr == nil {
			category := 0
			if existing.CategoryID != nil {
				category = *existing.CategoryID
			}
			earned, redeemed := 0, 0
			if req.Action == "add" {
				earned = req.Points
			} else {
				redeemed = req.Points
			}
			if existing.UserID != id || category != *req.CategoryID || existing.Earned != earned || existing.Redeemed != redeemed {
				conflict = true
				return errors.New("idempotency key used for another adjustment")
			}
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		history := pModel.PointsHistory{CircleID: m.CircleID, UserID: id, CreatedBy: m.UserID, Points: req.Points, XPCategoryID: req.CategoryID}
		if req.Action == "add" {
			if err := pRepo.ValidateCategory(tx, m.CircleID, req.CategoryID, true); err != nil {
				return err
			}
			history.Action = pModel.PointsHistoryActionAdd
			if err := pRepo.ChangeEarned(tx, m.CircleID, id, req.Points, req.CategoryID, key, 0); err != nil {
				return err
			}
		} else {
			history.Action = pModel.PointsHistoryActionRemove
			if err := pRepo.Spend(tx, m.CircleID, id, req.Points, req.CategoryID, key); err != nil {
				return err
			}
		}
		return tx.Create(&history).Error
	})
	if conflict {
		c.JSON(409, gin.H{"error": "This Idempotency-Key was used for a different adjustment"})
		return
	}
	if err != nil {
		apiError(c, err)
		return
	}
	c.JSON(200, gin.H{"res": fmt.Sprintf("XP %s recorded", req.Action)})
}

func Routes(r *gin.Engine, h *Handler, multi *auth.MultiAuthMiddleware) {
	g := r.Group("api/v1")
	g.Use(multi.MiddlewareFunc())
	g.GET("/xp/settings", h.settings)
	g.PUT("/xp/settings", h.updateSettings)
	g.GET("/xp/categories", h.categories)
	g.POST("/xp/categories", h.saveCategory)
	g.PUT("/xp/categories/:id", h.saveCategory)
	g.GET("/users/:id/xp", h.userXP)
	g.POST("/users/:id/xp/adjust", h.adjust)
	g.GET("/circles/:id/xp", h.circleXP)
}
