package migrations

import (
	"context"
	"encoding/json"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

// MigrateUnifyFrequencyToRRule20260621 converts the legacy fragmented frequency
// types (interval / days_of_the_week / day_of_the_month) into the unified
// RRULE-equivalent model: FrequencyType becomes the RRULE FREQ
// (hourly/daily/weekly/monthly/yearly), the `frequency` column becomes the RRULE
// INTERVAL, and frequency_meta_v2 carries setPos/monthDays/dayToken selectors.
//
// daily/weekly/monthly/yearly and the special types (once/trigger/no_repeat/
// adaptive) are already valid and are left untouched.
type MigrateUnifyFrequencyToRRule20260621 struct{}

func (m MigrateUnifyFrequencyToRRule20260621) ID() string {
	return "20260621_unify_frequency_to_rrule"
}

func (m MigrateUnifyFrequencyToRRule20260621) Description() string {
	return `Unify legacy interval/days_of_the_week/day_of_the_month frequency types into the RRULE-equivalent model.`
}

type unifyChoreRow struct {
	ID                  int     `gorm:"column:id;primary_key"`
	FrequencyType       string  `gorm:"column:frequency_type"`
	Frequency           int     `gorm:"column:frequency"`
	FrequencyMetadataV2 *string `gorm:"column:frequency_meta_v2"`
}

func (unifyChoreRow) TableName() string { return "chores" }

// Down is intentionally a no-op: the unified model is the new canonical form and
// the legacy fragmented representation cannot be reconstructed without loss.
func (m MigrateUnifyFrequencyToRRule20260621) Down(ctx context.Context, db *gorm.DB) error {
	logging.FromContext(ctx).Info("20260621_unify_frequency_to_rrule is not automatically reversible; skipping Down")
	return nil
}

func (m MigrateUnifyFrequencyToRRule20260621) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	if !db.Migrator().HasColumn(&unifyChoreRow{}, "frequency_meta_v2") {
		log.Info("Column frequency_meta_v2 does not exist, skipping migration")
		return nil
	}

	var chores []unifyChoreRow
	if err := db.Table("chores").Select("id, frequency_type, frequency, frequency_meta_v2").Find(&chores).Error; err != nil {
		log.Errorf("Failed to fetch chores: %v", err)
		return err
	}

	for _, chore := range chores {
		newType, newFreq, meta, changed := convertLegacyFrequency(chore)
		if !changed {
			continue
		}

		metaJSON, err := json.Marshal(meta)
		if err != nil {
			log.Warnf("Chore %d: failed to marshal frequency_meta_v2: %v", chore.ID, err)
			continue
		}

		updates := map[string]interface{}{
			"frequency_type":    newType,
			"frequency":         newFreq,
			"frequency_meta_v2": string(metaJSON),
		}
		if err := db.Table("chores").Where("id = ?", chore.ID).Updates(updates).Error; err != nil {
			log.Warnf("Chore %d: failed to update frequency: %v", chore.ID, err)
			continue
		}
		log.Infof("Chore %d: migrated %q -> %q (interval=%d)", chore.ID, chore.FrequencyType, newType, newFreq)
	}
	return nil
}

// convertLegacyFrequency maps a legacy chore row to its unified (type, interval,
// metadata). The returned bool is false when no conversion is needed.
func convertLegacyFrequency(chore unifyChoreRow) (string, int, map[string]interface{}, bool) {
	meta := map[string]interface{}{}
	if chore.FrequencyMetadataV2 != nil && *chore.FrequencyMetadataV2 != "" {
		_ = json.Unmarshal([]byte(*chore.FrequencyMetadataV2), &meta)
	}

	switch chore.FrequencyType {
	case "interval":
		unit, _ := meta["unit"].(string)
		delete(meta, "unit")
		newType := "daily"
		switch unit {
		case "hours":
			newType = "hourly"
		case "days":
			newType = "daily"
		case "weeks":
			newType = "weekly"
		case "months":
			newType = "monthly"
		case "years":
			newType = "yearly"
		}
		freq := chore.Frequency
		if freq < 1 {
			freq = 1
		}
		return newType, freq, meta, true

	case "days_of_the_week":
		pattern, _ := meta["weekPattern"].(string)
		setPos := readOccurrences(meta)
		delete(meta, "weekPattern")
		delete(meta, "occurrences")
		delete(meta, "weekNumbers")

		switch pattern {
		case "week_of_month":
			meta["setPos"] = setPos
			meta["dayToken"] = "specific"
			return "monthly", 1, meta, true
		case "week_of_quarter":
			// Approximate: quarter occurrence -> every 3 months. Verify these rows.
			meta["setPos"] = setPos
			meta["dayToken"] = "specific"
			return "monthly", 3, meta, true
		default: // every_week or unset
			return "weekly", 1, meta, true
		}

	case "day_of_the_month":
		// Legacy stored the day number in the `frequency` column.
		day := chore.Frequency
		if day >= 1 && day <= 31 {
			meta["monthDays"] = []int{day}
		}
		if isAllMonths(meta["months"]) {
			delete(meta, "months")
			return "monthly", 1, meta, true
		}
		return "yearly", 1, meta, true
	}

	// daily/weekly/monthly/yearly and special types are already canonical.
	return chore.FrequencyType, chore.Frequency, meta, false
}

// readOccurrences extracts ordinal positions from the legacy `occurrences` (or
// `weekNumbers`) JSON field as a []int (e.g. -1 = last).
func readOccurrences(meta map[string]interface{}) []int {
	out := []int{}
	for _, key := range []string{"occurrences", "weekNumbers"} {
		if raw, ok := meta[key].([]interface{}); ok && len(raw) > 0 {
			for _, v := range raw {
				if f, ok := v.(float64); ok {
					out = append(out, int(f))
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return out
}

// isAllMonths reports whether a legacy `months` value covers all 12 months.
func isAllMonths(v interface{}) bool {
	arr, ok := v.([]interface{})
	if !ok {
		return false
	}
	return len(arr) >= 12
}

func init() {
	Register(MigrateUnifyFrequencyToRRule20260621{})
}
