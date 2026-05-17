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

	// Compute the next cursor:
	// - If any stream was truncated (hasMore* == true), the cursor is the minimum
	//   last sync_version among truncated streams — we must not skip their unsent items.
	// - If all streams are fully drained, the cursor advances to the maximum last
	//   sync_version across all streams, so the next request truly starts fresh.
	cursor := since
	if hasMore {
		first := true
		minTruncated := int64(0)
		if hasMoreChores && len(chores) > 0 {
			v := chores[len(chores)-1].SyncVersion
			if first || v < minTruncated {
				minTruncated = v
				first = false
			}
		}
		if hasMoreHistories && len(histories) > 0 {
			v := histories[len(histories)-1].SyncVersion
			if first || v < minTruncated {
				minTruncated = v
				first = false
			}
		}
		if hasMoreChoreTombstones && len(choreTombstones) > 0 {
			v := choreTombstones[len(choreTombstones)-1].SyncVersion
			if first || v < minTruncated {
				minTruncated = v
				first = false
			}
		}
		if hasMoreHistoryTombstones && len(historyTombstones) > 0 {
			v := historyTombstones[len(historyTombstones)-1].SyncVersion
			if first || v < minTruncated {
				minTruncated = v
				first = false
			}
		}
		if !first {
			cursor = minTruncated
		}
	} else {
		// All streams fully drained — advance to the max version seen.
		hasAny := false
		maxAll := int64(0)
		updateMax := func(v int64) {
			if v > 0 && (!hasAny || v > maxAll) {
				maxAll = v
				hasAny = true
			}
		}
		if len(chores) > 0 {
			updateMax(chores[len(chores)-1].SyncVersion)
		}
		if len(histories) > 0 {
			updateMax(histories[len(histories)-1].SyncVersion)
		}
		if len(choreTombstones) > 0 {
			updateMax(choreTombstones[len(choreTombstones)-1].SyncVersion)
		}
		if len(historyTombstones) > 0 {
			updateMax(historyTombstones[len(historyTombstones)-1].SyncVersion)
		}
		if hasAny {
			cursor = maxAll
		}
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
