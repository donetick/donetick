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

	// Fetch one extra from each stream to determine if there are more changes.
	chores, err := h.choreRepo.GetChoreChangesSince(c, circleID, since, defaultSyncLimit+1)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch changes"})
		return
	}

	hasMoreChores := len(chores) > defaultSyncLimit
	if hasMoreChores {
		chores = chores[:defaultSyncLimit]
	}

	choreTombstones, err := h.choreRepo.GetTombstonesSince(c, circleID, syncModel.EntityTypeChore, since, defaultSyncLimit+1)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch deletions"})
		return
	}
	hasMoreChoreTombstones := len(choreTombstones) > defaultSyncLimit
	if hasMoreChoreTombstones {
		choreTombstones = choreTombstones[:defaultSyncLimit]
	}
	deletedChoreIDs := make([]int, 0, len(choreTombstones))
	for _, t := range choreTombstones {
		deletedChoreIDs = append(deletedChoreIDs, t.EntityID)
	}

	histories, err := h.choreRepo.GetChoreHistoryChangesSince(c, circleID, since, defaultSyncLimit+1)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch history changes"})
		return
	}
	hasMoreHistories := len(histories) > defaultSyncLimit
	if hasMoreHistories {
		histories = histories[:defaultSyncLimit]
	}

	historyTombstones, err := h.choreRepo.GetTombstonesSince(c, circleID, syncModel.EntityTypeChoreHistory, since, defaultSyncLimit+1)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch history deletions"})
		return
	}
	hasMoreHistoryTombstones := len(historyTombstones) > defaultSyncLimit
	if hasMoreHistoryTombstones {
		historyTombstones = historyTombstones[:defaultSyncLimit]
	}
	deletedHistoryIDs := make([]int, 0, len(historyTombstones))
	for _, t := range historyTombstones {
		deletedHistoryIDs = append(deletedHistoryIDs, t.EntityID)
	}

	hasMore := hasMoreChores || hasMoreHistories || hasMoreChoreTombstones || hasMoreHistoryTombstones

	cursor := since
	hasAny := false
	minLast := int64(0)

	if len(chores) > 0 {
		last := chores[len(chores)-1].SyncVersion
		if !hasAny || last < minLast {
			minLast = last
		}
		hasAny = true
	}
	if len(histories) > 0 {
		last := histories[len(histories)-1].SyncVersion
		if !hasAny || last < minLast {
			minLast = last
		}
		hasAny = true
	}
	if len(choreTombstones) > 0 {
		last := choreTombstones[len(choreTombstones)-1].SyncVersion
		if !hasAny || last < minLast {
			minLast = last
		}
		hasAny = true
	}
	if len(historyTombstones) > 0 {
		last := historyTombstones[len(historyTombstones)-1].SyncVersion
		if !hasAny || last < minLast {
			minLast = last
		}
		hasAny = true
	}

	if hasAny {
		cursor = minLast
	}

	c.JSON(200, gin.H{
		"changes": gin.H{
			"chores":         chores,
			"choreHistories": histories,
		},
		"deletions": gin.H{
			"chores":         deletedChoreIDs,
			"choreHistories": deletedHistoryIDs,
		},
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
