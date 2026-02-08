package repo

import (
	"context"
	"errors"

	config "donetick.com/core/config"
	fModel "donetick.com/core/internal/filter/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type FilterRepository struct {
	db *gorm.DB
}

func NewFilterRepository(db *gorm.DB, cfg *config.Config) *FilterRepository {
	return &FilterRepository{db: db}
}

// GetCircleFilters gets all filters for a circle
func (r *FilterRepository) GetCircleFilters(ctx context.Context, circleID int) ([]*fModel.Filter, error) {
	var filters []*fModel.Filter
	if err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("name ASC").Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// GetFilterByID gets a specific filter by ID
func (r *FilterRepository) GetFilterByID(ctx context.Context, filterID int, circleID int) (*fModel.Filter, error) {
	var filter fModel.Filter
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", filterID, circleID).First(&filter).Error; err != nil {
		return nil, err
	}
	return &filter, nil
}

// CreateFilter creates a new filter
func (r *FilterRepository) CreateFilter(ctx context.Context, filter *fModel.Filter) error {
	if err := r.db.WithContext(ctx).Create(filter).Error; err != nil {
		return err
	}
	return nil
}

// UpdateFilter updates an existing filter
func (r *FilterRepository) UpdateFilter(ctx context.Context, filter *fModel.Filter, userID int, circleID int) error {
	log := logging.FromContext(ctx)

	// Check if user has permission to update this filter
	var existingFilter fModel.Filter
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", filter.ID, circleID).First(&existingFilter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("filter not found")
		}
		log.Error("Error finding filter", "error", err)
		return err
	}

	// Only creator can update filter
	if existingFilter.CreatedBy != userID {
		return errors.New("user does not have permission to update this filter")
	}

	updates := map[string]interface{}{
		"name":        filter.Name,
		"description": filter.Description,
		"color":       filter.Color,
		"icon":        filter.Icon,
		"conditions":  filter.Conditions,
		"operator":    filter.Operator,
		"is_pinned":   filter.IsPinned,
	}

	if err := r.db.WithContext(ctx).Model(&fModel.Filter{}).Where("id = ? AND circle_id = ?", filter.ID, circleID).Updates(updates).Error; err != nil {
		return err
	}
	return nil
}

// DeleteFilter deletes a filter
func (r *FilterRepository) DeleteFilter(ctx context.Context, filterID int, userID int, circleID int) error {
	log := logging.FromContext(ctx)

	// Check if filter exists and user has permission
	var filter fModel.Filter
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", filterID, circleID).First(&filter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("filter not found")
		}
		return err
	}

	// Check if user has permission to delete this filter
	if filter.CreatedBy != userID {
		return errors.New("user does not have permission to delete this filter")
	}

	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", filterID, circleID).Delete(&fModel.Filter{}).Error; err != nil {
		log.Error("Error deleting filter", "error", err)
		return err
	}

	return nil
}

// ToggleFilterPin toggles the pin status of a filter
func (r *FilterRepository) ToggleFilterPin(ctx context.Context, filterID int, userID int, circleID int) (bool, error) {
	log := logging.FromContext(ctx)

	// Check if filter exists and user has permission
	var filter fModel.Filter
	if err := r.db.WithContext(ctx).Where("id = ? AND circle_id = ?", filterID, circleID).First(&filter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errors.New("filter not found")
		}
		return false, err
	}

	// Only creator can toggle pin
	if filter.CreatedBy != userID {
		return false, errors.New("user does not have permission to update this filter")
	}

	newPinStatus := !filter.IsPinned
	if err := r.db.WithContext(ctx).Model(&fModel.Filter{}).Where("id = ? AND circle_id = ?", filterID, circleID).Update("is_pinned", newPinStatus).Error; err != nil {
		log.Error("Error toggling filter pin", "error", err)
		return false, err
	}

	return newPinStatus, nil
}

// GetPinnedFilters gets all pinned filters for a circle
func (r *FilterRepository) GetPinnedFilters(ctx context.Context, circleID int) ([]*fModel.Filter, error) {
	var filters []*fModel.Filter
	if err := r.db.WithContext(ctx).Where("circle_id = ? AND is_pinned = ?", circleID, true).Order("name ASC").Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// GetFiltersByUsage gets filters sorted by usage count
func (r *FilterRepository) GetFiltersByUsage(ctx context.Context, circleID int) ([]*fModel.Filter, error) {
	var filters []*fModel.Filter
	if err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("usage_count DESC, name ASC").Find(&filters).Error; err != nil {
		return nil, err
	}
	return filters, nil
}

// FilterNameExists checks if a filter name already exists (case-insensitive)
func (r *FilterRepository) FilterNameExists(ctx context.Context, name string, circleID int, excludeFilterID *int) (bool, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&fModel.Filter{}).Where("LOWER(name) = LOWER(?) AND circle_id = ?", name, circleID)

	if excludeFilterID != nil {
		query = query.Where("id != ?", *excludeFilterID)
	}

	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
