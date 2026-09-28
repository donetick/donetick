package repo

import (
	"context"
	"errors"

	config "donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	pModel "donetick.com/core/internal/project/model"
	syncModel "donetick.com/core/internal/sync/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProjectRepository struct {
	db        *gorm.DB
	choreRepo *chRepo.ChoreRepository
}

func NewProjectRepository(db *gorm.DB, cfg *config.Config, choreRepo *chRepo.ChoreRepository) *ProjectRepository {
	return &ProjectRepository{db: db, choreRepo: choreRepo}
}

// bumpChoreSyncVersions assigns a fresh sync version to every chore in choreIDs so
// clients syncing on sync_version pick up the project change. Versions are reserved as a
// contiguous range and applied one per chore, so no two chores share a version (sharing
// one would let sync pagination skip records). Must run inside tx.
func (r *ProjectRepository) bumpChoreSyncVersions(ctx context.Context, tx *gorm.DB, circleID int, choreIDs []int) error {
	if len(choreIDs) < 1 {
		return nil
	}

	var endVersion int64
	if err := tx.WithContext(ctx).Raw(`
		INSERT INTO sync_cursors (circle_id, entity_type, max_version)
		VALUES (?, ?, ?)
		ON CONFLICT (circle_id, entity_type) DO UPDATE SET max_version = sync_cursors.max_version + ?
		RETURNING max_version`,
		circleID, syncModel.EntityTypeChore, len(choreIDs), len(choreIDs),
	).Scan(&endVersion).Error; err != nil {
		return err
	}

	startVersion := endVersion - int64(len(choreIDs)) + 1
	for i, choreID := range choreIDs {
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).Where("id = ?", choreID).
			Update("sync_version", startVersion+int64(i)).Error; err != nil {
			return err
		}
	}
	return nil
}

// choreIDsInProject returns the chores currently assigned to projectID within circleID.
func (r *ProjectRepository) choreIDsInProject(ctx context.Context, tx *gorm.DB, projectID int, circleID int) ([]int, error) {
	var choreIDs []int
	if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
		Where("project_id = ? AND circle_id = ?", projectID, circleID).
		Pluck("id", &choreIDs).Error; err != nil {
		return nil, err
	}
	return choreIDs, nil
}

// GetCircleProjects returns the projects of a circle the user is allowed to see:
// every public one plus the private ones they created.
func (r *ProjectRepository) GetCircleProjects(ctx context.Context, circleID int, userID int) ([]*pModel.Project, error) {
	var projects []*pModel.Project
	if err := r.db.WithContext(ctx).
		Where("circle_id = ? AND (is_private = ? OR created_by = ?)", circleID, false, userID).
		Order("name ASC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *ProjectRepository) GetProjectByID(ctx context.Context, projectID int, circleID int) (*pModel.Project, error) {
	var project pModel.Project
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", projectID, circleID).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *ProjectRepository) CreateProject(ctx context.Context, project *pModel.Project) error {
	if err := r.db.WithContext(ctx).Create(project).Error; err != nil {
		return err
	}
	return nil
}

// UpdateProject updates a project owned by the user. isPrivate is optional: when
// nil the current visibility is kept. Flipping it propagates to every chore in the
// project, since a chore inherits the privacy of the project it belongs to.
func (r *ProjectRepository) UpdateProject(ctx context.Context, project *pModel.Project, isPrivate *bool, userID int, circleID int) error {
	log := logging.FromContext(ctx)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize privacy changes with chore moves that lock their destination
		// project. Reading this inside the transaction also prevents two concurrent
		// project updates from making propagation decisions from stale state.
		var existingProject pModel.Project
		query := tx.WithContext(ctx)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND circle_id = ?", project.ID, circleID).First(&existingProject).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("project not found")
			}
			log.Error("Error finding project", "error", err)
			return err
		}

		if existingProject.CreatedBy != userID {
			if !existingProject.CanView(userID) {
				return errors.New("project not found")
			}
			return errors.New("user does not have permission to update this project")
		}

		destinationPrivacy := existingProject.IsPrivate
		if isPrivate != nil {
			destinationPrivacy = *isPrivate
		}
		updates := map[string]interface{}{
			"name":        project.Name,
			"description": project.Description,
			"color":       project.Color,
			"icon":        project.Icon,
			"is_private":  destinationPrivacy,
		}

		if err := tx.WithContext(ctx).Model(&pModel.Project{}).Where("id = ? AND circle_id = ?", project.ID, circleID).Updates(updates).Error; err != nil {
			log.Error("Error updating project", "error", err)
			return err
		}

		if existingProject.IsPrivate != destinationPrivacy {
			if err := r.choreRepo.SetProjectChoresPrivacy(ctx, tx, circleID, project.ID, existingProject.CreatedBy, destinationPrivacy); err != nil {
				log.Error("Error propagating project privacy to chores", "error", err, "projectID", project.ID)
				return err
			}
			return nil
		}

		// Clients render the project (name/color/icon) alongside its chores, so the
		// chores have to be re-synced even though the chore rows are untouched.
		choreIDs, err := r.choreIDsInProject(ctx, tx, project.ID, circleID)
		if err != nil {
			log.Error("Error getting chores for project", "error", err)
			return err
		}
		return r.bumpChoreSyncVersions(ctx, tx, circleID, choreIDs)
	})
}

func (r *ProjectRepository) DeleteProject(ctx context.Context, projectID int, userID int, circleID int) error {
	log := logging.FromContext(ctx)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize deletion with project privacy changes and chore creation/moves.
		var project pModel.Project
		query := tx.WithContext(ctx)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Where("id = ? AND circle_id = ?", projectID, circleID).First(&project).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("project not found")
			}
			return err
		}
		if project.IsDefault {
			return errors.New("cannot delete default project")
		}
		if project.CreatedBy != userID {
			if !project.CanView(userID) {
				return errors.New("project not found")
			}
			return errors.New("user does not have permission to delete this project")
		}

		choreIDs, err := r.choreIDsInProject(ctx, tx, projectID, circleID)
		if err != nil {
			log.Error("Error getting chores for project", "error", err)
			return err
		}
		if err := tx.WithContext(ctx).Model(&chModel.Chore{}).
			Where("project_id = ? AND circle_id = ?", projectID, circleID).
			Update("project_id", nil).Error; err != nil {
			log.Error("Error updating chores when deleting project", "error", err)
			return err
		}
		if err := r.bumpChoreSyncVersions(ctx, tx, circleID, choreIDs); err != nil {
			log.Error("Error bumping chore sync versions when deleting project", "error", err)
			return err
		}
		if err := tx.Where("id = ? AND circle_id = ?", projectID, circleID).Delete(&pModel.Project{}).Error; err != nil {
			log.Error("Error deleting project", "error", err)
			return err
		}
		return nil
	})
}
