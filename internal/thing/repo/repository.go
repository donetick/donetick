package chore

import (
	"context"
	"time"

	config "donetick.com/core/config"
	tModel "donetick.com/core/internal/thing/model"
	"gorm.io/gorm"
)

type ThingRepository struct {
	db     *gorm.DB
	dbType string
}

func NewThingRepository(db *gorm.DB, cfg *config.Config) *ThingRepository {
	return &ThingRepository{db: db, dbType: cfg.Database.Type}
}

func (r *ThingRepository) UpsertThing(c context.Context, thing *tModel.Thing) error {
	return r.db.WithContext(c).Model(&thing).Save(thing).Error
}

func (r *ThingRepository) UpdateThingState(c context.Context, thing *tModel.Thing) error {
	// update the state of the thing where the id is the same:
	if err := r.db.WithContext(c).Model(&thing).Where("id = ?", thing.ID).Updates(map[string]interface{}{
		"state":      thing.State,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return err
	}
	// Create history Record of the thing :
	createdAt := time.Now().UTC()
	thingHistory := &tModel.ThingHistory{
		ThingID:   thing.ID,
		State:     thing.State,
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
	}

	if err := r.db.WithContext(c).Create(thingHistory).Error; err != nil {
		return err
	}

	return nil
}
func (r *ThingRepository) GetThingByID(c context.Context, thingID int) (*tModel.Thing, error) {
	var thing tModel.Thing
	if err := r.db.WithContext(c).Model(&tModel.Thing{}).Preload("ThingChores").First(&thing, thingID).Error; err != nil {
		return nil, err
	}
	return &thing, nil
}

func (r *ThingRepository) GetThingByChoreID(c context.Context, choreID int) (*tModel.Thing, error) {
	var thing tModel.Thing
	if err := r.db.WithContext(c).Model(&tModel.Thing{}).Joins("left join thing_chores on things.id = thing_chores.thing_id").First(&thing, "thing_chores.chore_id = ?", choreID).Error; err != nil {
		return nil, err
	}
	return &thing, nil
}

func (r *ThingRepository) AssociateThingWithChore(c context.Context, thingID int, choreID int, triggerState string, condition string) error {

	return r.db.WithContext(c).Save(&tModel.ThingChore{ThingID: thingID, ChoreID: choreID, TriggerState: triggerState, Condition: condition}).Error
}

func (r *ThingRepository) DissociateThingWithChore(c context.Context, thingID int, choreID int) error {
	return r.db.WithContext(c).Where("thing_id = ? AND chore_id = ?", thingID, choreID).Delete(&tModel.ThingChore{}).Error
}

func (r *ThingRepository) DissociateChoreWithThing(c context.Context, choreID int) error {
	return r.db.WithContext(c).Where("chore_id = ?", choreID).Delete(&tModel.ThingChore{}).Error
}

func (r *ThingRepository) GetThingHistoryWithOffset(c context.Context, thingID int, offset int) ([]*tModel.ThingHistory, error) {
	var thingHistory []*tModel.ThingHistory
	if err := r.db.WithContext(c).Model(&tModel.ThingHistory{}).Where("thing_id = ?", thingID).Order("created_at desc").Offset(offset).Limit(10).Find(&thingHistory).Error; err != nil {
		return nil, err
	}
	return thingHistory, nil
}

func (r *ThingRepository) GetUserThings(c context.Context, userID int) ([]*tModel.Thing, error) {
	var things []*tModel.Thing
	if err := r.db.WithContext(c).Model(&tModel.Thing{}).Where("user_id = ?", userID).Find(&things).Error; err != nil {
		return nil, err
	}
	return things, nil
}

func (r *ThingRepository) DeleteThing(c context.Context, thingID int) error {
	//  one transaction to delete the thing and its history :
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := r.db.WithContext(c).Where("thing_id = ?", thingID).Delete(&tModel.ThingHistory{}).Error; err != nil {
			return err
		}
		if err := r.db.WithContext(c).Delete(&tModel.Thing{}, thingID).Error; err != nil {
			return err
		}
		return nil
	})
}

// get ThingChores by thingID:
func (r *ThingRepository) GetThingChoresByThingId(c context.Context, thingID int) ([]*tModel.ThingChore, error) {
	var thingChores []*tModel.ThingChore
	if err := r.db.WithContext(c).Model(&tModel.ThingChore{}).Where("thing_id = ?", thingID).Find(&thingChores).Error; err != nil {
		return nil, err
	}
	return thingChores, nil
}

func (r *ThingRepository) GetThingsByUserID(c context.Context, userID int) ([]*tModel.Thing, error) {
	var things []*tModel.Thing
	if err := r.db.WithContext(c).Model(&tModel.Thing{}).Where("user_id = ?", userID).Find(&things).Error; err != nil {
		return nil, err
	}
	return things, nil
}

// func (r *ThingRepository) GetChoresByThingId(c context.Context, thingID int) ([]*chModel.Chore, error) {
// 	var chores []*chModel.Chore
// 	if err := r.db.WithContext(c).Model(&chModel.Chore{}).Joins("left join thing_chores on chores.id = thing_chores.chore_id").Where("thing_chores.thing_id = ?", thingID).Find(&chores).Error; err != nil {
// 		return nil, err
// 	}
// 	return chores, nil
// }
