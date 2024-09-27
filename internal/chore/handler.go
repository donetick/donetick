package chore

import (
	"encoding/json"
	"fmt"
	"html"
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
	nRepo "donetick.com/core/internal/notifier/repo"
	nps "donetick.com/core/internal/notifier/service"
	telegram "donetick.com/core/internal/notifier/telegram"
	tRepo "donetick.com/core/internal/thing/repo"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
)

type ThingTrigger struct {
	ID           int    `json:"thingID" binding:"required"`
	TriggerState string `json:"triggerState" binding:"required"`
	Condition    string `json:"condition"`
}

type ChoreReq struct {
	Name                 string                        `json:"name" binding:"required"`
	FrequencyType        chModel.FrequencyType         `json:"frequencyType"`
	ID                   int                           `json:"id"`
	DueDate              string                        `json:"dueDate"`
	Assignees            []chModel.ChoreAssignees      `json:"assignees"`
	AssignStrategy       string                        `json:"assignStrategy" binding:"required"`
	AssignedTo           int                           `json:"assignedTo"`
	IsRolling            bool                          `json:"isRolling"`
	IsActive             bool                          `json:"isActive"`
	Frequency            int                           `json:"frequency"`
	FrequencyMetadata    *chModel.FrequencyMetadata    `json:"frequencyMetadata"`
	Notification         bool                          `json:"notification"`
	NotificationMetadata *chModel.NotificationMetadata `json:"notificationMetadata"`
	Labels               []string                      `json:"labels"`
	ThingTrigger         *ThingTrigger                 `json:"thingTrigger"`
}
type Handler struct {
	choreRepo  *chRepo.ChoreRepository
	circleRepo *cRepo.CircleRepository
	notifier   *telegram.TelegramNotifier
	nPlanner   *nps.NotificationPlanner
	nRepo      *nRepo.NotificationRepository
	tRepo      *tRepo.ThingRepository
}

func NewHandler(cr *chRepo.ChoreRepository, circleRepo *cRepo.CircleRepository, nt *telegram.TelegramNotifier,
	np *nps.NotificationPlanner, nRepo *nRepo.NotificationRepository, tRepo *tRepo.ThingRepository) *Handler {
	return &Handler{
		choreRepo:  cr,
		circleRepo: circleRepo,
		notifier:   nt,
		nPlanner:   np,
		nRepo:      nRepo,
		tRepo:      tRepo,
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
	chores, err := h.choreRepo.GetChores(c, u.CircleID, u.ID)
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
	var choreReq ChoreReq
	if err := c.ShouldBindJSON(&choreReq); err != nil {
		log.Print(err)
		c.JSON(400, gin.H{
			"error": "Invalid request",
		})
		return
	}

	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
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

	freqencyMetadataBytes, err := json.Marshal(choreReq.FrequencyMetadata)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error marshalling frequency metadata",
		})
		return
	}
	stringFrequencyMetadata := string(freqencyMetadataBytes)

	notificationMetadataBytes, err := json.Marshal(choreReq.NotificationMetadata)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error marshalling notification metadata",
		})
		return
	}
	stringNotificationMetadata := string(notificationMetadataBytes)

	var stringLabels *string
	if len(choreReq.Labels) > 0 {
		var escapedLabels []string
		for _, label := range choreReq.Labels {
			escapedLabels = append(escapedLabels, html.EscapeString(label))
		}

		labels := strings.Join(escapedLabels, ",")
		stringLabels = &labels
	}
	createdChore := &chModel.Chore{

		Name:                 choreReq.Name,
		FrequencyType:        choreReq.FrequencyType,
		Frequency:            choreReq.Frequency,
		FrequencyMetadata:    &stringFrequencyMetadata,
		NextDueDate:          dueDate,
		AssignStrategy:       choreReq.AssignStrategy,
		AssignedTo:           choreReq.AssignedTo,
		IsRolling:            choreReq.IsRolling,
		UpdatedBy:            currentUser.ID,
		IsActive:             true,
		Notification:         choreReq.Notification,
		NotificationMetadata: &stringNotificationMetadata,
		Labels:               stringLabels,
		CreatedBy:            currentUser.ID,
		CreatedAt:            time.Now().UTC(),
		CircleID:             currentUser.CircleID,
	}
	id, err := h.choreRepo.CreateChore(c, createdChore)
	createdChore.ID = id

	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error creating chore",
		})
		return
	}

	var choreAssignees []*chModel.ChoreAssignees
	for _, assignee := range choreReq.Assignees {
		choreAssignees = append(choreAssignees, &chModel.ChoreAssignees{
			ChoreID: id,
			UserID:  assignee.UserID,
		})
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
	shouldReturn := HandleThingAssociation(choreReq, h, c, currentUser)
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

	var choreReq ChoreReq
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
	if currentUser.ID != oldChore.CreatedBy {
		c.JSON(403, gin.H{
			"error": "You are not allowed to edit this chore",
		})
		return
	}
	freqencyMetadataBytes, err := json.Marshal(choreReq.FrequencyMetadata)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error marshalling frequency metadata",
		})
		return
	}

	stringFrequencyMetadata := string(freqencyMetadataBytes)

	notificationMetadataBytes, err := json.Marshal(choreReq.NotificationMetadata)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error marshalling notification metadata",
		})
		return
	}
	stringNotificationMetadata := string(notificationMetadataBytes)

	// escape special characters in labels and store them as a string :
	var stringLabels *string
	if len(choreReq.Labels) > 0 {
		var escapedLabels []string
		for _, label := range choreReq.Labels {
			escapedLabels = append(escapedLabels, html.EscapeString(label))
		}

		labels := strings.Join(escapedLabels, ",")
		stringLabels = &labels
	}
	updatedChore := &chModel.Chore{
		ID:                choreReq.ID,
		Name:              choreReq.Name,
		FrequencyType:     choreReq.FrequencyType,
		Frequency:         choreReq.Frequency,
		FrequencyMetadata: &stringFrequencyMetadata,
		// Assignees:         &assignees,
		NextDueDate:          dueDate,
		AssignStrategy:       choreReq.AssignStrategy,
		AssignedTo:           choreReq.AssignedTo,
		IsRolling:            choreReq.IsRolling,
		IsActive:             choreReq.IsActive,
		Notification:         choreReq.Notification,
		NotificationMetadata: &stringNotificationMetadata,
		Labels:               stringLabels,
		CircleID:             oldChore.CircleID,
		UpdatedBy:            currentUser.ID,
		CreatedBy:            oldChore.CreatedBy,
		CreatedAt:            oldChore.CreatedAt,
	}
	if err := h.choreRepo.UpsertChore(c, updatedChore); err != nil {
		c.JSON(500, gin.H{
			"error": "Error adding chore",
		})
		return
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
	if oldChore.ThingChore != nil {
		// TODO: Add check to see if dissociation is necessary
		h.tRepo.DissociateThingWithChore(c, oldChore.ThingChore.ThingID, oldChore.ID)

	}
	shouldReturn := HandleThingAssociation(choreReq, h, c, currentUser)
	if shouldReturn {
		return
	}

	c.JSON(200, gin.H{
		"message": "Chore added successfully",
	})
}

func HandleThingAssociation(choreReq ChoreReq, h *Handler, c *gin.Context, currentUser *uModel.User) bool {
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

	if err := h.choreRepo.DeleteChore(c, id); err != nil {
		c.JSON(500, gin.H{
			"error": "Error deleting chore",
		})
		return
	}
	h.nRepo.DeleteAllChoreNotifications(id)
	h.tRepo.DissociateChoreWithThing(c, id)

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
		Assignee int `json:"assignee" binding:"required"`
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

	chore.UpdatedBy = currentUser.ID
	chore.AssignedTo = assigneeReq.Assignee
	if err := h.choreRepo.UpsertChore(c, chore); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating assignee",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
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
	nextDueDate, err := scheduleNextDueDate(chore, chore.NextDueDate.UTC())
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error scheduling next due date",
		})
		return
	}

	nextAssigedTo := chore.AssignedTo
	if err := h.choreRepo.CompleteChore(c, chore, nil, currentUser.ID, nextDueDate, nil, nextAssigedTo); err != nil {
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
		DueDate string `json:"dueDate" binding:"required"`
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
	chore.NextDueDate = &dueDate
	chore.UpdatedBy = currentUser.ID
	if err := h.choreRepo.UpsertChore(c, chore); err != nil {
		c.JSON(500, gin.H{
			"error": "Error updating due date",
		})
		return
	}

	c.JSON(200, gin.H{
		"res": chore,
	})
}
func (h *Handler) completeChore(c *gin.Context) {
	type CompleteChoreReq struct {
		Note string `json:"note"`
	}
	var req CompleteChoreReq
	currentUser, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(500, gin.H{
			"error": "Error getting current user",
		})
		return
	}
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
		nextDueDate, err = scheduleNextDueDate(chore, completedDate)
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

	nextAssignedTo, err := checkNextAssignee(chore, choreHistory, currentUser.ID)
	if err != nil {
		log.Printf("Error checking next assignee: %s", err)
		c.JSON(500, gin.H{
			"error": "Error checking next assignee",
		})
		return
	}

	if err := h.choreRepo.CompleteChore(c, chore, additionalNotes, currentUser.ID, nextDueDate, &completedDate, nextAssignedTo); err != nil {
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
	go func() {
		h.notifier.SendChoreCompletion(c, chore, currentUser)
		h.nPlanner.GenerateNotifications(c, updatedChore)
	}()
	c.JSON(200, gin.H{
		"res": updatedChore,
	})
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
		CompletedAt *time.Time `json:"completedAt"`
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
	if req.CompletedAt != nil {
		history.CompletedAt = req.CompletedAt
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
	case "least_assigned":
		// find the assignee with the least number of chores
		assigneeChores := map[int]int{}
		for _, performer := range chore.Assignees {
			assigneeChores[performer.UserID] = 0
		}
		for _, history := range history {
			if ok := assigneesMap[history.AssignedTo]; !ok {
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
	case "least_completed":
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
	case "random":
		nextAssignee = chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
	case "keep_last_assigned":
		// keep the last assignee
		nextAssignee = chore.AssignedTo
	case "random_except_last_assigned":
		var lastAssigned = chore.AssignedTo
		AssigneesCopy := make([]chModel.ChoreAssignees, len(chore.Assignees))
		copy(AssigneesCopy, chore.Assignees)
		var removeLastAssigned = remove(AssigneesCopy, lastAssigned)
		nextAssignee = removeLastAssigned[rand.Intn(len(removeLastAssigned))].UserID

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

	choresRoutes := router.Group("chores")
	choresRoutes.Use(auth.MiddlewareFunc())
	{
		choresRoutes.GET("/", h.getChores)
		choresRoutes.PUT("/", h.editChore)
		choresRoutes.POST("/", h.createChore)
		choresRoutes.GET("/:id", h.getChore)
		choresRoutes.GET("/:id/details", h.GetChoreDetail)
		choresRoutes.GET("/:id/history", h.GetChoreHistory)
		choresRoutes.PUT("/:id/history/:history_id", h.ModifyHistory)
		choresRoutes.DELETE("/:id/history/:history_id", h.DeleteHistory)
		choresRoutes.POST("/:id/do", h.completeChore)
		choresRoutes.POST("/:id/skip", h.skipChore)
		choresRoutes.PUT("/:id/assignee", h.updateAssignee)
		choresRoutes.PUT("/:id/dueDate", h.updateDueDate)
		choresRoutes.DELETE("/:id", h.deleteChore)
	}

}
