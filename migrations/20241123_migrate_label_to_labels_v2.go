package migrations

import (
	"context"
	"strings"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type MigrateLabels20241123 struct{}

func (m MigrateLabels20241123) ID() string {
	return "20241123_migrate_label_to_labels_v2"
}

func (m MigrateLabels20241123) Description() string {
	return `Migrate label to labels v2 table, Allow more advanced features with labels like assign color`
}

func (m MigrateLabels20241123) Down(ctx context.Context, db *gorm.DB) error {
	return nil
}

func (m MigrateLabels20241123) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	type Label struct {
		ID        int    `gorm:"column:id;primary_key"`
		Name      string `gorm:"column:name"`
		CreatedBy int    `gorm:"column:created_by"`
		CircleID  *int   `gorm:"column:circle_id"`
		ChoresId  []int  `gorm:"-"`
	}

	type Chore struct {
		Labels    *string `gorm:"column:labels"`
		ID        int     `gorm:"column:id;primary_key"`
		CircleID  int     `gorm:"column:circle_id"`
		CreatedBy int     `gorm:"column:created_by"`
	}

	type ChoreLabel struct {
		ChoreID int `gorm:"column:chore_id"`
		LabelID int `gorm:"column:label_id"`
		UserID  int `gorm:"column:user_id"`
	}

	// Start a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// Get all chores with labels
		var choreRecords []Chore
		if err := tx.Table("chores").Select("id, labels, circle_id, created_by").Find(&choreRecords).Error; err != nil {
			log.Errorf("Failed to fetch chores with label: %v", err)
			return err
		}

		// Map to store new labels
		newLabelsMap := make(map[string]Label)
		for _, choreRecord := range choreRecords {
			if choreRecord.Labels != nil {
				labels := strings.Split(*choreRecord.Labels, ",")
				for _, label := range labels {
					label = strings.TrimSpace(label)
					if _, ok := newLabelsMap[label]; !ok {
						newLabelsMap[label] = Label{
							Name:      label,
							CreatedBy: choreRecord.CreatedBy,
							CircleID:  &choreRecord.CircleID,
							ChoresId:  []int{choreRecord.ID},
						}
					} else {
						labelToUpdate := newLabelsMap[label]
						labelToUpdate.ChoresId = append(labelToUpdate.ChoresId, choreRecord.ID)
						newLabelsMap[label] = labelToUpdate
					}
				}
			}
		}

		// Insert new labels and update chore_labels
		for labelName, label := range newLabelsMap {
			// Check if the label already exists
			var existingLabel Label
			if err := tx.Table("labels").Where("name = ? AND created_by = ? AND circle_id = ?", labelName, label.CreatedBy, label.CircleID).First(&existingLabel).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// Insert new label
					if err := tx.Table("labels").Create(&label).Error; err != nil {
						log.Errorf("Failed to insert new label: %v", err)
						return err
					}
					existingLabel = label
				} else {
					log.Errorf("Failed to check existing label: %v", err)
					return err
				}
			}

			// Prepare chore_labels for batch insertion
			var choreLabels []ChoreLabel
			for _, choreId := range label.ChoresId {
				choreLabels = append(choreLabels, ChoreLabel{
					ChoreID: choreId,
					LabelID: existingLabel.ID,
					UserID:  label.CreatedBy,
				})
			}

			// Batch insert chore_labels
			if err := tx.Table("chore_labels").Create(&choreLabels).Error; err != nil {
				log.Errorf("Failed to insert chore labels: %v", err)
				return err
			}
		}

		return nil
	})
}

// Register this migration
func init() {
	Register(MigrateLabels20241123{})
}
