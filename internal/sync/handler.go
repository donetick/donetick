package sync

import (
	"strconv"

	auth "donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	syncModel "donetick.com/core/internal/sync/model"
	"github.com/gin-gonic/gin"
)

const defaultSyncLimit = 200

type SyncHandler struct {
	choreRepo *chRepo.ChoreRepository
}

func NewHandler(cr *chRepo.ChoreRepository) *SyncHandler {
	return &SyncHandler{
		choreRepo: cr,
	}
}

func (h *SyncHandler) getChanges(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	circleID := currentUser.CircleID

	sinceStr := c.DefaultQuery("since", "0")
	since, err := strconv.ParseInt(sinceStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid 'since' parameter"})
		return
	}

	// Fetch one extra to determine hasMore
	chores, err := h.choreRepo.GetChoreChangesSince(c, circleID, since, defaultSyncLimit+1)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch changes"})
		return
	}

	hasMore := len(chores) > defaultSyncLimit
	if hasMore {
		chores = chores[:defaultSyncLimit]
	}

	var cursor int64
	if len(chores) > 0 {
		cursor = chores[len(chores)-1].SyncVersion
	} else {
		cursor = since
	}

	tombstones, err := h.choreRepo.GetTombstonesSince(c, circleID, syncModel.EntityTypeChore, since)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch deletions"})
		return
	}

	deletedIDs := make([]int, 0, len(tombstones))
	for _, t := range tombstones {
		deletedIDs = append(deletedIDs, t.EntityID)
	}

	c.JSON(200, gin.H{
		"changes":  gin.H{"chores": chores},
		"deletions": gin.H{"chores": deletedIDs},
		"cursor":  cursor,
		"hasMore": hasMore,
	})
}

func Routes(router *gin.Engine, h *SyncHandler, auth *auth.MultiAuthMiddleware) {
	syncRoutes := router.Group("api/v1/sync")
	syncRoutes.Use(auth.MiddlewareFunc())
	{
		syncRoutes.GET("/changes", h.getChanges)
	}
}
