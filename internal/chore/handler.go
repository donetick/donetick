package chore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	auth "donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	circle "donetick.com/core/internal/circle/model"
	cRepo "donetick.com/core/internal/circle/repo"
	dRepo "donetick.com/core/internal/device/repo"
	"donetick.com/core/internal/events"
	lRepo "donetick.com/core/internal/label/repo"
	"donetick.com/core/internal/notifier"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	fcmService "donetick.com/core/internal/notifier/service/fcm"
	"donetick.com/core/internal/realtime"
	storage "donetick.com/core/internal/storage"
	storageModel "donetick.com/core/internal/storage/model"
	storageRepo "donetick.com/core/internal/storage/repo"
	stModel "donetick.com/core/internal/subtask/model"
	stRepo "donetick.com/core/internal/subtask/repo"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	choreRepo       *chRepo.ChoreRepository
	circleRepo      *cRepo.CircleRepository
	notifier        *notifier.Notifier
	nPlanner        *nps.NotificationPlanner
	nRepo           *nRepo.NotificationRepository
	tRepo           *tRepo.ThingRepository
	lRepo           *lRepo.LabelRepository
	uRepo           *uRepo.UserRepository
	deviceRepo      *dRepo.DeviceRepository
	eventProducer   *events.EventsProducer
	stRepo          *stRepo.SubTasksRepository
	storageRepo     *storageRepo.StorageRepository
	storage         *storage.S3Storage
	realTimeService *realtime.RealTimeService
}

func NewHandler(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository, nt *notifier.Notifier,
	np *nps.NotificationPlanner, nRepo *nRepo.NotificationRepository, tRepo *tRepo.ThingRepository, lRepo *lRepo.LabelRepository,
	ep *events.EventsProducer, stRepo *stRepo.SubTasksRepository,
	storage *storage.S3Storage,
	ur *uRepo.UserRepository,
	dr *dRepo.DeviceRepository,
	stoRepo *storageRepo.StorageRepository,
	rts *realtime.RealTimeService) *Handler {
	return &Handler{
		choreRepo:       cr,
		uRepo:           ur,
		deviceRepo:      dr,
		circleRepo:      circleRepo,
		notifier:        nt,
		nPlanner:        np,
		nRepo:           nRepo,
		tRepo:           tRepo,
		lRepo:           lRepo,
		eventProducer:   ep,
		stRepo:          stRepo,
		storageRepo:     stoRepo,
		storage:         storage,
		realTimeService: rts,
	}
}

// GetChores godoc
//
//	@Summary		Get all chores
//	@Description	Retrieves all chores for the current user's circle with optional archived chores
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			includeArchived	query		boolean						false	"Include archived chores"
//	@Success		200				{object}	map[string][]model.Chore	"res: array of chores"
//	@Failure		401				{object}	map[string]string			"error: Authentication failed"
//	@Failure		500				{object}	map[string]string			"error: Failed to retrieve chores"
//	@Router			/chores [get]
func (h *Handler) getChores(c *gin.Context) {
	logger := logging.FromContext(c)
	u, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	includeArchived := false

	if c.Query("includeArchived") == "true" {
		includeArchived = true
	}

	chores, err := h.choreRepo.GetChores(c, u.CircleID, u.ID, includeArchived)
	if err != nil {
		logger.Error("Failed to retrieve chores", "error", err, "userID", u.ID, "circleID", u.CircleID, "includeArchived", includeArchived)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chores",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chores,
	})
}

// GetArchivedChores godoc
//
//	@Summary		Get archived chores
//	@Description	Retrieves all archived chores for the current user's circle
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Success		200	{object}	map[string][]model.Chore	"res: array of archived chores"
//	@Failure		401	{object}	map[string]string			"error: Authentication failed"
//	@Failure		500	{object}	map[string]string			"error: Failed to retrieve archived chores"
//	@Router			/chores/archived [get]
func (h *Handler) getArchivedChores(c *gin.Context) {
	logger := logging.FromContext(c)
	u, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	chores, err := h.choreRepo.GetArchivedChores(c, u.CircleID, u.ID)
	if err != nil {
		logger.Error("Failed to retrieve archived chores", "error", err, "userID", u.ID, "circleID", u.CircleID)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve archived chores",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chores,
	})
}

// GetChore godoc
//
//	@Summary		Get a specific chore
//	@Description	Retrieves detailed information about a specific chore by ID
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]model.Chore	"res: chore object"
//	@Failure		400	{object}	map[string]string		"error: Invalid chore ID"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		403	{object}	map[string]string		"error: You are not allowed to view this chore"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore"
//	@Router			/chores/{id} [get]
func (h *Handler) getChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		logger.Error("Invalid chore ID format", "error", err, "rawID", rawID)
		c.JSON(400, gin.H{
			"error": "Invalid chore ID",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err, "choreID", id, "userID", currentUser.ID)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err, "circleID", currentUser.CircleID, "userID", currentUser.ID)
		c.JSON(500, gin.H{"error": "Failed to retrieve circle users"})
		return
	}

	if !chore.CanView(currentUser.ID, circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to view this chore",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
}

// CreateChore godoc
//
//	@Summary		Create a new chore
//	@Description	Creates a new chore with assignees, labels, and optional configurations
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			chore	body		model.ChoreReq		true	"Chore creation request"
//	@Success		200		{object}	map[string]int		"res: created chore ID"
//	@Failure		400		{object}	map[string]string	"error: Invalid request format | Assignee not found in circle | Invalid date"
//	@Failure		401		{object}	map[string]string	"error: Authentication failed"
//	@Failure		500		{object}	map[string]string	"error: Failed to create chore | Error adding labels | Error adding chore assignees"
//	@Router			/chores [post]
func (h *Handler) createChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)

	logger.Debug("Create chore", "currentUser", currentUser)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	// Validate chore:
	var choreReq chModel.ChoreReq
	if err := c.ShouldBindJSON(&choreReq); err != nil {
		logger.Error("Invalid request body", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err, "circleID", currentUser.CircleID, "userID", currentUser.ID)
		c.JSON(500, gin.H{"error": "Failed to retrieve circle users"})
		return
	}
	for _, assignee := range choreReq.Assignees {
		userFound := false
		for _, circleUser := range circleUsers {
			if assignee.UserID == circleUser.UserID {
				userFound = true
				break
			}
		}
		if !userFound {
			c.JSON(400, gin.H{
				"error": "Assignee not found in circle",
			})
			return
		}

	}
	// Remove the auto-assignment logic - if no assignee then keep no assignee

	var dueDate *time.Time

	if choreReq.DueDate != "" {
		rawDueDate, err := time.Parse(time.RFC3339, choreReq.DueDate)
		rawDueDate = rawDueDate.UTC()
		dueDate = &rawDueDate
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid date",
			})
			return
		}

	}

	createdChore := &chModel.Chore{

		Name:                   choreReq.Name,
		FrequencyType:          choreReq.FrequencyType,
		Frequency:              choreReq.Frequency,
		FrequencyMetadata:      nil, // deprecated in favor of FrequencyMetadataV2
		FrequencyMetadataV2:    choreReq.FrequencyMetadata,
		NextDueDate:            dueDate,
		AssignStrategy:         choreReq.AssignStrategy,
		AssignedTo:             choreReq.AssignedTo,
		IsRolling:              choreReq.IsRolling,
		UpdatedBy:              currentUser.ID,
		IsActive:               true,
		Notification:           choreReq.Notification,
		NotificationMetadata:   nil, // deprecated in favor of NotificationMetadataV2
		NotificationMetadataV2: choreReq.NotificationMetadata,
		Labels:                 nil, // deprecated in favor of LabelsV2
		CreatedBy:              currentUser.ID,
		CreatedAt:              time.Now().UTC(),
		CircleID:               currentUser.CircleID,
		Points:                 choreReq.Points,
		CompletionWindow:       choreReq.CompletionWindow,
		Description:            choreReq.Description,
		Priority:               choreReq.Priority,
		RequireApproval:        choreReq.RequireApproval,
		IsPrivate:              choreReq.IsPrivate,
		ProjectID:              choreReq.ProjectID,
		// SubTasks removed to prevent duplicate creation - handled by UpdateSubtask call below
		// it's need custom logic to handle subtask creation as we send negative ids sometimes when we creating parent child releationship
		// when the subtask is not yet created
	}
	id, err := h.choreRepo.CreateChore(c, createdChore)
	createdChore.ID = id

	if err != nil {
		logger.Error("Failed to create chore", "error", err, "userID", currentUser.ID, "circleID", currentUser.CircleID)
		c.JSON(500, gin.H{
			"error": "Failed to create chore",
		})
		return
	}

	if choreReq.SubTasks != nil {
		h.stRepo.UpdateSubtask(c, createdChore.ID, nil, *choreReq.SubTasks)
	}

	var choreAssignees []*chModel.ChoreAssignees
	for _, assignee := range choreReq.Assignees {
		choreAssignees = append(choreAssignees, &chModel.ChoreAssignees{
			ChoreID: id,
			UserID:  assignee.UserID,
		})
	}
	if choreReq.LabelsV2 != nil {
		labelsV2 := make([]int, len(*choreReq.LabelsV2))
		for i, label := range *choreReq.LabelsV2 {
			labelsV2[i] = int(label.LabelID)
		}
		if err := h.lRepo.AssignLabelsToChore(c, createdChore.ID, currentUser.ID, currentUser.CircleID, labelsV2, []int{}); err != nil {
			c.JSON(500, gin.H{
				"error": "Error adding labels",
			})
			return
		}
	}

	if choreReq.Description != nil {
		description := *choreReq.Description
		if err := h.cleanUpUnreferencedFiles(c, currentUser.ID, storageModel.EntityTypeChoreDescription, createdChore.ID, description); err != nil {
			c.JSON(500, gin.H{
				"error": "Error processing description",
			})
			return
		}
	}
	if len(choreAssignees) > 0 {
		if err := h.choreRepo.UpdateChoreAssignees(c, choreAssignees); err != nil {
			c.JSON(500, gin.H{
				"error": "Error adding chore assignees",
			})
			return
		}
	}
	go func() {
		h.nPlanner.GenerateNotifications(c, createdChore)
	}()

	// Broadcast real-time chore creation event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		broadcaster.BroadcastChoreCreated(createdChore, &currentUser.User)
	}

	shouldReturn := HandleThingAssociation(choreReq, createdChore, h, c, &currentUser.User)
	if shouldReturn {
		return
	}
	c.JSON(200, gin.H{
		"res": id,
	})
}

// EditChore godoc
//
//	@Summary		Edit an existing chore
//	@Description	Updates chore details including assignees, labels, subtasks, and metadata
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			chore	body		model.ChoreReq		true	"Chore update request"
//	@Success		200		{object}	map[string]string	"message: Chore added successfully"
//	@Failure		400		{object}	map[string]string	"error: Invalid request format | Invalid date | Assignee not found in circle | Assigned to not found in assignees"
//	@Failure		401		{object}	map[string]string	"error: Authentication failed"
//	@Failure		403		{object}	map[string]string	"error: You cannot edit this chore"
//	@Failure		500		{object}	map[string]string	"error: Failed to retrieve chore | Error adding chore | Error adding subtasks"
//	@Router			/chores [put]
func (h *Handler) editChore(c *gin.Context) {
	// logger := logging.FromContext(c)
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	var choreReq chModel.ChoreReq
	if err := c.ShouldBindJSON(&choreReq); err != nil {
		logger.Error("Invalid request body", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}

	existedChoreAssignees, err := h.choreRepo.GetChoreAssignees(c, choreReq.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore assignees",
		})
		return
	}

	var choreAssigneesToAdd []*chModel.ChoreAssignees
	var choreAssigneesToDelete []*chModel.ChoreAssignees

	//  filter assignees that not in the circle
	for _, assignee := range choreReq.Assignees {
		userFound := false
		for _, circleUser := range circleUsers {
			if assignee.UserID == circleUser.UserID {
				userFound = true
				break
			}
		}
		if !userFound {
			c.JSON(400, gin.H{
				"error": "Assignee not found in circle",
			})
			return
		}
		userAlreadyAssignee := false
		for _, existedChoreAssignee := range existedChoreAssignees {
			if existedChoreAssignee.UserID == assignee.UserID {
				userAlreadyAssignee = true
				break
			}
		}
		if !userAlreadyAssignee {
			choreAssigneesToAdd = append(choreAssigneesToAdd, &chModel.ChoreAssignees{
				ChoreID: choreReq.ID,
				UserID:  assignee.UserID,
			})
		}
	}

	//  remove assignees if they are not in the assignees list anymore
	for _, existedChoreAssignee := range existedChoreAssignees {
		userFound := false
		for _, assignee := range choreReq.Assignees {
			if existedChoreAssignee.UserID == assignee.UserID {
				userFound = true
				break
			}
		}
		if !userFound {
			choreAssigneesToDelete = append(choreAssigneesToDelete, existedChoreAssignee)
		}
	}

	var dueDate *time.Time

	if choreReq.DueDate != "" {
		rawDueDate, err := time.Parse(time.RFC3339, choreReq.DueDate)
		rawDueDate = rawDueDate.UTC()
		dueDate = &rawDueDate
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid date",
			})
			return
		}

	}

	//  validate assignedTo part of the assignees:
	if choreReq.AssignedTo != nil {
		assigneeFound := false
		for _, assignee := range choreReq.Assignees {
			if assignee.UserID == *choreReq.AssignedTo {
				assigneeFound = true
				break
			}
		}
		if !assigneeFound {
			c.JSON(400, gin.H{
				"error": "Assigned to not found in assignees",
			})
			return
		}
	}

	// Remove the auto-assignment logic - if no assignee then keep no assignee
	oldChore, err := h.choreRepo.GetChore(c, choreReq.ID, currentUser.ID)

	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	if err := oldChore.CanEdit(currentUser.ID, circleUsers, choreReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{
			"error": fmt.Sprintf("You cannot edit this chore: %s", err.Error()),
		})
		return
	}

	// Create a map to store the existing labels for quick lookup
	oldLabelsMap := make(map[int]struct{})
	for _, oldLabel := range *oldChore.LabelsV2 {
		oldLabelsMap[oldLabel.ID] = struct{}{}
	}
	newLabelMap := make(map[int]struct{})
	for _, newLabel := range *choreReq.LabelsV2 {
		newLabelMap[newLabel.LabelID] = struct{}{}
	}
	// check what labels need to be added and what labels need to be deleted:
	labelsV2ToAdd := make([]int, 0)
	labelsV2ToBeRemoved := make([]int, 0)

	for _, label := range *choreReq.LabelsV2 {
		if _, ok := oldLabelsMap[label.LabelID]; !ok {
			labelsV2ToAdd = append(labelsV2ToAdd, label.LabelID)
		}
	}
	for _, oldLabel := range *oldChore.LabelsV2 {
		if _, ok := newLabelMap[oldLabel.ID]; !ok {
			labelsV2ToBeRemoved = append(labelsV2ToBeRemoved, oldLabel.ID)
		}
	}

	if err := h.lRepo.AssignLabelsToChore(c, choreReq.ID, currentUser.ID, currentUser.CircleID, labelsV2ToAdd, labelsV2ToBeRemoved); err != nil {
		c.JSON(500, gin.H{
			"error": "Error adding labels",
		})
		return
	}
	description := *choreReq.Description
	if choreReq.Description == nil && oldChore.Description != nil {
		description = ""

	}
	if err := h.cleanUpUnreferencedFiles(c, currentUser.ID, storageModel.EntityTypeChoreDescription, choreReq.ID, description); err != nil {
		c.JSON(500, gin.H{
			"error": "Error processing description",
		})
		return
	}

	updatedChore := &chModel.Chore{
		ID:                  choreReq.ID,
		Name:                choreReq.Name,
		FrequencyType:       choreReq.FrequencyType,
		Frequency:           choreReq.Frequency,
		FrequencyMetadata:   nil, // deprecated in favor of FrequencyMetadataV2 v0.1.39
		FrequencyMetadataV2: choreReq.FrequencyMetadata,
		// Assignees:         &assignees,
		NextDueDate:            dueDate,
		AssignStrategy:         choreReq.AssignStrategy,
		AssignedTo:             choreReq.AssignedTo,
		IsRolling:              choreReq.IsRolling,
		IsActive:               choreReq.IsActive,
		Notification:           choreReq.Notification,
		NotificationMetadata:   nil, // deprecated in favor of NotificationMetadataV2 v0.1.39
		NotificationMetadataV2: choreReq.NotificationMetadata,
		Labels:                 nil, // deprecated in favor of LabelsV2 v0.1.39
		CircleID:               oldChore.CircleID,
		UpdatedBy:              currentUser.ID,
		CreatedBy:              oldChore.CreatedBy,
		CreatedAt:              oldChore.CreatedAt,
		Points:                 choreReq.Points,
		CompletionWindow:       choreReq.CompletionWindow,
		Description:            choreReq.Description,
		Priority:               choreReq.Priority,
		RequireApproval:        choreReq.RequireApproval,
		IsPrivate:              choreReq.IsPrivate,
		ProjectID:              choreReq.ProjectID,
		Status:                 oldChore.Status,
	}
	if err := h.choreRepo.UpsertChore(c, updatedChore); err != nil {
		c.JSON(500, gin.H{
			"error": "Error adding chore",
		})
		return
	}
	if choreReq.SubTasks != nil {
		ToBeRemoved := []stModel.SubTask{}
		ToBeAdded := []stModel.SubTask{}
		if oldChore.SubTasks == nil {
			oldChore.SubTasks = &[]stModel.SubTask{}
		}
		if choreReq.SubTasks == nil {
			choreReq.SubTasks = &[]stModel.SubTask{}
		}
		for _, existedSubTask := range *oldChore.SubTasks {
			found := false
			for _, newSubTask := range *choreReq.SubTasks {
				if existedSubTask.ID == newSubTask.ID {
					found = true
					break
				}
			}
			if !found {
				ToBeRemoved = append(ToBeRemoved, existedSubTask)
			}
		}

		for _, newSubTask := range *choreReq.SubTasks {
			found := false
			newSubTask.ChoreID = oldChore.ID

			for _, existedSubTask := range *oldChore.SubTasks {
				if existedSubTask.ID == newSubTask.ID {
					if existedSubTask.Name != newSubTask.Name || existedSubTask.OrderID != newSubTask.OrderID {
						// there is a change in the subtask, update it
						break
					}
					found = true
					break
				}
			}
			if !found {
				ToBeAdded = append(ToBeAdded, newSubTask)
			}
		}
		if err := h.stRepo.UpdateSubtask(c, oldChore.ID, ToBeRemoved, ToBeAdded); err != nil {
			c.JSON(500, gin.H{
				"error": "Error adding subtasks",
			})
			return
		}
	}

	if len(choreAssigneesToAdd) > 0 {
		err = h.choreRepo.UpdateChoreAssignees(c, choreAssigneesToAdd)

		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error updating chore assignees",
			})
			return
		}
	}
	if len(choreAssigneesToDelete) > 0 {
		err = h.choreRepo.DeleteChoreAssignees(c, choreAssigneesToDelete)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error deleting chore assignees",
			})
			return
		}
	}
	go func() {
		h.nPlanner.GenerateNotifications(c, updatedChore)
	}()

	// Broadcast real-time chore update event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Build changes map (simplified - in real implementation you might want to track actual changes)
		changes := map[string]interface{}{
			"updatedBy": currentUser.ID,
			"updatedAt": time.Now().UTC(),
		}
		broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
	}

	if oldChore.ThingChore != nil {
		// TODO: Add check to see if dissociation is necessary
		h.tRepo.DissociateThingWithChore(c, oldChore.ThingChore.ThingID, oldChore.ID)

	}
	shouldReturn := HandleThingAssociation(choreReq, updatedChore, h, c, &currentUser.User)
	if shouldReturn {
		return
	}

	c.JSON(200, gin.H{
		"message": "Chore added successfully",
	})
}

func (h *Handler) cleanUpUnreferencedFiles(ctx *gin.Context, userID int, entityType storageModel.EntityType, entityID int, text string) error {
	existedFiles, err := h.storageRepo.GetFilesByUser(ctx, userID, entityType, entityID)
	if err != nil {
		return err
	}
	referencedFiles := utils.ExtractImageURLs(text)
	var filesToBeDeleted []*storageModel.StorageFile
	var filePathsToBeDeleted []string
	for _, file := range existedFiles {
		found := false
		for _, refFile := range referencedFiles {
			if strings.Contains(refFile, file.FilePath) {
				found = true
				break
			}
		}
		if !found {
			// if the file is not referenced in the text, delete it
			filesToBeDeleted = append(filesToBeDeleted, file)
			filePathsToBeDeleted = append(filePathsToBeDeleted, file.FilePath)
		}
	}

	h.storage.Delete(ctx, filePathsToBeDeleted)
	h.storageRepo.RemoveFileRecords(ctx, filesToBeDeleted, userID)
	return nil
}

func HandleThingAssociation(choreReq chModel.ChoreReq, savedChore *chModel.Chore, h *Handler, c *gin.Context, currentUser *uModel.User) bool {
	if choreReq.ThingTrigger != nil {
		thing, err := h.tRepo.GetThingByID(c, choreReq.ThingTrigger.ID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting thing",
			})
			return true
		}
		if thing.UserID != currentUser.ID {
			c.JSON(403, gin.H{
				"error": "You are not allowed to trigger this thing",
			})
			return true
		}
		if err := h.tRepo.AssociateThingWithChore(c, choreReq.ThingTrigger.ID, savedChore.ID, choreReq.ThingTrigger.TriggerState, choreReq.ThingTrigger.Condition); err != nil {
			c.JSON(500, gin.H{
				"error": "Error associating thing with chore",
			})
			return true
		}

	}
	return false
}

// DeleteChore godoc
//
//	@Summary		Delete a chore
//	@Description	Deletes a chore and associated notifications (owner only)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Chore ID"
//	@Success		200	{object}	map[string]string	"message: Chore deleted successfully"
//	@Failure		400	{object}	map[string]string	"error: Invalid ID"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		403	{object}	map[string]string	"error: You are not allowed to delete this chore"
//	@Failure		500	{object}	map[string]string	"error: Failed to retrieve chore | Failed to delete chore"
//	@Router			/chores/{id} [delete]
func (h *Handler) deleteChore(c *gin.Context) {
	// logger := logging.FromContext(c)
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	// check if the user is the owner of the chore before deleting
	if err := h.choreRepo.IsChoreOwner(c, id, currentUser.ID); err != nil {
		c.JSON(403, gin.H{
			"error": "You are not allowed to delete this chore",
		})
		return
	}

	// Get chore details before deletion for real-time event
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	if err := h.choreRepo.DeleteChore(c, id); err != nil {
		logger.Error("Failed to delete chore", "error", err, "choreID", id, "userID", currentUser.ID)
		c.JSON(500, gin.H{
			"error": "Failed to delete chore",
		})
		return
	}
	h.nRepo.DeleteAllChoreNotifications(id)
	h.tRepo.DissociateChoreWithThing(c, id)

	// Broadcast real-time chore deletion event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		broadcaster.BroadcastChoreDeleted(chore.ID, chore.Name, chore.CircleID, &currentUser.User)
	}

	c.JSON(200, gin.H{
		"message": "Chore deleted successfully",
	})
}

// func (h *Handler) createChore(c *gin.Context) {
// 	logger := logging.FromContext(c)
// 	currentUser, ok := auth.CurrentUser(c)

// 	logger.Debug("Create chore", "currentUser", currentUser)
// 	if !ok {
// 		c.JSON(500, gin.H{
// 			"error": "Error getting current user",
// 		})
// 		return
// 	}
// 	id, err := h.choreRepo.CreateChore(currentUser.ID, currentUser.CircleID)
// 	if err != nil {
// 		c.JSON(500, gin.H{
// 			"error": "Error creating chore",
// 		})
// 		return
// 	}

// 	c.JSON(200, gin.H{
// 		"res": id,
// 	})
// }

// UpdateAssignee godoc
//
//	@Summary		Update chore assignee
//	@Description	Updates the currently assigned user for a chore
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int										true	"Chore ID"
//	@Param			assignee	body		object{assignee=int,updatedAt=string}	true	"Assignee update request"
//	@Success		200			{object}	map[string]model.Chore					"res: updated chore"
//	@Failure		400			{object}	map[string]string						"error: Invalid ID | Invalid request | Assignee not found in assignees"
//	@Failure		401			{object}	map[string]string						"error: Authentication failed"
//	@Failure		403			{object}	map[string]string						"error: You cannot edit this chore"
//	@Failure		500			{object}	map[string]string						"error: Failed to retrieve chore | Error updating assignee"
//	@Router			/chores/{id}/assignee [put]
func (h *Handler) updateAssignee(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	type AssigneeReq struct {
		Assignee  int       `json:"assignee" binding:"required"`
		UpdatedAt time.Time `json:"updatedAt" binding:"required"`
	}

	var assigneeReq AssigneeReq
	if err := c.ShouldBindJSON(&assigneeReq); err != nil {
		logging.FromContext(c).Error("Operation failed", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	// confirm that the assignee is one of the assignees:
	assigneeFound := false
	for _, assignee := range chore.Assignees {

		if assignee.UserID == assigneeReq.Assignee {
			assigneeFound = true
			break
		}
	}
	if !assigneeFound {
		c.JSON(400, gin.H{
			"error": "Assignee not found in assignees",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, &assigneeReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{
			"error": fmt.Sprintf("You cannot edit this chore: %s", err.Error()),
		})
		return
	}

	if err := h.choreRepo.UpdateChoreFields(c, id, map[string]interface{}{
		"assigned_to": assigneeReq.Assignee,
		"updated_by":  currentUser.ID,
		"updated_at":  assigneeReq.UpdatedAt,
	}); err != nil {
		logging.FromContext(c).Error("Error updating assignee", "error", err, "choreID", id, "assignee", assigneeReq.Assignee)

		c.JSON(500, gin.H{
			"error": "Error updating assignee",
		})
		return
	}

	// Broadcast real-time assignee update event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"assignedTo": assigneeReq.Assignee,
				"updatedBy":  currentUser.ID,
				"updatedAt":  assigneeReq.UpdatedAt,
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
}

// StartChore godoc
//
//	@Summary		Start a chore timer
//	@Description	Starts or resumes a chore timer session (supports impersonation)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]interface{}	"res: {timerUpdatedAt, status, duration}"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID | Chore is not in a state that can be started"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		403	{object}	map[string]string		"error: You are not allowed to start this chore"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore | Error creating time session"
//	@Router			/chores/{id}/start [put]
func (h *Handler) startChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	logger := logging.FromContext(c)

	// Get actual user and impersonated user (if any)
	actualUser, impersonatedUser, hasImpersonation := auth.CurrentUserWithImpersonation(c)
	if actualUser == nil {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// Use impersonated user for operations
	var effectiveUser *uModel.UserDetails
	if hasImpersonation {
		effectiveUser = impersonatedUser
		logger.Info("Starting chore with impersonation",
			"actualUserID", actualUser.ID,
			"impersonatedUserID", impersonatedUser.ID,
			"choreID", id)
	} else {
		effectiveUser = actualUser
	}

	chore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, actualUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(effectiveUser.ID, circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to start this chore",
		})
		return
	}
	var session *chModel.TimeSession
	switch chore.Status {
	case chModel.ChoreStatusNoStatus:
		session, err = h.choreRepo.CreateTimeSession(c, chore, effectiveUser.ID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error creating time session",
			})
			return
		}
		h.choreRepo.UpdateChoreStatus(c, chore.ID, chModel.ChoreStatusInProgress)
	case chModel.ChoreStatusPaused:
		session, err = h.choreRepo.GetActiveTimeSession(c, chore.ID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting active time session",
			})
			return
		}
		if session != nil {
			session.Start(effectiveUser.ID)
			if err := h.choreRepo.UpdateTimeSession(c, session); err != nil {
				c.JSON(500, gin.H{
					"error": "Error updating time session",
				})
				return
			}
		}
		h.choreRepo.UpdateChoreStatus(c, chore.ID, chModel.ChoreStatusInProgress)

	default:
		c.JSON(400, gin.H{
			"error": "Chore is not in a state that can be started",
		})
		return
	}
	if h.realTimeService != nil {
		chore.Status = chModel.ChoreStatusInProgress
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Build changes map (simplified - in real implementation you might want to track actual changes)
		changes := map[string]interface{}{
			"updatedBy":      actualUser.ID, // Use actual user for audit trail
			"updatedAt":      time.Now().UTC(),
			"status":         chModel.ChoreStatusInProgress,
			"timerUpdatedAt": session.UpdateAt,
		}
		broadcaster.BroadcastChoreUpdated(chore, &effectiveUser.User, changes, nil)
	}

	if session != nil {
		c.JSON(200, gin.H{
			"res": map[string]interface{}{
				"timerUpdatedAt": session.UpdateAt,
				"status":         chModel.ChoreStatusInProgress,
				"duration":       session.Duration,
			},
		})
	}
}

// PauseChore godoc
//
//	@Summary		Pause a chore timer
//	@Description	Pauses an active chore timer session (supports impersonation)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]interface{}	"res: {duration, status, timerUpdatedAt}"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID | No active time session found for this chore"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		403	{object}	map[string]string		"error: You are not allowed to pause this chore"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore | Error getting active time session"
//	@Router			/chores/{id}/pause [put]
func (h *Handler) pauseChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	logger := logging.FromContext(c)

	// Get actual user and impersonated user (if any)
	actualUser, impersonatedUser, hasImpersonation := auth.CurrentUserWithImpersonation(c)
	if actualUser == nil {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// Use impersonated user for operations
	var effectiveUser *uModel.UserDetails
	if hasImpersonation {
		effectiveUser = impersonatedUser
		logger.Info("Pausing chore with impersonation",
			"actualUserID", actualUser.ID,
			"impersonatedUserID", impersonatedUser.ID,
			"choreID", id)
	} else {
		effectiveUser = actualUser
	}

	chore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, actualUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(effectiveUser.ID, circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to pause this chore",
		})
		return
	}

	session, err := h.choreRepo.GetActiveTimeSession(c, chore.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting active time session",
		})
		return
	}
	if session == nil {
		c.JSON(400, gin.H{
			"error": "No active time session found for this chore",
		})
		return
	}
	session.Pause(effectiveUser.ID)
	if err := h.choreRepo.UpdateTimeSession(c, session); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating time session",
		})
		return
	}
	h.choreRepo.UpdateChoreStatus(c, chore.ID, chModel.ChoreStatusPaused)
	if h.realTimeService != nil {
		chore.Status = chModel.ChoreStatusPaused
		broadcaster := h.realTimeService.GetEventBroadcaster()

		broadcaster.BroadcastChoreStatus(chore, &effectiveUser.User,
			map[string]interface{}{
				"updatedBy":      actualUser.ID, // Use actual user for audit trail
				"updatedAt":      time.Now().UTC(),
				"status":         chore.Status,
				"timerUpdatedAt": session.UpdateAt,
			})
	}

	c.JSON(200, gin.H{
		"res": map[string]interface{}{
			"duration":       session.Duration,
			"status":         chModel.ChoreStatusPaused,
			"timerUpdatedAt": session.UpdateAt,
		},
	})

}

// ResetChoreTimer godoc
//
//	@Summary		Reset chore timer
//	@Description	Resets the chore timer to zero and starts a new session
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]interface{}	"res: {timerUpdatedAt, status, duration}"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID | No active time session found for this chore"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		403	{object}	map[string]string		"error: You are not allowed to reset timer for this chore"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore | Error updating time session"
//	@Router			/chores/{id}/timer/reset [put]
func (h *Handler) ResetChoreTimer(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(currentUser.ID, circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to reset timer for this chore",
		})
		return
	}

	session, err := h.choreRepo.GetActiveTimeSession(c, chore.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting active time session",
		})
		return
	}

	if session == nil {
		c.JSON(400, gin.H{
			"error": "No active time session found for this chore",
		})
		return
	}

	// Reset the timer: clear pause log, reset duration, set start time to now
	timeNow := time.Now().UTC()
	session.PauseLog = chModel.PauseLogEntries{}
	session.Duration = 0
	session.StartTime = timeNow
	session.Status = chModel.TimeSessionStatusActive
	session.UpdateBy = currentUser.ID
	session.UpdateAt = timeNow

	// Add new pause log entry for the reset session
	session.PauseLog = append(session.PauseLog, &chModel.PauseLogEntry{
		StartTime: timeNow,
		UpdateBy:  currentUser.ID,
	})

	if err := h.choreRepo.UpdateTimeSession(c, session); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating time session",
		})
		return
	}

	// Update chore status to in progress
	h.choreRepo.UpdateChoreStatus(c, chore.ID, chModel.ChoreStatusInProgress)

	// Broadcast the change via real-time service
	if h.realTimeService != nil {
		chore.Status = chModel.ChoreStatusInProgress
		broadcaster := h.realTimeService.GetEventBroadcaster()

		changes := map[string]interface{}{
			"updatedBy":      currentUser.ID,
			"updatedAt":      timeNow,
			"status":         chModel.ChoreStatusInProgress,
			"timerUpdatedAt": session.UpdateAt,
		}
		broadcaster.BroadcastChoreUpdated(chore, &currentUser.User, changes, nil)
	}

	c.JSON(200, gin.H{
		"res": map[string]interface{}{
			"timerUpdatedAt": session.UpdateAt,
			"status":         chModel.ChoreStatusInProgress,
			"duration":       session.Duration,
		},
	})
}

// SkipChore godoc
//
//	@Summary		Skip a chore
//	@Description	Skips the current chore iteration and schedules the next due date (supports impersonation)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]model.Chore	"res: updated chore"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore | Error scheduling next due date | Error completing chore"
//	@Router			/chores/{id}/skip [post]
func (h *Handler) skipChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	logger := logging.FromContext(c)

	// Get actual user and impersonated user (if any)
	actualUser, impersonatedUser, hasImpersonation := auth.CurrentUserWithImpersonation(c)
	if actualUser == nil {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// Use impersonated user for operations
	var effectiveUser *uModel.UserDetails
	if hasImpersonation {
		effectiveUser = impersonatedUser
		logger.Info("Skipping chore with impersonation",
			"actualUserID", actualUser.ID,
			"impersonatedUserID", impersonatedUser.ID,
			"choreID", id)
	} else {
		effectiveUser = actualUser
	}

	chore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	nextDueDate, err := scheduleNextDueDate(c, chore, chore.NextDueDate.UTC())
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error scheduling next due date",
		})
		return
	}

	nextAssignedTo := chore.AssignedTo
	if err := h.choreRepo.SkipChore(c, chore, effectiveUser.ID, nextDueDate, nextAssignedTo); err != nil {
		c.JSON(500, gin.H{
			"error": "Error completing chore",
		})
		return
	}

	updatedChore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	h.eventProducer.ChoreSkipped(c, effectiveUser.WebhookURL, updatedChore, &effectiveUser.User)

	// Broadcast real-time chore skip event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Get the skip history entry
		history, _ := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 1)
		var choreHistory *chModel.ChoreHistory
		if len(history) > 0 {
			choreHistory = history[0]
		}
		broadcaster.BroadcastChoreSkipped(updatedChore, &effectiveUser.User, choreHistory, nil)
	}

	c.JSON(200, gin.H{
		"res": updatedChore,
	})
}

// UpdateDueDate godoc
//
//	@Summary		Update chore due date
//	@Description	Updates the next due date for a chore (can be set to null)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int										true	"Chore ID"
//	@Param			dueDate	body		object{dueDate=string,updatedAt=string}	true	"Due date update request"
//	@Success		200		{object}	map[string]model.Chore					"res: updated chore"
//	@Failure		400		{object}	map[string]string						"error: Invalid ID | Invalid request | Invalid date"
//	@Failure		401		{object}	map[string]string						"error: Authentication failed"
//	@Failure		403		{object}	map[string]string						"error: You cannot edit this chore"
//	@Failure		500		{object}	map[string]string						"error: Failed to retrieve chore | Error updating due date"
//	@Router			/chores/{id}/dueDate [put]
func (h *Handler) updateDueDate(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	type DueDateReq struct {
		DueDate   *string   `json:"dueDate"`
		UpdatedAt time.Time `json:"updatedAt" binding:"required"`
	}

	var dueDateReq DueDateReq
	if err := c.ShouldBindJSON(&dueDateReq); err != nil {
		logging.FromContext(c).Error("Operation failed", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	var dueDate *time.Time

	// Handle due date: if nil, empty string, or "null", set to nil
	if dueDateReq.DueDate != nil && *dueDateReq.DueDate != "" && *dueDateReq.DueDate != "null" {
		rawDueDate, err := time.Parse(time.RFC3339, *dueDateReq.DueDate)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid date",
			})
			return
		}
		utcDueDate := rawDueDate.UTC()
		dueDate = &utcDueDate
	}
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, &dueDateReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{})
		return
	}
	if err := h.choreRepo.UpdateChoreFields(c, chore.ID, map[string]interface{}{
		"next_due_date": dueDate,
		"updated_by":    currentUser.ID,
		"updated_at":    time.Now().UTC(),
	}); err != nil {
		logging.FromContext(c).Error("Failed to update due date", "error", err)
		c.JSON(500, gin.H{
			"error": "Error updating due date",
		})
		return
	}

	// Broadcast real-time due date update event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, chore.ID, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"nextDueDate": dueDate,
				"updatedBy":   currentUser.ID,
				"updatedAt":   time.Now().UTC(),
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
}

// ArchiveChore godoc
//
//	@Summary		Archive a chore
//	@Description	Archives a chore, hiding it from the main list
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Chore ID"
//	@Success		200	{object}	map[string]string	"message: Chore archived successfully"
//	@Failure		400	{object}	map[string]string	"error: Invalid ID"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		500	{object}	map[string]string	"error: Error archiving chore"
//	@Router			/chores/{id}/archive [put]
func (h *Handler) archiveChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	err = h.choreRepo.ArchiveChore(c, id, currentUser.ID)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error archiving chore",
		})
		return
	}

	// Broadcast real-time chore archive event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"archived":  true,
				"updatedBy": currentUser.ID,
				"updatedAt": time.Now().UTC(),
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"message": "Chore archived successfully",
	})
}

// UnarchiveChore godoc
//
//	@Summary		Unarchive a chore
//	@Description	Restores an archived chore to the main list
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int					true	"Chore ID"
//	@Success		200	{object}	map[string]string	"message: Chore unarchived successfully"
//	@Failure		400	{object}	map[string]string	"error: Invalid ID"
//	@Failure		401	{object}	map[string]string	"error: Authentication failed"
//	@Failure		500	{object}	map[string]string	"error: Error unarchiving chore"
//	@Router			/chores/{id}/unarchive [put]
func (h *Handler) UnarchiveChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	err = h.choreRepo.UnarchiveChore(c, id, currentUser.ID)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error unarchiving chore",
		})
		return
	}

	// Broadcast real-time chore unarchive event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"archived":  false,
				"updatedBy": currentUser.ID,
				"updatedAt": time.Now().UTC(),
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"message": "Chore unarchived successfully",
	})
}

// CompleteChore godoc
//
//	@Summary		Complete a chore
//	@Description	Marks a chore as completed and schedules the next occurrence (supports impersonation)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id				path		int									true	"Chore ID"
//	@Param			completedDate	query		string								false	"Completion date in RFC3339 format (defaults to now)"
//	@Param			completion		body		object{note=string,completedBy=int}	false	"Completion details"
//	@Success		200				{object}	map[string]model.Chore				"res: updated chore"
//	@Failure		400				{object}	map[string]string					"error: Invalid ID | Invalid date | User is not assigned to chore | Chore is out of completion window"
//	@Failure		401				{object}	map[string]string					"error: Authentication failed"
//	@Failure		403				{object}	map[string]string					"error: You are not allowed to complete this action"
//	@Failure		500				{object}	map[string]string					"error: Failed to retrieve chore | Error completing chore"
//	@Router			/chores/{id}/do [post]
func (h *Handler) completeChore(c *gin.Context) {
	type CompleteChoreReq struct {
		Note string `json:"note"`
		// the completed by only can be populated by the admin or super user
		CompletedBy *int `json:"completedBy"`
	}
	var req CompleteChoreReq
	logger := logging.FromContext(c)

	actualUser, impersonatedUser, hasImpersonation := auth.CurrentUserWithImpersonation(c)
	if actualUser == nil {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// Use impersonated user for operations, actual user for audit
	var effectiveUser *uModel.UserDetails
	if hasImpersonation {
		effectiveUser = impersonatedUser
		logger.Info("Completing chore with impersonation",
			"actualUserID", actualUser.ID,
			"impersonatedUserID", impersonatedUser.ID,
			"choreID", c.Param("id"))
	} else {
		effectiveUser = actualUser
	}

	completedBy := effectiveUser.ID
	completeChoreID := c.Param("id")
	var completedDate time.Time
	rawCompletedDate := c.Query("completedDate")
	if rawCompletedDate == "" {
		completedDate = time.Now().UTC()
	} else {
		var err error
		completedDate, err = time.Parse(time.RFC3339, rawCompletedDate)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "Invalid date",
			})
			return
		}
	}

	var additionalNotes *string
	_ = c.ShouldBind(&req)

	if req.Note != "" {
		additionalNotes = &req.Note
	}

	id, err := strconv.Atoi(completeChoreID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	chore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	circleUsers, err := h.circleRepo.GetCircleUsers(c, actualUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(effectiveUser.ID, circleUsers) {
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}

	// confirm that the chore in completion window:
	if chore.CompletionWindow != nil {
		if completedDate.UTC().Before(chore.NextDueDate.UTC().Add(-time.Hour * time.Duration(*chore.CompletionWindow))) {
			c.JSON(400, gin.H{
				"error": "Chore is out of completion window",
			})
			return
		}
	}

	if req.CompletedBy != nil {
		// Only allow admins to complete chores on behalf of others in the circle
		// Use actualUser for authorization since this is an admin function
		ok := authorizeChoreCompletionForUser(h, c, actualUser, req.CompletedBy)
		if !ok {
			return
		}
		completedBy = *req.CompletedBy
	}
	var nextDueDate *time.Time
	if chore.FrequencyType == "adaptive" {
		history, err := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 5)
		if err != nil {
			logging.FromContext(c).Errorw("Failed to fetch chore history for adaptive scheduling", "error", err, "choreID", chore.ID)
			c.JSON(500, gin.H{
				"error": "Failed to fetch chore history for adaptive scheduling",
			})
			return
		}
		nextDueDate, err = scheduleAdaptiveNextDueDate(chore, completedDate, history)
		if err != nil {
			logging.FromContext(c).Error("Failed to schedule next due date", "error", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}

	} else {
		nextDueDate, err = scheduleNextDueDate(c, chore, completedDate.UTC())
		if err != nil {
			logging.FromContext(c).Error("Failed to schedule next due date", "error", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}
	}
	choreHistory, err := h.choreRepo.GetChoreHistory(c, chore.ID)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch chore history for assignee calculation", "error", err, "choreID", chore.ID)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore history for assignee calculation",
		})
		return
	}

	// Check if chore requires approval
	if chore.RequireApproval {
		// Set chore status to pending approval instead of completing
		if err := h.choreRepo.SetChorePendingApproval(c, chore, additionalNotes, completedBy, &completedDate); err != nil {
			c.JSON(500, gin.H{
				"error": "Error setting chore pending approval",
			})
			return
		}

		updatedChore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting chore",
			})
			return
		}

		// Broadcast pending approval event
		if h.realTimeService != nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"status":    chModel.ChoreStatusPendingApproval,
				"updatedBy": actualUser.ID, // Use actual user for audit trail
				"updatedAt": time.Now().UTC(),
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &effectiveUser.User, changes, additionalNotes)
		}

		c.JSON(200, gin.H{
			"res":     updatedChore,
			"message": "Chore completion submitted for approval",
		})
		return
	}

	nextAssignedTo, err := checkNextAssignee(chore, choreHistory, completedBy)
	if err != nil {
		logging.FromContext(c).Error("Failed to check next assignee", "error", err)
		c.JSON(500, gin.H{
			"error": "Error checking next assignee",
		})
		return
	}

	if err := h.choreRepo.CompleteChore(c, chore, additionalNotes, completedBy, nextDueDate, &completedDate, nextAssignedTo, true); err != nil {
		c.JSON(500, gin.H{
			"error": "Error completing chore",
		})
		return
	}
	updatedChore, err := h.choreRepo.GetChore(c, id, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	if updatedChore.SubTasks != nil && updatedChore.FrequencyType != chModel.FrequencyTypeOnce {
		h.stRepo.ResetSubtasksCompletion(c, updatedChore.ID)
	}

	// go func() {

	// 	h.notifier.SendChoreCompletion(c, chore, effectiveUser)
	// }()
	h.nPlanner.GenerateNotifications(c, updatedChore)
	h.eventProducer.ChoreCompleted(c, effectiveUser.WebhookURL, chore, &effectiveUser.User)
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Get the completion history entry
		history, _ := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 1)

		var choreHistory *chModel.ChoreHistory
		if len(history) > 0 {
			choreHistory = history[0]
		}
		broadcaster.BroadcastChoreCompleted(updatedChore, &effectiveUser.User, choreHistory, additionalNotes)
	}

	c.JSON(200, gin.H{
		"res": updatedChore,
	})
}

func authorizeChoreCompletionForUser(h *Handler, c *gin.Context, currentUser *uModel.UserDetails, completedByUserID *int) bool {
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting circle users",
		})
		return false
	}

	isAuthorized := false
	isCompletedByAuthorized := false

	for _, circleUser := range circleUsers {
		if circleUser.UserID == currentUser.ID && (circleUser.Role == circle.UserRoleAdmin || circleUser.Role == circle.UserRoleManager) {
			isAuthorized = true
		}
		if circleUser.UserID == *completedByUserID {
			isCompletedByAuthorized = true
		}
	}
	if !isAuthorized || !isCompletedByAuthorized {

		c.JSON(403, gin.H{
			"error": "You are not allowed to complete this action, either you are not admin or the completed by user is not in the circle",
		})
		return false
	}

	return true
}

// GetChoreHistory godoc
//
//	@Summary		Get chore history
//	@Description	Retrieves the completion history for a specific chore
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int								true	"Chore ID"
//	@Success		200	{object}	map[string][]model.ChoreHistory	"res: array of chore history entries"
//	@Failure		400	{object}	map[string]string				"error: Invalid ID"
//	@Failure		500	{object}	map[string]string				"error: Failed to fetch chore history"
//	@Router			/chores/{id}/history [get]
func (h *Handler) GetChoreHistory(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	choreHistory, err := h.choreRepo.GetChoreHistory(c, id)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch chore history", "error", err, "choreID", id)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore history",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": choreHistory,
	})
}

// GetChoreDetail godoc
//
//	@Summary		Get chore details
//	@Description	Retrieves comprehensive details about a chore including metadata
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]interface{}	"res: detailed chore information"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		500	{object}	map[string]string		"error: Failed to fetch chore details"
//	@Router			/chores/{id}/details [get]
func (h *Handler) GetChoreDetail(c *gin.Context) {

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	detailed, err := h.choreRepo.GetChoreDetailByID(c, id, currentUser.CircleID, currentUser.ID)
	if err != nil {
		logger.Errorw("Failed to fetch chore details", "error", err, "choreID", id, "userID", currentUser.ID)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore details",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": detailed,
	})
}

// ModifyHistory godoc
//
//	@Summary		Modify chore history entry
//	@Description	Updates a specific chore history entry (performed date, due date, notes)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int														true	"Chore ID"
//	@Param			history_id	path		int														true	"History ID"
//	@Param			history		body		object{performedAt=string,dueDate=string,notes=string}	true	"History modification request"
//	@Success		200			{object}	map[string]model.ChoreHistory							"res: updated history entry"
//	@Failure		400			{object}	map[string]string										"error: Invalid Chore ID | Invalid History ID | Invalid request"
//	@Failure		401			{object}	map[string]string										"error: Authentication failed"
//	@Failure		403			{object}	map[string]string										"error: You are not allowed to modify this history"
//	@Failure		500			{object}	map[string]string										"error: Failed to fetch chore history entry | Error updating history"
//	@Router			/chores/{id}/history/{history_id} [put]
func (h *Handler) ModifyHistory(c *gin.Context) {

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	choreID, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid Chore ID",
		})
		return
	}
	type ModifyHistoryReq struct {
		PerformedAt *time.Time `json:"performedAt"`
		DueDate     *time.Time `json:"dueDate"`
		Notes       *string    `json:"notes"`
	}

	var req ModifyHistoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.FromContext(c).Error("Operation failed", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	rawHistoryID := c.Param("history_id")
	historyID, err := strconv.Atoi(rawHistoryID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid History ID",
		})
		return
	}

	history, err := h.choreRepo.GetChoreHistoryByID(c, choreID, historyID)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch chore history entry", "error", err, "choreID", choreID, "historyID", historyID)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore history entry",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}

	if currentUser.ID != history.CompletedBy && (history.AssignedTo == nil || currentUser.ID != *history.AssignedTo) && !currentUser.IsAdminOrManager(circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to modify this history",
		})
		return
	}
	if req.PerformedAt != nil {
		history.PerformedAt = req.PerformedAt
	}
	if req.DueDate != nil {
		history.DueDate = req.DueDate
	}
	if req.Notes != nil {
		history.Note = req.Notes
	}

	if err := h.choreRepo.UpdateChoreHistory(c, history); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating history",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": history,
	})
}

// UpdatePriority godoc
//
//	@Summary		Update chore priority
//	@Description	Updates the priority level of a chore (0-4)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int						true	"Chore ID"
//	@Param			priority	body		object{priority=int}	true	"Priority value (0-4)"
//	@Success		200			{object}	map[string]string		"message: Priority updated successfully"
//	@Failure		400			{object}	map[string]string		"error: Invalid ID | Invalid request"
//	@Failure		401			{object}	map[string]string		"error: Authentication failed"
//	@Failure		403			{object}	map[string]string		"error: You are not allowed to edit this chore"
//	@Failure		500			{object}	map[string]string		"error: Failed to retrieve chore | Error updating priority"
//	@Router			/chores/{id}/priority [put]
func (h *Handler) updatePriority(c *gin.Context) {
	type PriorityReq struct {
		Priority *int `json:"priority" binding:"required,gt=-1,lt=5"`
	}

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	var priorityReq PriorityReq
	if err := c.ShouldBindJSON(&priorityReq); err != nil {
		logging.FromContext(c).Error("Operation failed", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	// config user can edit:
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, nil); err != nil {
		logger.Error("User not allowed to edit chore", "userID", currentUser.ID, "choreID", chore.ID)
		c.JSON(403, gin.H{
			"error": "You are not allowed to edit this chore",
		})
		return
	}

	if err := h.choreRepo.UpdateChorePriority(c, currentUser.ID, id, *priorityReq.Priority); err != nil {
		logger.Error("Failed to update priority", "error", err)
		c.JSON(500, gin.H{
			"error": "Error updating priority",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "Priority updated successfully",
	})
}

// GetChoresHistory godoc
//
//	@Summary		Get chores history for user
//	@Description	Retrieves completion history for all chores for the current user
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			limit	query		int								false	"Number of days to retrieve history (default: 7)"
//	@Param			members	query		boolean							false	"Include other circle members' history"
//	@Success		200		{object}	map[string][]model.ChoreHistory	"res: array of chore history entries"
//	@Failure		400		{object}	map[string]string				"error: Invalid duration"
//	@Failure		401		{object}	map[string]string				"error: Authentication failed"
//	@Failure		500		{object}	map[string]string				"error: Failed to fetch user's chore history"
//	@Router			/chores/history [get]
func (h *Handler) getChoresHistory(c *gin.Context) {

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}
	durationRaw := c.Query("limit")
	if durationRaw == "" {
		durationRaw = "7"
	}

	duration, err := strconv.Atoi(durationRaw)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid duration",
		})
		return
	}
	includeCircleRaw := c.Query("members")
	includeCircle := false
	if includeCircleRaw == "true" {
		includeCircle = true
	}

	choreHistories, err := h.choreRepo.GetChoresHistoryByUserID(c, currentUser.ID, currentUser.CircleID, duration, includeCircle)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch user's chore history", "error", err, "userID", currentUser.ID, "duration", duration)
		c.JSON(500, gin.H{
			"error": "Failed to fetch user's chore history",
		})
		return
	}
	c.JSON(200, gin.H{
		"res": choreHistories,
	})
}

// DeleteHistory godoc
//
//	@Summary		Delete chore history entry
//	@Description	Deletes a specific chore history entry (creator only)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int					true	"Chore ID"
//	@Param			history_id	path		int					true	"History ID"
//	@Success		200			{object}	map[string]string	"message: History deleted successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid Chore ID | Invalid History ID"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		403			{object}	map[string]string	"error: You are not allowed to delete this history"
//	@Failure		500			{object}	map[string]string	"error: Failed to fetch chore history entry | Error deleting history"
//	@Router			/chores/{id}/history/{history_id} [delete]
func (h *Handler) DeleteHistory(c *gin.Context) {

	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	choreID, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid Chore ID",
		})
		return
	}

	rawHistoryID := c.Param("history_id")
	historyID, err := strconv.Atoi(rawHistoryID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid History ID",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err, "circleID", currentUser.CircleID, "userID", currentUser.ID)
		c.JSON(500, gin.H{"error": "Failed to retrieve circle users"})
		return
	}

	history, err := h.choreRepo.GetChoreHistoryByID(c, choreID, historyID)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch chore history entry", "error", err, "choreID", choreID, "historyID", historyID)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore history entry",
		})
		return
	}

	if !chore.CanDeleteHistory(currentUser.ID, circleUsers, history) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to delete this history",
		})
		return
	}

	if err := h.choreRepo.DeleteChoreHistory(c, historyID); err != nil {
		c.JSON(500, gin.H{
			"error": "Error deleting history",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "History deleted successfully",
	})
}

// UpdateSubtaskCompletedAt godoc
//
//	@Summary		Update subtask completion status
//	@Description	Marks a subtask as completed or uncompleted (supports impersonation)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int												true	"Chore ID"
//	@Param			subtask	body		object{id=int,choreId=int,completedAt=string}	true	"Subtask completion request"
//	@Success		200		{object}	map[string]interface{}							"Empty success response"
//	@Failure		400		{object}	map[string]string								"error: Invalid Chore ID | Invalid request | User is not assigned to chore"
//	@Failure		401		{object}	map[string]string								"error: Authentication failed"
//	@Failure		500		{object}	map[string]string								"error: Failed to retrieve chore | Error getting subtask"
//	@Router			/chores/{id}/subtask [put]
func (h *Handler) UpdateSubtaskCompletedAt(c *gin.Context) {
	logger := logging.FromContext(c)

	// Get actual user and impersonated user (if any)
	actualUser, impersonatedUser, hasImpersonation := auth.CurrentUserWithImpersonation(c)
	if actualUser == nil {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	// Use impersonated user for operations
	var effectiveUser *uModel.UserDetails
	if hasImpersonation {
		effectiveUser = impersonatedUser
		logger.Info("Updating subtask with impersonation",
			"actualUserID", actualUser.ID,
			"impersonatedUserID", impersonatedUser.ID,
			"choreID", c.Param("id"))
	} else {
		effectiveUser = actualUser
	}

	rawID := c.Param("id")
	choreID, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid Chore ID",
		})
		return
	}

	type SubtaskReq struct {
		ID          int        `json:"id"`
		ChoreID     int        `json:"choreId"`
		CompletedAt *time.Time `json:"completedAt"`
	}

	var req SubtaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.FromContext(c).Error("Operation failed", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	chore, err := h.choreRepo.GetChore(c, choreID, effectiveUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, actualUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanComplete(effectiveUser.ID, circleUsers) {
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}
	var completedAt *time.Time
	if req.CompletedAt != nil {
		completedAt = req.CompletedAt
	}
	err = h.stRepo.UpdateSubTaskStatus(c, effectiveUser.ID, req.ID, completedAt)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting subtask",
		})
		return
	}

	// Broadcast real-time subtask event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()

		broadcaster.BroadcastSubtaskUpdated(
			choreID,
			req.ID,
			completedAt,
			&effectiveUser.User,
			chore.CircleID,
		)

	}

	h.eventProducer.SubtaskUpdated(c, effectiveUser.WebhookURL,
		&stModel.SubTask{
			ID:          req.ID,
			ChoreID:     req.ChoreID,
			CompletedAt: completedAt,
			CompletedBy: effectiveUser.ID,
		},
	)
	c.JSON(200, gin.H{})

}

// GetChoreTimeSessions godoc
//
//	@Summary		Get chore time sessions
//	@Description	Retrieves all time tracking sessions for a chore
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id				path		int								true	"Chore ID"
//	@Param			choreHistoryId	query		int								false	"Filter by specific history ID"
//	@Success		200				{object}	map[string][]model.TimeSession	"res: array of time sessions"
//	@Failure		400				{object}	map[string]string				"error: Invalid ID | Invalid choreHistoryId parameter"
//	@Failure		401				{object}	map[string]string				"error: Authentication failed"
//	@Failure		403				{object}	map[string]string				"error: You are not allowed to view time sessions for this chore"
//	@Failure		404				{object}	map[string]string				"error: No time session found"
//	@Failure		500				{object}	map[string]string				"error: Failed to retrieve chore | Error getting chore time session"
//	@Router			/chores/{id}/timer [get]
func (h *Handler) GetChoreTimeSessions(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	// First, get the chore to check authorization
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if !chore.CanView(currentUser.ID, circleUsers) {
		c.JSON(403, gin.H{
			"error": "You are not allowed to view time sessions for this chore",
		})
		return
	}

	// Check for optional choreHistoryId query parameter
	var choreHistoryId *int
	if historyIdStr := c.Query("choreHistoryId"); historyIdStr != "" {
		if historyIdInt, err := strconv.Atoi(historyIdStr); err == nil {
			choreHistoryId = &historyIdInt
		} else {
			c.JSON(400, gin.H{
				"error": "Invalid choreHistoryId parameter",
			})
			return
		}
	}

	session, err := h.choreRepo.GetTimeSessionsByChoreID(c, id, choreHistoryId)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore time session",
		})
		return
	}

	if session == nil {
		c.JSON(404, gin.H{
			"error": "No time session found",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": session,
	})
}

// UpdateTimeSession godoc
//
//	@Summary		Update a time session
//	@Description	Updates start time, end time, or pause log for a time session
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int														true	"Chore ID"
//	@Param			session_id	path		int														true	"Session ID"
//	@Param			session		body		object{startTime=string,endTime=string,pauseLog=array}	true	"Time session update request"
//	@Success		200			{object}	map[string]model.TimeSession							"res: updated time session"
//	@Failure		400			{object}	map[string]string										"error: Invalid chore ID | Invalid session ID | Invalid request body | Time session does not belong to the specified chore"
//	@Failure		401			{object}	map[string]string										"error: Authentication failed"
//	@Failure		403			{object}	map[string]string										"error: You are not allowed to modify time sessions for this chore"
//	@Failure		500			{object}	map[string]string										"error: Failed to retrieve chore | Error getting time session | Error updating time session"
//	@Router			/chores/{id}/timer/{session_id} [put]
func (h *Handler) UpdateTimeSession(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawChoreID := c.Param("id")
	choreID, err := strconv.Atoi(rawChoreID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid chore ID",
		})
		return
	}

	rawSessionID := c.Param("session_id")
	sessionID, err := strconv.Atoi(rawSessionID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid session ID",
		})
		return
	}

	type UpdateTimeSessionReq struct {
		StartTime *time.Time               `json:"startTime"`
		EndTime   *time.Time               `json:"endTime"`
		PauseLog  *chModel.PauseLogEntries `json:"pauseLog"`
	}

	var req UpdateTimeSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// First, get the chore to check authorization
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Check if user has permission to modify time sessions for this chore
	// User can modify if they are the creator OR an assignee
	isAssignee := false
	for _, assignee := range chore.Assignees {
		if assignee.UserID == currentUser.ID {
			isAssignee = true
			break
		}
	}

	if currentUser.ID != chore.CreatedBy && !isAssignee {
		c.JSON(403, gin.H{
			"error": "You are not allowed to modify time sessions for this chore",
		})
		return
	}

	// Get the time session to ensure it exists and belongs to the chore
	session, err := h.choreRepo.GetTimeSessionByID(c, sessionID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting time session",
		})
		return
	}

	// Verify the session belongs to the specified chore
	if session.ChoreID != choreID {
		c.JSON(400, gin.H{
			"error": "Time session does not belong to the specified chore",
		})
		return
	}

	// Update the session fields
	if req.StartTime != nil {
		session.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		session.EndTime = req.EndTime
	}
	if req.PauseLog != nil {
		session.PauseLog = *req.PauseLog
	}

	session.UpdateBy = currentUser.ID

	// Save the updated session (this will recalculate duration)
	if err := h.choreRepo.UpdateTimeSessionData(c, session); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating time session",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": session,
	})
}

// DeleteTimeSession godoc
//
//	@Summary		Delete a time session
//	@Description	Deletes a specific time tracking session
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int					true	"Chore ID"
//	@Param			session_id	path		int					true	"Session ID"
//	@Success		200			{object}	map[string]string	"message: Time session deleted successfully"
//	@Failure		400			{object}	map[string]string	"error: Invalid chore ID | Invalid session ID | Time session does not belong to the specified chore"
//	@Failure		401			{object}	map[string]string	"error: Authentication failed"
//	@Failure		403			{object}	map[string]string	"error: You are not allowed to delete time sessions for this chore"
//	@Failure		500			{object}	map[string]string	"error: Failed to retrieve chore | Error getting time session | Error deleting time session"
//	@Router			/chores/{id}/timer/{session_id} [delete]
func (h *Handler) DeleteTimeSession(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawChoreID := c.Param("id")
	choreID, err := strconv.Atoi(rawChoreID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid chore ID",
		})
		return
	}

	rawSessionID := c.Param("session_id")
	sessionID, err := strconv.Atoi(rawSessionID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid session ID",
		})
		return
	}

	// First, get the chore to check authorization
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Check if user has permission to delete time sessions for this chore
	// User can delete if they are the creator OR an assignee
	isAssignee := false
	for _, assignee := range chore.Assignees {
		if assignee.UserID == currentUser.ID {
			isAssignee = true
			break
		}
	}

	if currentUser.ID != chore.CreatedBy && !isAssignee {
		c.JSON(403, gin.H{
			"error": "You are not allowed to delete time sessions for this chore",
		})
		return
	}

	// Get the time session to ensure it exists and belongs to the chore
	session, err := h.choreRepo.GetTimeSessionByID(c, sessionID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting time session",
		})
		return
	}

	// Verify the session belongs to the specified chore
	if session.ChoreID != choreID {
		c.JSON(400, gin.H{
			"error": "Time session does not belong to the specified chore",
		})
		return
	}

	// Delete the time session
	if err := h.choreRepo.DeleteTimeSession(c, sessionID, choreID); err != nil {
		c.JSON(500, gin.H{
			"error": "Error deleting time session",
		})
		return
	}
	if chore.Status == chModel.ChoreStatusInProgress || chore.Status == chModel.ChoreStatusPaused {
		h.choreRepo.UpdateChoreStatus(c, choreID, chModel.ChoreStatusNoStatus)
		c.JSON(200, gin.H{
			"message": "Time session deleted successfully",
		})

		return
	}
}

// ApproveChore godoc
//
//	@Summary		Approve a chore completion
//	@Description	Approves a pending chore completion (admin/manager only)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id	path		int						true	"Chore ID"
//	@Success		200	{object}	map[string]model.Chore	"res: updated chore"
//	@Failure		400	{object}	map[string]string		"error: Invalid ID | Chore is not pending approval"
//	@Failure		401	{object}	map[string]string		"error: Authentication failed"
//	@Failure		403	{object}	map[string]string		"error: Only admins can approve chores"
//	@Failure		500	{object}	map[string]string		"error: Failed to retrieve chore | Error approving chore"
//	@Router			/chores/{id}/approve [post]
func (h *Handler) approveChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	// Get the chore
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Check if user is admin in the circle
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}

	isAdmin := false
	for _, user := range circleUsers {
		if user.UserID == currentUser.ID && (user.Role == circle.UserRoleAdmin || user.Role == circle.UserRoleManager) {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "Only admins can approve chores",
		})
		return
	}

	// Check if chore is pending approval
	if chore.Status != chModel.ChoreStatusPendingApproval {
		c.JSON(400, gin.H{
			"error": "Chore is not pending approval",
		})
		return
	}

	// Get the pending approval history to determine who completed it
	allHistory, err := h.choreRepo.GetChoreHistory(c, chore.ID)
	if err != nil {
		logging.FromContext(c).Errorw("Failed to fetch chore history for approval process", "error", err, "choreID", chore.ID)
		c.JSON(500, gin.H{
			"error": "Failed to fetch chore history for approval process",
		})
		return
	}

	// Find the most recent pending approval entry
	var pendingHistory *chModel.ChoreHistory
	for _, h := range allHistory {
		if h.Status == chModel.ChoreHistoryStatusPendingApproval {
			pendingHistory = h
			break
		}
	}

	if pendingHistory == nil {
		c.JSON(500, gin.H{
			"error": "No pending approval history found",
		})
		return
	}

	completedBy := pendingHistory.CompletedBy
	completedDate := *pendingHistory.PerformedAt

	// Calculate next due date and assignee like in normal completion
	var nextDueDate *time.Time
	if chore.FrequencyType == "adaptive" {
		allHistory, err := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 5)
		if err != nil {
			logging.FromContext(c).Errorw("Failed to fetch chore history for adaptive scheduling during approval", "error", err, "choreID", chore.ID)
			c.JSON(500, gin.H{
				"error": "Failed to fetch chore history for adaptive scheduling",
			})
			return
		}
		nextDueDate, err = scheduleAdaptiveNextDueDate(chore, completedDate, allHistory)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}
	} else {
		nextDueDate, err = scheduleNextDueDate(c, chore, completedDate.UTC())
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}
	}

	nextAssignedTo, err := checkNextAssignee(chore, allHistory, completedBy)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error checking next assignee",
		})
		return
	}

	// Approve the chore
	if err := h.choreRepo.ApproveChore(c, chore, currentUser.ID, nextDueDate, nextAssignedTo, true); err != nil {
		c.JSON(500, gin.H{
			"error": "Error approving chore",
		})
		return
	}

	updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	if updatedChore.SubTasks != nil && updatedChore.FrequencyType != chModel.FrequencyTypeOnce {
		h.stRepo.ResetSubtasksCompletion(c, updatedChore.ID)
	}

	h.nPlanner.GenerateNotifications(c, updatedChore)
	h.eventProducer.ChoreCompleted(c, currentUser.WebhookURL, chore, &currentUser.User)

	// Broadcast real-time chore approved event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		broadcaster.BroadcastChoreCompleted(updatedChore, &currentUser.User, pendingHistory, nil)
	}

	c.JSON(200, gin.H{
		"res":     updatedChore,
		"message": "Chore approved successfully",
	})
}

// RejectChore godoc
//
//	@Summary		Reject a chore completion
//	@Description	Rejects a pending chore completion and returns it to no status (admin/manager only)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id			path		int						true	"Chore ID"
//	@Param			rejection	body		object{note=string}		false	"Rejection details"
//	@Success		200			{object}	map[string]interface{}	"res: updated chore, message: Chore rejected successfully"
//	@Failure		400			{object}	map[string]string		"error: Invalid ID | Chore is not pending approval"
//	@Failure		401			{object}	map[string]string		"error: Authentication failed"
//	@Failure		403			{object}	map[string]string		"error: Only admins can reject chores"
//	@Failure		500			{object}	map[string]string		"error: Failed to retrieve chore | Error rejecting chore"
//	@Router			/chores/{id}/reject [post]
func (h *Handler) rejectChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	type RejectChoreReq struct {
		Note string `json:"note"`
	}
	var req RejectChoreReq
	_ = c.ShouldBind(&req)

	// Get the chore
	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Check if user is admin in the circle
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}

	isAdmin := false
	for _, user := range circleUsers {
		if user.UserID == currentUser.ID && (user.Role == circle.UserRoleAdmin || user.Role == circle.UserRoleManager) {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		c.JSON(403, gin.H{
			"error": "Only admins can reject chores",
		})
		return
	}

	// Check if chore is pending approval
	if chore.Status != chModel.ChoreStatusPendingApproval {
		c.JSON(400, gin.H{
			"error": "Chore is not pending approval",
		})
		return
	}

	var rejectionNote *string
	if req.Note != "" {
		rejectionNote = &req.Note
	}

	// Reject the chore
	if err := h.choreRepo.RejectChore(c, id, rejectionNote); err != nil {
		c.JSON(500, gin.H{
			"error": "Error rejecting chore",
		})
		return
	}

	updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Broadcast real-time chore rejected event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		changes := map[string]interface{}{
			"status":    chModel.ChoreStatusNoStatus,
			"updatedBy": currentUser.ID,
			"updatedAt": time.Now().UTC(),
		}
		broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, rejectionNote)
	}

	c.JSON(200, gin.H{
		"res":     updatedChore,
		"message": "Chore rejected successfully",
	})
}

// UpdateChoreStatus godoc
//
//	@Summary		Update chore status
//	@Description	Updates the status of a chore (e.g., no status, in progress, paused)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int										true	"Chore ID"
//	@Param			status	body		object{status=string,updatedAt=string}	true	"Status update request"
//	@Success		200		{object}	map[string]string						"message: Chore status updated successfully"
//	@Failure		400		{object}	map[string]string						"error: Invalid ID | Invalid request"
//	@Failure		401		{object}	map[string]string						"error: Authentication failed"
//	@Failure		403		{object}	map[string]string						"error: You cannot update the status of this chore"
//	@Failure		500		{object}	map[string]string						"error: Failed to retrieve chore | Error updating chore status"
//	@Router			/chores/{id}/status [put]
func (h *Handler) updateChoreStatus(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	type StatusUpdateReq struct {
		Status    chModel.Status `json:"status" binding:"required"`
		UpdatedAt time.Time      `json:"updatedAt" binding:"required"`
	}

	var statusReq StatusUpdateReq
	if err := c.ShouldBindJSON(&statusReq); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, &statusReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{
			"error": fmt.Sprintf("You cannot update the status of this chore: %s", err.Error()),
		})
		return
	}

	if err := h.choreRepo.UpdateChoreFields(c, id, map[string]interface{}{
		"status":     statusReq.Status,
		"updated_by": currentUser.ID,
		"updated_at": statusReq.UpdatedAt,
	}); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating chore status",
		})
		return
	}

	// Broadcast real-time chore status update event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"status":    statusReq.Status,
				"updatedBy": currentUser.ID,
				"updatedAt": statusReq.UpdatedAt,
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"message": "Chore status updated successfully",
	})
}

// UpdateTimer godoc
//
//	@Summary		Update chore timer duration
//	@Description	Manually updates the timer duration for a chore
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int										true	"Chore ID"
//	@Param			timer	body		object{duration=int,updatedAt=string}	true	"Timer update request"
//	@Success		200		{object}	map[string]string						"message: Chore timer updated successfully"
//	@Failure		400		{object}	map[string]string						"error: Invalid ID | Invalid request"
//	@Failure		401		{object}	map[string]string						"error: Authentication failed"
//	@Failure		403		{object}	map[string]string						"error: You cannot update the timer of this chore"
//	@Failure		500		{object}	map[string]string						"error: Failed to retrieve chore | Error updating chore timer"
//	@Router			/chores/{id}/timer [put]
func (h *Handler) updateTimer(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	type TimerUpdateReq struct {
		Duration  int       `json:"duration" binding:"required"`
		UpdatedAt time.Time `json:"updatedAt" binding:"required"`
	}

	var timerReq TimerUpdateReq
	if err := c.ShouldBindJSON(&timerReq); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		logger.Error("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, &timerReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{
			"error": fmt.Sprintf("You cannot update the timer of this chore: %s", err.Error()),
		})
		return
	}

	if err := h.choreRepo.UpdateChoreFields(c, id, map[string]interface{}{
		"timer":      timerReq.Duration,
		"updated_by": currentUser.ID,
		"updated_at": timerReq.UpdatedAt,
	}); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating chore timer",
		})
		return
	}

	// Broadcast real-time chore timer update event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, id, currentUser.ID)
		if err == nil {
			broadcaster := h.realTimeService.GetEventBroadcaster()
			changes := map[string]interface{}{
				"timer":     timerReq.Duration,
				"updatedBy": currentUser.ID,
				"updatedAt": timerReq.UpdatedAt,
			}
			broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
		}
	}

	c.JSON(200, gin.H{
		"message": "Chore timer updated successfully",
	})
}

func checkNextAssignee(chore *chModel.Chore, choresHistory []*chModel.ChoreHistory, performerID int) (*int, error) {
	// copy the history to avoid modifying the original:
	history := make([]*chModel.ChoreHistory, len(choresHistory))
	copy(history, choresHistory)

	assigneesMap := map[int]bool{}
	for _, assignee := range chore.Assignees {
		assigneesMap[assignee.UserID] = true
	}
	var nextAssignee *int
	if len(history) == 0 {
		// if there is no history, just assume the current operation as the first
		history = append(history, &chModel.ChoreHistory{
			AssignedTo: &performerID,
		})
	}

	switch chore.AssignStrategy {
	case chModel.AssignmentStrategyLeastAssigned:
		// find the assignee with the least number of chores
		assigneeChores := map[int]int{}
		for _, performer := range chore.Assignees {
			assigneeChores[performer.UserID] = 0
		}
		for _, history := range history {
			if history.AssignedTo != nil {
				if ok := assigneesMap[*history.AssignedTo]; ok {
					// calculate the number of chores assigned to each assignee
					assigneeChores[*history.AssignedTo]++
				}
			}
		}

		var minChores int64 = math.MaxInt64
		var bestAssignee int
		for assignee, numChores := range assigneeChores {
			// if this is the first assignee or if the number of
			// chores assigned to this assignee is less than the current minimum
			if int64(numChores) < minChores {
				minChores = int64(numChores)
				// set the next assignee to this assignee
				bestAssignee = assignee
			}
		}
		if len(assigneeChores) > 0 {
			nextAssignee = &bestAssignee
		}
	case chModel.AssignmentStrategyLeastCompleted:
		// find the assignee who has completed the least number of chores
		assigneeChores := map[int]int{}
		for _, performer := range chore.Assignees {
			assigneeChores[performer.UserID] = 0
		}
		for _, history := range history {
			// calculate the number of chores completed by each assignee
			assigneeChores[history.CompletedBy]++
		}

		// max Int value
		var minChores int64 = math.MaxInt64
		var bestAssignee int

		for assignee, numChores := range assigneeChores {
			// if this is the first assignee or if the number of
			// chores completed by this assignee is less than the current minimum
			if int64(numChores) < minChores {
				minChores = int64(numChores)
				// set the next assignee to this assignee
				bestAssignee = assignee
			}
		}
		if len(assigneeChores) > 0 {
			nextAssignee = &bestAssignee
		}
	case chModel.AssignmentStrategyRandom:
		if len(chore.Assignees) > 0 {
			assigneeID := chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
			nextAssignee = &assigneeID
		}
	case chModel.AssignmentStrategyNoAssignee:
		nextAssignee = nil
	case chModel.AssignmentStrategyKeepLastAssigned:
		// keep the last assignee
		nextAssignee = chore.AssignedTo
	case chModel.AssignmentStrategyRandomExceptLastAssigned:
		var lastAssigned *int = chore.AssignedTo
		AssigneesCopy := make([]chModel.ChoreAssignees, len(chore.Assignees))
		copy(AssigneesCopy, chore.Assignees)
		var removeLastAssigned []chModel.ChoreAssignees
		if lastAssigned != nil {
			removeLastAssigned = remove(AssigneesCopy, *lastAssigned)
		} else {
			removeLastAssigned = AssigneesCopy
		}
		if len(removeLastAssigned) > 0 {
			assigneeID := removeLastAssigned[rand.Intn(len(removeLastAssigned))].UserID
			nextAssignee = &assigneeID
		}
	case chModel.AssignmentStrategyRoundRobin:
		if len(chore.Assignees) == 0 {
			return chore.AssignedTo, fmt.Errorf("no assignees available")
		}

		// Find current assignee index
		currentIndex := -1
		for i, assignee := range chore.Assignees {
			if chore.AssignedTo != nil && assignee.UserID == *chore.AssignedTo {
				currentIndex = i
				break
			}
		}

		// If current assignee is not found, start from the beginning
		var assigneeID int
		if currentIndex == -1 {
			assigneeID = chore.Assignees[0].UserID
		} else {
			nextIndex := (currentIndex + 1) % len(chore.Assignees)
			assigneeID = chore.Assignees[nextIndex].UserID
		}
		nextAssignee = &assigneeID
	default:
		return chore.AssignedTo, fmt.Errorf("invalid assign strategy")
	}
	return nextAssignee, nil
}

func remove(s []chModel.ChoreAssignees, i int) []chModel.ChoreAssignees {
	var targetIndex = indexOf(s, i)
	if targetIndex == -1 {
		return s
	}
	s[targetIndex] = s[len(s)-1]
	return s[:len(s)-1]
}

func indexOf(arr []chModel.ChoreAssignees, value int) int {
	for i, v := range arr {
		if v.UserID == value {
			return i
		}
	}
	return -1
}

type NudgeRequest struct {
	AllAssignees bool   `json:"all_assignees"`     // If true, send to all assignees; if false, send to current assignee
	Message      string `json:"message,omitempty"` // Optional custom message
}

// SendNudgeNotification godoc
//
//	@Summary		Send a nudge notification
//	@Description	Sends a reminder notification to chore assignee(s)
//	@Tags			chores
//	@Accept			json
//	@Produce		json
//	@Security		JWTKeyAuth && APIKeyAuth
//	@Param			id		path		int												true	"Chore ID"
//	@Param			nudge	body		object{all_assignees=boolean,message=string}	true	"Nudge request"
//	@Success		200		{object}	map[string]interface{}							"message: Nudge sent status"
//	@Failure		400		{object}	map[string]string								"error: Invalid chore ID | Invalid request payload | Chore has no assignees | Cannot nudge yourself"
//	@Failure		401		{object}	map[string]string								"error: Authentication failed"
//	@Failure		404		{object}	map[string]string								"error: Chore not found"
//	@Router			/chores/{id}/nudge [post]
func (h *Handler) sendNudgeNotification(c *gin.Context) {
	log := logging.FromContext(c)

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		log.Error("Failed to get current user from authentication context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	choreID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Error("Invalid chore ID", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chore ID"})
		return
	}

	var req NudgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error("Invalid request payload", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Get chore with assignees
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		log.Error("Chore not found or access denied", "error", err, "choreID", choreID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Chore not found"})
		return
	}

	// Determine target user IDs based on request
	var targetUserIDs []int

	if req.AllAssignees {
		// Send to all assignees
		if len(chore.Assignees) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Chore has no assignees"})
			return
		}
		for _, assignee := range chore.Assignees {
			targetUserIDs = append(targetUserIDs, assignee.UserID)
		}
	} else {
		// Send to current assignee only
		if chore.AssignedTo == nil || *chore.AssignedTo == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Chore has no current assignee"})
			return
		}
		targetUserIDs = []int{*chore.AssignedTo}
	}

	// Remove current user from targets (can't nudge yourself)
	filteredTargets := make([]int, 0, len(targetUserIDs))
	for _, userID := range targetUserIDs {
		if userID != currentUser.ID {
			filteredTargets = append(filteredTargets, userID)
		}
	}

	if len(filteredTargets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot nudge yourself or no valid targets found"})
		return
	}

	// Send nudge notifications to all target users
	totalDevicesSent := 0
	var errors []string

	for _, userID := range filteredTargets {
		deviceCount, err := h.sendNudgeToUser(c, userID, chore, currentUser, req.Message)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Failed to send to user %d: %v", userID, err))
			log.Error("Failed to send nudge to user", "error", err, "targetUserID", userID)
		} else {
			totalDevicesSent += deviceCount
		}
	}

	log.Infow("Nudge notification process completed",
		"fromUserID", currentUser.ID,
		"choreID", choreID,
		"targetUsers", len(filteredTargets),
		"totalDevicesSent", totalDevicesSent,
		"errors", len(errors))

	response := gin.H{
		"message": fmt.Sprintf("Nudge sent to %d user(s) across %d device(s)", len(filteredTargets)-len(errors), totalDevicesSent),
	}

	if len(errors) > 0 {
		response["warnings"] = errors
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) sendNudgeToUser(c context.Context, userID int, chore *chModel.Chore, fromUser *uModel.UserDetails, customMessage string) (int, error) {
	// Get all active device tokens for the target user
	deviceTokens, err := h.deviceRepo.GetActiveDeviceTokens(c, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get device tokens: %w", err)
	}

	if len(deviceTokens) == 0 {
		return 0, nil // No devices, but not an error
	}

	// Extract FCM tokens
	fcmTokens := make([]string, 0, len(deviceTokens))
	for _, deviceToken := range deviceTokens {
		if deviceToken.Token != "" {
			fcmTokens = append(fcmTokens, deviceToken.Token)
		}
	}

	if len(fcmTokens) == 0 {
		return 0, nil // No valid FCM tokens, but not an error
	}

	// Prepare notification content
	message := customMessage
	if message == "" {
		message = fmt.Sprintf("👋 %s nudged you about '%s'", fromUser.DisplayName, chore.Name)
	}

	title := "Gentle Nudge"

	// Send FCM notification to all devices
	err = h.sendNudgeToDevices(c, fcmTokens, title, message, chore, fromUser)
	if err != nil {
		return 0, fmt.Errorf("failed to send FCM notifications: %w", err)
	}

	return len(fcmTokens), nil
}

func (h *Handler) sendNudgeToDevices(c context.Context, fcmTokens []string, title, message string, chore *chModel.Chore, fromUser *uModel.UserDetails) error {
	// Create FCM payload
	payload := fcmService.FCMNotificationPayload{
		Title: title,
		Body:  message,
		Data: map[string]string{
			"type":         "nudge",
			"chore_id":     fmt.Sprintf("%d", chore.ID),
			"chore_name":   chore.Name,
			"from_user_id": fmt.Sprintf("%d", fromUser.ID),
			"from_user":    fromUser.DisplayName,
		},
	}

	// Get FCM notifier from the main notifier
	if h.notifier.FCM == nil {
		return fmt.Errorf("FCM notifier not available")
	}

	// Send multicast notification
	response, err := h.notifier.FCM.SendMulticast(c, fcmTokens, payload)
	if err != nil {
		return fmt.Errorf("failed to send multicast notification: %w", err)
	}

	// Log any failures
	if response.FailureCount > 0 {
		for i, result := range response.Responses {
			if !result.Success {
				logging.FromContext(c).Warn("Failed to send notification to token",
					"token_index", i,
					"error", result.Error)
			}
		}
	}

	return nil
}

func (h *Handler) undoChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		logger.Error("Failed to get current user from authentication context")
		c.JSON(401, gin.H{
			"error": "Authentication failed",
		})
		return
	}

	rawID := c.Param("id")
	choreID, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid chore ID",
		})
		return
	}

	// Check if user can access this chore
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve chore",
		})
		return
	}

	// Find user's last action on this chore within 5 minutes
	fiveMinutes := 5 * time.Minute
	lastAction, err := h.choreRepo.GetUserLastChoreAction(c, choreID, currentUser.ID, fiveMinutes)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(400, gin.H{
				"error": "No recent action found to undo (actions can only be undone within 5 minutes)",
			})
		} else {
			logger.Error("Failed to get user's last action", "error", err)
			c.JSON(500, gin.H{
				"error": "Failed to check recent actions",
			})
		}
		return
	}

	// Determine previous state based on action type
	var previousAssignedTo *int
	var previousDueDate *time.Time

	switch lastAction.Status {
	case chModel.ChoreHistoryStatusCompleted, chModel.ChoreHistoryStatusSkipped:
		// For completed/skipped, restore to the state before this completion
		// Get the previous completion/skip to determine what the assignee and due date should be
		previousHistory, err := h.choreRepo.GetChoreStateBefore(c, choreID, lastAction.ID)
		if err != nil {
			// No previous history found - restore to original state (assigned to original assignee)
			if len(chore.Assignees) > 0 {
				// Use the assignee from the action being undone as the original assignee
				previousAssignedTo = lastAction.AssignedTo
			}
			previousDueDate = lastAction.DueDate
		} else {
			// Use the state from before this action
			previousAssignedTo = previousHistory.AssignedTo
			previousDueDate = previousHistory.DueDate
		}

	case chModel.ChoreHistoryStatusPendingApproval:
		// For pending approval, restore to the state before submission
		previousAssignedTo = lastAction.AssignedTo
		previousDueDate = lastAction.DueDate

	case chModel.ChoreHistoryStatusRejected:
		// For rejected, restore to pending approval
		previousAssignedTo = lastAction.AssignedTo
		previousDueDate = lastAction.DueDate

	default:
		c.JSON(400, gin.H{
			"error": "Cannot undo this type of action",
		})
		return
	}

	// Perform the undo
	err = h.choreRepo.UndoChoreAction(c, choreID, lastAction.ID, previousAssignedTo, previousDueDate)
	if err != nil {
		logger.Error("Failed to undo chore action", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to undo action",
		})
		return
	}

	// Special handling for rejected actions - restore to pending approval
	if lastAction.Status == chModel.ChoreHistoryStatusRejected {
		if err := h.choreRepo.UpdateChoreFields(c, choreID, map[string]interface{}{
			"status": chModel.ChoreStatusPendingApproval,
		}); err != nil {
			logger.Error("Failed to restore pending approval status", "error", err)
			c.JSON(500, gin.H{
				"error": "Failed to restore pending approval status",
			})
			return
		}
	}

	// Get updated chore
	updatedChore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID)
	if err != nil {
		logger.Error("Failed to retrieve updated chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve updated chore",
		})
		return
	}

	// Broadcast real-time undo event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		changes := map[string]interface{}{
			"undoAction":  lastAction.Status,
			"undoneBy":    currentUser.ID,
			"undoneAt":    time.Now().UTC(),
			"assignedTo":  previousAssignedTo,
			"nextDueDate": previousDueDate,
			"status":      updatedChore.Status,
		}
		broadcaster.BroadcastChoreUpdated(updatedChore, &currentUser.User, changes, nil)
	}

	c.JSON(200, gin.H{
		"message": fmt.Sprintf("Successfully undid %s action", getActionName(lastAction.Status)),
		"res":     updatedChore,
	})
}

func getActionName(status chModel.ChoreHistoryStatus) string {
	switch status {
	case chModel.ChoreHistoryStatusCompleted:
		return "completion"
	case chModel.ChoreHistoryStatusSkipped:
		return "skip"
	case chModel.ChoreHistoryStatusPendingApproval:
		return "approval submission"
	case chModel.ChoreHistoryStatusRejected:
		return "rejection"
	default:
		return "action"
	}
}

func Routes(router *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {

	choresRoutes := router.Group("api/v1/chores")
	choresRoutes.Use(multiAuthMiddleware.MiddlewareFunc())
	choresRoutes.Use(auth.ImpersonationMiddleware(h.uRepo, h.circleRepo))
	{
		choresRoutes.GET("/", h.getChores)
		choresRoutes.GET("/archived", h.getArchivedChores)
		choresRoutes.GET("/history", h.getChoresHistory)
		choresRoutes.PUT("/", h.editChore)
		choresRoutes.PUT("/:id/priority", h.updatePriority)
		choresRoutes.POST("/", h.createChore)
		choresRoutes.GET("/:id", h.getChore)
		choresRoutes.PUT("/:id/subtask", h.UpdateSubtaskCompletedAt)
		choresRoutes.GET("/:id/details", h.GetChoreDetail)
		choresRoutes.GET("/:id/history", h.GetChoreHistory)
		choresRoutes.PUT("/:id/history/:history_id", h.ModifyHistory)
		choresRoutes.DELETE("/:id/history/:history_id", h.DeleteHistory)
		choresRoutes.POST("/:id/do", h.completeChore)
		choresRoutes.POST("/:id/skip", h.skipChore)
		choresRoutes.PUT("/:id/start", h.startChore)
		choresRoutes.PUT("/:id/pause", h.pauseChore)
		choresRoutes.GET("/:id/timer", h.GetChoreTimeSessions)
		choresRoutes.PUT("/:id/timer/reset", h.ResetChoreTimer)
		choresRoutes.PUT("/:id/assignee", h.updateAssignee)
		choresRoutes.PUT("/:id/dueDate", h.updateDueDate)
		choresRoutes.PUT("/:id/archive", h.archiveChore)
		choresRoutes.PUT("/:id/unarchive", h.UnarchiveChore)
		choresRoutes.POST("/:id/approve", h.approveChore)
		choresRoutes.POST("/:id/reject", h.rejectChore)
		choresRoutes.DELETE("/:id", h.deleteChore)
		choresRoutes.PUT("/:id/timer/:session_id", h.UpdateTimeSession)
		choresRoutes.DELETE("/:id/timer/:session_id", h.DeleteTimeSession)
		choresRoutes.POST("/:id/nudge", h.sendNudgeNotification)
		choresRoutes.POST("/:id/undo", h.undoChore)
	}
}
