package chore

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	auth "donetick.com/core/internal/authorization"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	cRepo "donetick.com/core/internal/circle/repo"
	"donetick.com/core/internal/events"
	lRepo "donetick.com/core/internal/label/repo"
	"donetick.com/core/internal/notifier"
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/realtime"
	storage "donetick.com/core/internal/storage"
	storageModel "donetick.com/core/internal/storage/model"
	storageRepo "donetick.com/core/internal/storage/repo"
	stModel "donetick.com/core/internal/subtask/model"
	stRepo "donetick.com/core/internal/subtask/repo"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	choreRepo       *chRepo.ChoreRepository
	circleRepo      *cRepo.CircleRepository
	notifier        *notifier.Notifier
	nPlanner        *nps.NotificationPlanner
	nRepo           *nRepo.NotificationRepository
	tRepo           *tRepo.ThingRepository
	lRepo           *lRepo.LabelRepository
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
	stoRepo *storageRepo.StorageRepository,
	rts *realtime.RealTimeService) *Handler {
	return &Handler{
		choreRepo:       cr,
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

func (h *Handler) getChores(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current circle",
		})
		return
	}
	includeArchived := false

	if c.Query("includeArchived") == "true" {
		includeArchived = true
	}

	chores, err := h.choreRepo.GetChores(c, u.CircleID, u.ID, includeArchived)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chores",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chores,
	})
}

func (h *Handler) getArchivedChores(c *gin.Context) {
	u, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current circle",
		})
		return
	}
	chores, err := h.choreRepo.GetArchivedChores(c, u.CircleID, u.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chores",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chores,
	})
}
func (h *Handler) getChore(c *gin.Context) {

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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

	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	isAssignee := false

	for _, assignee := range chore.Assignees {
		if assignee.UserID == currentUser.ID {
			isAssignee = true
			break
		}
	}

	if currentUser.ID != chore.CreatedBy && !isAssignee {
		c.JSON(403, gin.H{
			"error": "You are not allowed to view this chore",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
}

func (h *Handler) createChore(c *gin.Context) {
	logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)

	logger.Debug("Create chore", "currentUser", currentUser)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	// Validate chore:
	var choreReq chModel.ChoreReq
	if err := c.ShouldBindJSON(&choreReq); err != nil {
		log.Print(err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Print(err)
		c.JSON(500, gin.H{"error": "Error getting circle users"})
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
	if choreReq.AssignedTo <= 0 && len(choreReq.Assignees) > 0 {
		// if the assigned to field is not set, randomly assign the chore to one of the assignees
		choreReq.AssignedTo = choreReq.Assignees[rand.Intn(len(choreReq.Assignees))].UserID
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
		// SubTasks removed to prevent duplicate creation - handled by UpdateSubtask call below
		// it's need custom logic to handle subtask creation as we send negative ids sometimes when we creating parent child releationship
		// when the subtask is not yet created
	}
	id, err := h.choreRepo.CreateChore(c, createdChore)
	createdChore.ID = id

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating chore",
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
	if err := h.choreRepo.UpdateChoreAssignees(c, choreAssignees); err != nil {
		c.JSON(500, gin.H{
			"error": "Error adding chore assignees",
		})
		return
	}
	go func() {
		h.nPlanner.GenerateNotifications(c, createdChore)
	}()

	// Broadcast real-time chore creation event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		broadcaster.BroadcastChoreCreated(createdChore, &currentUser.User)
	}

	shouldReturn := HandleThingAssociation(choreReq, h, c, &currentUser.User)
	if shouldReturn {
		return
	}
	c.JSON(200, gin.H{
		"res": id,
	})
}

func (h *Handler) editChore(c *gin.Context) {
	// logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	var choreReq chModel.ChoreReq
	if err := c.ShouldBindJSON(&choreReq); err != nil {
		log.Print(err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting circle users",
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
	assigneeFound := false
	for _, assignee := range choreReq.Assignees {
		if assignee.UserID == choreReq.AssignedTo {
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

	if choreReq.AssignedTo <= 0 && len(choreReq.Assignees) > 0 {
		// if the assigned to field is not set, randomly assign the chore to one of the assignees
		choreReq.AssignedTo = choreReq.Assignees[rand.Intn(len(choreReq.Assignees))].UserID
	}
	oldChore, err := h.choreRepo.GetChore(c, choreReq.ID)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
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
	shouldReturn := HandleThingAssociation(choreReq, h, c, &currentUser.User)
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

func HandleThingAssociation(choreReq chModel.ChoreReq, h *Handler, c *gin.Context, currentUser *uModel.User) bool {
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
		if err := h.tRepo.AssociateThingWithChore(c, choreReq.ThingTrigger.ID, choreReq.ID, choreReq.ThingTrigger.TriggerState, choreReq.ThingTrigger.Condition); err != nil {
			c.JSON(500, gin.H{
				"error": "Error associating thing with chore",
			})
			return true
		}

	}
	return false
}

func (h *Handler) deleteChore(c *gin.Context) {
	// logger := logging.FromContext(c)
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	if err := h.choreRepo.DeleteChore(c, id); err != nil {
		c.JSON(500, gin.H{
			"error": "Error deleting chore",
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

func (h *Handler) updateAssignee(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
		log.Print(err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
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
		c.JSON(500, gin.H{
			"error": "Error getting circle users",
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
		updatedChore, err := h.choreRepo.GetChore(c, id)
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

func (h *Handler) updateChoreStatus(c *gin.Context) {
	type StatusReq struct {
		Status *chModel.Status `json:"status" binding:"required"`
	}

	var statusReq StatusReq

	if err := c.ShouldBindJSON(&statusReq); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
	}

	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	if chore.CircleID != currentUser.CircleID {

		c.JSON(403, gin.H{
			"error": "You are not allowed to start this chore",
		})
		return
	}
	if err := h.choreRepo.UpdateChoreStatus(c, chore.ID, currentUser.ID, *statusReq.Status); err != nil {

		c.JSON(500, gin.H{
			"error": "Error starting chore",
		})
		return
	}
	c.JSON(200, gin.H{})

}

func (h *Handler) skipChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
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

	nextAssigedTo := chore.AssignedTo
	if err := h.choreRepo.SkipChore(c, chore, currentUser.ID, nextDueDate, nextAssigedTo); err != nil {
		c.JSON(500, gin.H{
			"error": "Error completing chore",
		})
		return
	}
	updatedChore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	h.eventProducer.ChoreSkipped(c, currentUser.WebhookURL, updatedChore, &currentUser.User)

	// Broadcast real-time chore skip event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Get the skip history entry
		history, _ := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 1)
		var choreHistory *chModel.ChoreHistory
		if len(history) > 0 {
			choreHistory = history[0]
		}
		broadcaster.BroadcastChoreSkipped(updatedChore, &currentUser.User, choreHistory, nil)
	}

	c.JSON(200, gin.H{
		"res": updatedChore,
	})
}

func (h *Handler) updateDueDate(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	type DueDateReq struct {
		DueDate   string    `json:"dueDate" binding:"required"`
		UpdatedAt time.Time `json:"updatedAt" binding:"required"`
	}

	var dueDateReq DueDateReq
	if err := c.ShouldBindJSON(&dueDateReq); err != nil {
		log.Print(err)
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

	rawDueDate, err := time.Parse(time.RFC3339, dueDateReq.DueDate)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid date",
		})
		return
	}
	dueDate := rawDueDate.UTC()
	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting circle users",
		})
		return
	}
	if err := chore.CanEdit(currentUser.ID, circleUsers, &dueDateReq.UpdatedAt); err != nil {
		c.JSON(403, gin.H{
			"error": fmt.Sprintf("You cannot edit this chore: %s", err.Error()),
		})
		return
	}
	if err := h.choreRepo.UpdateChoreFields(c, chore.ID, map[string]interface{}{
		"next_due_date": dueDate,
		"updated_by":    currentUser.ID,
		"updated_at":    time.Now().UTC(),
	}); err != nil {
		log.Printf("Error updating due date: %s", err)
		c.JSON(500, gin.H{
			"error": "Error updating due date",
		})
		return
	}

	// Broadcast real-time due date update event
	if h.realTimeService != nil {
		updatedChore, err := h.choreRepo.GetChore(c, chore.ID)
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
func (h *Handler) archiveChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
		updatedChore, err := h.choreRepo.GetChore(c, id)
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

func (h *Handler) UnarchiveChore(c *gin.Context) {
	rawID := c.Param("id")
	id, err := strconv.Atoi(rawID)

	if err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
		updatedChore, err := h.choreRepo.GetChore(c, id)
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

func (h *Handler) completeChore(c *gin.Context) {
	type CompleteChoreReq struct {
		Note string `json:"note"`
		// the completed by only can be populated by the admin or super user
		CompletedBy *int `json:"completedBy"`
	}
	var req CompleteChoreReq
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
	completedBy := currentUser.ID
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
	chore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	if !chore.CanComplete(currentUser.ID) {
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
		ok := authorizeChoreCompletionForUser(h, c, currentUser, req.CompletedBy)
		if !ok {
			return
		}
		completedBy = *req.CompletedBy
	}
	var nextDueDate *time.Time
	if chore.FrequencyType == "adaptive" {
		history, err := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 5)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting chore history",
			})
			return
		}
		nextDueDate, err = scheduleAdaptiveNextDueDate(chore, completedDate, history)
		if err != nil {
			log.Printf("Error scheduling next due date: %s", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}

	} else {
		nextDueDate, err = scheduleNextDueDate(c, chore, completedDate.UTC())
		if err != nil {
			log.Printf("Error scheduling next due date: %s", err)
			c.JSON(500, gin.H{
				"error": "Error scheduling next due date",
			})
			return
		}
	}
	choreHistory, err := h.choreRepo.GetChoreHistory(c, chore.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	nextAssignedTo, err := checkNextAssignee(chore, choreHistory, completedBy)
	if err != nil {
		log.Printf("Error checking next assignee: %s", err)
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
	updatedChore, err := h.choreRepo.GetChore(c, id)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	if updatedChore.SubTasks != nil && updatedChore.FrequencyType != chModel.FrequencyTypeOnce {
		h.stRepo.ResetSubtasksCompletion(c, updatedChore.ID)
	}

	// go func() {

	// 	h.notifier.SendChoreCompletion(c, chore, currentUser)
	// }()
	h.nPlanner.GenerateNotifications(c, updatedChore)
	h.eventProducer.ChoreCompleted(c, currentUser.WebhookURL, chore, &currentUser.User)
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()
		// Get the completion history entry
		history, _ := h.choreRepo.GetChoreHistoryWithLimit(c, chore.ID, 1)

		var choreHistory *chModel.ChoreHistory
		if len(history) > 0 {
			choreHistory = history[0]
		}
		broadcaster.BroadcastChoreCompleted(updatedChore, &currentUser.User, choreHistory, additionalNotes)
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
		if circleUser.UserID == currentUser.ID && circleUser.Role == "admin" {
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
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": choreHistory,
	})
}

func (h *Handler) GetChoreDetail(c *gin.Context) {

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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

	detailed, err := h.choreRepo.GetChoreDetailByID(c, id, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": detailed,
	})
}

func (h *Handler) ModifyHistory(c *gin.Context) {

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
		log.Print(err)
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
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	if currentUser.ID != history.CompletedBy || currentUser.ID != history.AssignedTo {
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

func (h *Handler) updatePriority(c *gin.Context) {
	type PriorityReq struct {
		Priority *int `json:"priority" binding:"required,gt=-1,lt=5"`
	}

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}

	var priorityReq PriorityReq
	if err := c.ShouldBindJSON(&priorityReq); err != nil {
		log.Print(err)
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

	if err := h.choreRepo.UpdateChorePriority(c, currentUser.ID, id, *priorityReq.Priority); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating priority",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "Priority updated successfully",
	})
}

func (h *Handler) getChoresHistory(c *gin.Context) {

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}
	c.JSON(200, gin.H{
		"res": choreHistories,
	})
}

func (h *Handler) DeleteHistory(c *gin.Context) {

	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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

	history, err := h.choreRepo.GetChoreHistoryByID(c, choreID, historyID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}

	if currentUser.ID != history.CompletedBy || currentUser.ID != history.AssignedTo {
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

func (h *Handler) UpdateSubtaskCompletedAt(c *gin.Context) {
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
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

	type SubtaskReq struct {
		ID          int        `json:"id"`
		ChoreID     int        `json:"choreId"`
		CompletedAt *time.Time `json:"completedAt"`
	}

	var req SubtaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Print(err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}
	chore, err := h.choreRepo.GetChore(c, choreID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	if !chore.CanComplete(currentUser.ID) {
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}
	var completedAt *time.Time
	if req.CompletedAt != nil {
		completedAt = req.CompletedAt
	}
	err = h.stRepo.UpdateSubTaskStatus(c, currentUser.ID, req.ID, completedAt)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting subtask",
		})
		return
	}
	// h.choreRepo.setStatus(c, choreID, chModel.ChoreStatusInProgress, currentUser.ID)

	// Broadcast real-time subtask event
	if h.realTimeService != nil {
		broadcaster := h.realTimeService.GetEventBroadcaster()

		broadcaster.BroadcastSubtaskUpdated(
			choreID,
			req.ID,
			completedAt,
			&currentUser.User,
			chore.CircleID,
		)

	}

	h.eventProducer.SubtaskUpdated(c, currentUser.WebhookURL,
		&stModel.SubTask{
			ID:          req.ID,
			ChoreID:     req.ChoreID,
			CompletedAt: completedAt,
			CompletedBy: currentUser.ID,
		},
	)
	c.JSON(200, gin.H{})

}

func checkNextAssignee(chore *chModel.Chore, choresHistory []*chModel.ChoreHistory, performerID int) (int, error) {
	// copy the history to avoid modifying the original:
	history := make([]*chModel.ChoreHistory, len(choresHistory))
	copy(history, choresHistory)

	assigneesMap := map[int]bool{}
	for _, assignee := range chore.Assignees {
		assigneesMap[assignee.UserID] = true
	}
	var nextAssignee int
	if len(history) == 0 {
		// if there is no history, just assume the current operation as the first
		history = append(history, &chModel.ChoreHistory{
			AssignedTo: performerID,
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
			if ok := assigneesMap[history.AssignedTo]; ok {
				// calculate the number of chores assigned to each assignee
				assigneeChores[history.AssignedTo]++
			}
		}

		var minChores int64 = math.MaxInt64
		for assignee, numChores := range assigneeChores {
			// if this is the first assignee or if the number of
			// chores assigned to this assignee is less than the current minimum
			if int64(numChores) < minChores {
				minChores = int64(numChores)
				// set the next assignee to this assignee
				nextAssignee = assignee
			}
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

		for assignee, numChores := range assigneeChores {
			// if this is the first assignee or if the number of
			// chores completed by this assignee is less than the current minimum
			if int64(numChores) < minChores {
				minChores = int64(numChores)
				// set the next assignee to this assignee
				nextAssignee = assignee
			}
		}
	case chModel.AssignmentStrategyRandom:
		nextAssignee = chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
	case chModel.AssignmentStrategyKeepLastAssigned:
		// keep the last assignee
		nextAssignee = chore.AssignedTo
	case chModel.AssignmentStrategyRandomExceptLastAssigned:
		var lastAssigned = chore.AssignedTo
		AssigneesCopy := make([]chModel.ChoreAssignees, len(chore.Assignees))
		copy(AssigneesCopy, chore.Assignees)
		var removeLastAssigned = remove(AssigneesCopy, lastAssigned)
		nextAssignee = removeLastAssigned[rand.Intn(len(removeLastAssigned))].UserID
	case chModel.AssignmentStrategyRoundRobin:
		if len(chore.Assignees) == 0 {
			return chore.AssignedTo, fmt.Errorf("no assignees available")
		}

		// Find current assignee index
		currentIndex := -1
		for i, assignee := range chore.Assignees {
			if assignee.UserID == chore.AssignedTo {
				currentIndex = i
				break
			}
		}

		// If current assignee is not found, start from the beginning
		if currentIndex == -1 {
			nextAssignee = chore.Assignees[0].UserID
		} else {
			nextIndex := (currentIndex + 1) % len(chore.Assignees)
			nextAssignee = chore.Assignees[nextIndex].UserID
		}
	default:
		return chore.AssignedTo, fmt.Errorf("invalid assign strategy")

	}
	return nextAssignee, nil
}

func remove(s []chModel.ChoreAssignees, i int) []chModel.ChoreAssignees {
	var removalIndex = indexOf(s, i)
	if removalIndex == -1 {
		return s
	}

	s[removalIndex] = s[len(s)-1]
	return s[:len(s)-1]
}

func indexOf(arr []chModel.ChoreAssignees, value int) int {
	for i, v := range arr {
		if v.UserID == value {
			return i
		}
	}
	return -1 // Return -1 if the value is not found
}

func Routes(router *gin.Engine, h *Handler, auth *jwt.GinJWTMiddleware) {

	choresRoutes := router.Group("api/v1/chores")
	choresRoutes.Use(auth.MiddlewareFunc())
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
		choresRoutes.PUT("/:id/status", h.updateChoreStatus)
		choresRoutes.PUT("/:id/assignee", h.updateAssignee)
		choresRoutes.PUT("/:id/dueDate", h.updateDueDate)
		choresRoutes.PUT("/:id/archive", h.archiveChore)
		choresRoutes.PUT("/:id/unarchive", h.UnarchiveChore)
		choresRoutes.DELETE("/:id", h.deleteChore)
	}

}
