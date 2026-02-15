package repo

import (
	"context"
	"errors"

	config "donetick.com/core/config"
	pModel "donetick.com/core/internal/project/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB, cfg *config.Config) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) GetCircleProjects(ctx context.Context, circleID int) ([]*pModel.Project, error) {
	var projects []*pModel.Project
	if err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("name ASC").Find(&projects).Error; err != nil {
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

func (r *ProjectRepository) UpdateProject(ctx context.Context, project *pModel.Project, userID int, circleID int) error {
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

	updates := map[string]interface{}{
		"name":        project.Name,
		"description": project.Description,
		"color":       project.Color,
		"icon":        project.Icon,
	}

	if err := r.db.WithContext(ctx).Model(&pModel.Project{}).Where("id = ? AND circle_id = ?", project.ID, circleID).Updates(updates).Error; err != nil {
		return err
	}
	return nil
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
