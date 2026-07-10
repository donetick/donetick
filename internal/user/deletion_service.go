package user

import (
	"context"
	"fmt"
	"log"
	"strings"

	"donetick.com/core/internal/storage"
	uModel "donetick.com/core/internal/user/model"
	"gorm.io/gorm"
)

type DeletionService struct {
	db      *gorm.DB
	storage storage.Storage
}

func NewDeletionService(db *gorm.DB, storage storage.Storage) *DeletionService {
	return &DeletionService{
		db:      db,
		storage: storage,
	}
}

type DeletionResult struct {
	Success          bool                   `json:"success"`
	Message          string                 `json:"message"`
	DeletedData      map[string]int         `json:"deletedData"`
	CirclesLeft      []CircleTransferInfo   `json:"circlesLeft,omitempty"`
	RequiresTransfer bool                   `json:"requiresTransfer"`
	TransferOptions  []CircleTransferOption `json:"transferOptions,omitempty"`
}

type CircleTransferInfo struct {
	CircleID    int    `json:"circleId"`
	CircleName  string `json:"circleName"`
	MemberCount int    `json:"memberCount"`
}

type CircleTransferOption struct {
	CircleID     int    `json:"circleId"`
	NewOwnerID   int    `json:"newOwnerId"`
	NewOwnerName string `json:"newOwnerName"`
}

func (s *DeletionService) DeleteUserAccount(ctx context.Context, userID int, transferOptions []CircleTransferOption) (*DeletionResult, error) {
	return s.deleteUserAccount(ctx, userID, transferOptions, false)
}

func (s *DeletionService) CheckUserAccountDeletion(ctx context.Context, userID int) (*DeletionResult, error) {
	return s.deleteUserAccount(ctx, userID, nil, true)
}

func (s *DeletionService) deleteUserAccount(ctx context.Context, userID int, transferOptions []CircleTransferOption, dryRun bool) (*DeletionResult, error) {
	// Start transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	result := &DeletionResult{
		DeletedData: make(map[string]int),
	}

	// Check if this is a parent user and get their child users
	childUsers, err := s.getChildUsers(tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child users: %w", err)
	}

	// Check if user owns circles that need transfer
	circlesOwned, err := s.getOwnedCircles(tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check owned circles: %w", err)
	}

	// Handle circle ownership transfer
	if len(circlesOwned) > 0 && len(transferOptions) == 0 {
		// User owns circles but hasn't provided transfer options
		transferOpts, err := s.getTransferOptions(tx, circlesOwned)
		if err != nil {
			return nil, fmt.Errorf("failed to get transfer options: %w", err)
		}

		result.RequiresTransfer = true
		result.TransferOptions = transferOpts
		result.CirclesLeft = circlesOwned
		result.Message = "Account deletion requires circle ownership transfer"
		return result, nil
	}

	// For dry run, return what would be deleted without actually deleting
	if dryRun {
		// Count what would be deleted
		result.Message = "Preview of what would be deleted"
		result.Success = true

		// Count records that would be deleted
		deletionSteps := []struct {
			name     string
			function func(*gorm.DB, int) (int, error)
		}{
			{"mfa_sessions", s.countMFASessions},
			{"api_tokens", s.countAPITokens},
			{"password_reset_tokens", s.countPasswordResetTokens},
			{"user_notification_targets", s.countNotificationTargets},
			{"notifications", s.countNotifications},
			{"time_sessions", s.countTimeSessions},
			{"chore_history", s.countChoreHistory},
			{"chore_assignees", s.countChoreAssignees},
			{"chore_labels", s.countChoreLabels},
			{"subtasks", s.countUserSubtasks},
			{"chores", s.countUserChores},
			{"points_history", s.countPointsHistory},
			{"storage_files", s.countStorageFiles},
			{"storage_usage", s.countStorageUsage},
			{"user_circles", s.countUserCircles},
			{"labels", s.countUserLabels},
			{"things", s.countUserThings},
		}

		for _, step := range deletionSteps {
			count, err := step.function(tx, userID)
			if err != nil {
				return nil, fmt.Errorf("failed to count %s: %w", step.name, err)
			}
			if count > 0 {
				result.DeletedData[step.name] = count
			}
		}

		// Count the user record itself
		result.DeletedData["user"] = 1

		return result, nil
	}

	// Transfer circle ownership if needed
	if len(circlesOwned) > 0 {
		err = s.transferCircleOwnership(tx, transferOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer circle ownership: %w", err)
		}
	}

	// Delete child users first if this is a parent user
	if len(childUsers) > 0 && !dryRun {
		for _, childUserID := range childUsers {
			childResult, err := s.deleteUserAccount(ctx, childUserID, nil, false)
			if err != nil {
				return nil, fmt.Errorf("failed to delete child user %d: %w", childUserID, err)
			}
			// Aggregate child deletion counts
			for key, count := range childResult.DeletedData {
				result.DeletedData[key] += count
			}
		}
	}

	// For dry run, count child users that would be deleted
	if dryRun && len(childUsers) > 0 {
		for _, childUserID := range childUsers {
			childResult, err := s.deleteUserAccount(ctx, childUserID, nil, true)
			if err != nil {
				return nil, fmt.Errorf("failed to count child user %d data: %w", childUserID, err)
			}
			// Aggregate child deletion counts
			for key, count := range childResult.DeletedData {
				result.DeletedData[key] += count
			}
		}
	}

	// Delete user-related data in order (respecting foreign key constraints)
	deletionSteps := []struct {
		name     string
		function func(*gorm.DB, int) (int, error)
	}{
		{"mfa_sessions", s.deleteMFASessions},
		{"api_tokens", s.deleteAPITokens},
		{"password_reset_tokens", s.deletePasswordResetTokens},
		{"user_notification_targets", s.deleteNotificationTargets},
		{"notifications", s.deleteNotifications},
		{"time_sessions", s.deleteTimeSessions},
		{"chore_history", s.deleteChoreHistory},
		{"chore_assignees", s.deleteChoreAssignees},
		{"chore_labels", s.deleteChoreLabels},
		{"subtasks", s.deleteUserSubtasks},
		{"chores", s.deleteUserChores},
		{"points_history", s.deletePointsHistory},
		{"storage_files", s.deleteStorageFiles},
		{"storage_usage", s.deleteStorageUsage},
		{"user_circles", s.deleteUserCircles},
		{"labels", s.deleteUserLabels},
		{"things", s.deleteUserThings},
	}

	for _, step := range deletionSteps {
		count, err := step.function(tx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to delete %s: %w", step.name, err)
		}
		if count > 0 {
			result.DeletedData[step.name] = count
		}
	}

	// Delete storage files from storage backend
	err = s.deleteUserStorageFiles(ctx, userID)
	if err != nil {
		log.Printf("Warning: Failed to delete storage files for user %d: %v", userID, err)
		// Don't fail the entire operation for storage cleanup issues
	}

	// Finally, delete the user record
	userResult := tx.Delete(&uModel.User{}, userID)
	if userResult.Error != nil {
		return nil, fmt.Errorf("failed to delete user record: %w", userResult.Error)
	}
	result.DeletedData["user"] = int(userResult.RowsAffected)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result.Success = true
	result.Message = "Account deleted successfully"
	return result, nil
}

func (s *DeletionService) getChildUsers(tx *gorm.DB, userID int) ([]int, error) {
	var childUserIDs []int
	err := tx.Model(&uModel.User{}).Where("parent_user_id = ?", userID).Pluck("id", &childUserIDs).Error
	if err != nil {
		return nil, err
	}
	return childUserIDs, nil
}

func (s *DeletionService) getOwnedCircles(tx *gorm.DB, userID int) ([]CircleTransferInfo, error) {
	var circles []CircleTransferInfo

	query := `
		SELECT c.id as circle_id, c.name as circle_name, 
		       COUNT(uc.user_id) as member_count
		FROM circles c
		LEFT JOIN user_circles uc ON c.id = uc.circle_id AND uc.is_active = true
		WHERE c.created_by = ?
		GROUP BY c.id, c.name
	`

	rows, err := tx.Raw(query, userID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var circle CircleTransferInfo
		if err := rows.Scan(&circle.CircleID, &circle.CircleName, &circle.MemberCount); err != nil {
			return nil, err
		}
		circles = append(circles, circle)
	}

	return circles, nil
}

func (s *DeletionService) getTransferOptions(tx *gorm.DB, circles []CircleTransferInfo) ([]CircleTransferOption, error) {
	var options []CircleTransferOption

	for _, circle := range circles {
		// Get admin members of the circle who can take ownership
		query := `
			SELECT uc.user_id, u.display_name
			FROM user_circles uc
			JOIN users u ON uc.user_id = u.id
			WHERE uc.circle_id = ? AND uc.role = 'admin' AND uc.is_active = true
		`

		rows, err := tx.Raw(query, circle.CircleID).Rows()
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var option CircleTransferOption
			if err := rows.Scan(&option.NewOwnerID, &option.NewOwnerName); err != nil {
				rows.Close()
				return nil, err
			}
			option.CircleID = circle.CircleID
			options = append(options, option)
		}
		rows.Close()
	}

	return options, nil
}

func (s *DeletionService) transferCircleOwnership(tx *gorm.DB, options []CircleTransferOption) error {
	for _, option := range options {
		result := tx.Model(&struct {
			ID        int `gorm:"column:id"`
			CreatedBy int `gorm:"column:created_by"`
		}{}).
			Table("circles").
			Where("id = ?", option.CircleID).
			Update("created_by", option.NewOwnerID)

		if result.Error != nil {
			return result.Error
		}
	}
	return nil
}

// Deletion functions for each data type
func (s *DeletionService) deleteMFASessions(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM mfa_sessions WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteAPITokens(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM api_tokens WHERE user_id = ?", userID)
}

func (s *DeletionService) deletePasswordResetTokens(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM user_password_resets WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteNotificationTargets(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM user_notification_targets WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteNotifications(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM notifications WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteTimeSessions(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM time_sessions WHERE chore_id IN (SELECT id FROM chores WHERE created_by = ?) OR updated_by = ?", userID, userID)
}

func (s *DeletionService) deleteChoreHistory(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM chore_histories WHERE completed_by = ? OR chore_id IN (SELECT id FROM chores WHERE created_by = ?)", userID, userID)
}

func (s *DeletionService) deleteChoreAssignees(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM chore_assignees WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteChoreLabels(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM chore_labels WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteUserChores(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM chores WHERE created_by = ?", userID)
}

func (s *DeletionService) deletePointsHistory(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM points_histories WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteStorageFiles(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM storage_files WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteStorageUsage(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM storage_usages WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteUserCircles(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM user_circles WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteUserLabels(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM labels WHERE created_by = ?", userID)
}

func (s *DeletionService) deleteUserThings(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM things WHERE user_id = ?", userID)
}

func (s *DeletionService) deleteUserSubtasks(tx *gorm.DB, userID int) (int, error) {
	return s.safeDelete(tx, "DELETE FROM sub_tasks WHERE chore_id IN (SELECT id FROM chores WHERE created_by = ?)", userID)
}

func (s *DeletionService) deleteUserStorageFiles(ctx context.Context, userID int) error {
	// Get all file paths for the user
	var filePaths []string
	err := s.db.Table("storage_files").
		Where("user_id = ?", userID).
		Pluck("file_path", &filePaths).Error

	if err != nil || len(filePaths) == 0 {
		return err
	}

	// Delete files from storage backend
	return s.storage.Delete(ctx, filePaths)
}

// Helper function to handle table not found errors gracefully
func (s *DeletionService) safeDelete(tx *gorm.DB, query string, args ...interface{}) (int, error) {
	result := tx.Exec(query, args...)
	if result.Error != nil && (strings.Contains(result.Error.Error(), "no such table") ||
		strings.Contains(result.Error.Error(), "doesn't exist")) {
		return 0, nil
	}
	return int(result.RowsAffected), result.Error
}

// Helper function to count records for dry run
func (s *DeletionService) safeCount(tx *gorm.DB, query string, args ...interface{}) (int, error) {
	var count int64
	result := tx.Raw(query, args...).Count(&count)
	if result.Error != nil && (strings.Contains(result.Error.Error(), "no such table") ||
		strings.Contains(result.Error.Error(), "doesn't exist")) {
		return 0, nil
	}
	return int(count), result.Error
}

// Count methods for dry run
func (s *DeletionService) countMFASessions(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM mfa_sessions WHERE user_id = ?", userID)
}

func (s *DeletionService) countAPITokens(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM api_tokens WHERE user_id = ?", userID)
}

func (s *DeletionService) countPasswordResetTokens(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM user_password_resets WHERE user_id = ?", userID)
}

func (s *DeletionService) countNotificationTargets(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM user_notification_targets WHERE user_id = ?", userID)
}

func (s *DeletionService) countNotifications(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM notifications WHERE user_id = ?", userID)
}

func (s *DeletionService) countTimeSessions(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM time_sessions WHERE chore_id IN (SELECT id FROM chores WHERE created_by = ?) OR updated_by = ?", userID, userID)
}

func (s *DeletionService) countChoreHistory(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM chore_histories WHERE completed_by = ? OR chore_id IN (SELECT id FROM chores WHERE created_by = ?)", userID, userID)
}

func (s *DeletionService) countChoreAssignees(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM chore_assignees WHERE user_id = ?", userID)
}

func (s *DeletionService) countChoreLabels(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM chore_labels WHERE user_id = ?", userID)
}

func (s *DeletionService) countUserChores(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM chores WHERE created_by = ?", userID)
}

func (s *DeletionService) countPointsHistory(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM points_histories WHERE user_id = ?", userID)
}

func (s *DeletionService) countStorageFiles(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM storage_files WHERE user_id = ?", userID)
}

func (s *DeletionService) countStorageUsage(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM storage_usages WHERE user_id = ?", userID)
}

func (s *DeletionService) countUserCircles(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM user_circles WHERE user_id = ?", userID)
}

func (s *DeletionService) countUserLabels(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM labels WHERE created_by = ?", userID)
}

func (s *DeletionService) countUserThings(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM things WHERE user_id = ?", userID)
}

func (s *DeletionService) countUserSubtasks(tx *gorm.DB, userID int) (int, error) {
	return s.safeCount(tx, "SELECT COUNT(*) FROM sub_tasks WHERE chore_id IN (SELECT id FROM chores WHERE created_by = ?)", userID)
}
