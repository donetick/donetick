package repo

import (
	"context"
	"errors"

	config "donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	pModel "donetick.com/core/internal/project/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	db        *gorm.DB
	choreRepo *chRepo.ChoreRepository
}

func NewProjectRepository(db *gorm.DB, cfg *config.Config, choreRepo *chRepo.ChoreRepository) *ProjectRepository {
	return &ProjectRepository{db: db, choreRepo: choreRepo}
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

	// Check if user has permission to update this project
	var existingProject pModel.Project
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", project.ID, circleID).First(&existingProject).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		log.Error("Error finding project", "error", err)
		return err
	}

	// Only creator or admin can update project (implement admin check based on your auth system)
	if existingProject.CreatedBy != userID {
		return errors.New("user does not have permission to update this project")
	}

	if isPrivate == nil {
		isPrivate = &existingProject.IsPrivate
	}

	updates := map[string]interface{}{
		"name":        project.Name,
		"description": project.Description,
		"color":       project.Color,
		"icon":        project.Icon,
		"is_private":  *isPrivate,
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&pModel.Project{}).Where("id = ? AND circle_id = ?", project.ID, circleID).Updates(updates).Error; err != nil {
			log.Error("Error updating project", "error", err)
			return err
		}

		if existingProject.IsPrivate == *isPrivate {
			return nil
		}

		if err := r.choreRepo.SetProjectChoresPrivacy(ctx, tx, circleID, project.ID, existingProject.CreatedBy, *isPrivate); err != nil {
			log.Error("Error propagating project privacy to chores", "error", err, "projectID", project.ID)
			return err
		}
		return nil
	})
}

func (r *ProjectRepository) DeleteProject(ctx context.Context, projectID int, userID int, circleID int) error {
	log := logging.FromContext(ctx)

	// Check if it's the default project
	var project pModel.Project
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", projectID, circleID).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("project not found")
		}
		return err
	}

	if project.IsDefault {
		return errors.New("cannot delete default project")
	}

	// Check if user has permission to delete this project
	if project.CreatedBy != userID {
		return errors.New("user does not have permission to delete this project")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First, update all chores in this project to have no project (project_id = NULL)
		if err := tx.Exec("UPDATE chores SET project_id = NULL WHERE project_id = ?", projectID).Error; err != nil {
			log.Error("Error updating chores when deleting project", "error", err)
			return err
		}

		// Then delete the project
		if err := tx.Where("id = ? AND circle_id = ?", projectID, circleID).Delete(&pModel.Project{}).Error; err != nil {
			log.Error("Error deleting project", "error", err)
			return err
		}

		return nil
	})
}
