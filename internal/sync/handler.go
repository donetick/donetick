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

// GetChanges godoc
//
//	@Summary		Get sync changes
//	@Description	Returns chores and chore histories that have changed since the given sync version cursor, along with deleted entity IDs. Paginated — call repeatedly while hasMore is true using the returned cursor.
//	@Tags			sync
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth
//	@Security		APIKeyAuth
//	@Param			since	query		int64		false	"Sync version cursor (0 for initial full sync)"	default(0)
//	@Success		200		{object}	map[string]interface{}	"changes, deletions, cursor, hasMore"
//	@Failure		400		{object}	map[string]string		"error: Invalid 'since' parameter"
//	@Failure		401		{object}	map[string]string		"error: Authentication failed"
//	@Failure		500		{object}	map[string]string		"error: Failed to fetch changes"
//	@Router			/sync/changes [get]
func (h *SyncHandler) GetChanges(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	circleID := currentUser.CircleID
	userID := currentUser.ID

	sinceStr := c.DefaultQuery("since", "0")
	since, err := strconv.ParseInt(sinceStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid 'since' parameter"})
		return
	}

	// Fetch one extra from each stream to determine if there are more changes.
	// Use sync-aware GetChores with privacy filtering.
	syncOpts := &chRepo.SyncOptions{
		SyncVersion: &since,
		Limit:       defaultSyncLimit,
	}
	chores, err := h.choreRepo.GetChores(c, circleID, userID, true, syncOpts, false)
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

	// Use sync-aware GetChoresHistoryByCircle with privacy filtering.
	histories, err := h.choreRepo.GetChoresHistoryByCircle(c, circleID, userID, syncOpts)
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

	hasMore := hasMoreChores || hasMoreHistories || hasMoreChoreTombstones || hasMoreHistoryTombstones

	// Compute the next cursor:
	// - If any stream was truncated (hasMore* == true), the cursor is the minimum
	//   last sync_version among truncated streams — we must not skip their unsent items.
	//   Then cap every returned stream to <= cursor so the cursor means
	//   "everything <= cursor has been delivered".
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

			// Keep page boundary consistent across all streams.
			cappedChores := chores[:0]
			for _, item := range chores {
				if item.SyncVersion <= cursor {
					cappedChores = append(cappedChores, item)
				}
			}
			chores = cappedChores

			cappedHistories := histories[:0]
			for _, item := range histories {
				if item.SyncVersion <= cursor {
					cappedHistories = append(cappedHistories, item)
				}
			}
			histories = cappedHistories

			cappedChoreTombstones := choreTombstones[:0]
			for _, item := range choreTombstones {
				if item.SyncVersion <= cursor {
					cappedChoreTombstones = append(cappedChoreTombstones, item)
				}
			}
			choreTombstones = cappedChoreTombstones

			cappedHistoryTombstones := historyTombstones[:0]
			for _, item := range historyTombstones {
				if item.SyncVersion <= cursor {
					cappedHistoryTombstones = append(cappedHistoryTombstones, item)
				}
			}
			historyTombstones = cappedHistoryTombstones
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

	deletedChoreIDs := make([]int, 0, len(choreTombstones))
	for _, t := range choreTombstones {
		deletedChoreIDs = append(deletedChoreIDs, t.EntityID)
	}

	deletedHistoryIDs := make([]int, 0, len(historyTombstones))
	for _, t := range historyTombstones {
		deletedHistoryIDs = append(deletedHistoryIDs, t.EntityID)
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
		syncRoutes.GET("/changes", h.GetChanges)
	}
}
