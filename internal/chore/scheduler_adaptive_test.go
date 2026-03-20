package chore

import (
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/stretchr/testify/assert"
)

func TestScheduleAdaptiveNextDueDate_WithNilPerformedAt(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name          string
		chore         *chModel.Chore
		completedDate time.Time
		history       []*chModel.ChoreHistory
		wantNil       bool
		wantErr       bool
	}{
		{
			name: "All history entries have nil PerformedAt",
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequencyTypeAdaptive,
				NextDueDate:   timePtr(now.Add(24 * time.Hour)),
			},
			completedDate: now,
			history: []*chModel.ChoreHistory{
				{PerformedAt: nil},
				{PerformedAt: nil},
				{PerformedAt: nil},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "Mixed nil and valid PerformedAt entries",
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequencyTypeAdaptive,
			},
			completedDate: now,
			history: []*chModel.ChoreHistory{
				{PerformedAt: timePtr(now.Add(-24 * time.Hour))},
				{PerformedAt: timePtr(now.Add(-48 * time.Hour))},
				{PerformedAt: nil},
				{PerformedAt: timePtr(now.Add(-96 * time.Hour))},
			},
			wantNil: false, // Should calculate from valid consecutive entries
			wantErr: false,
		},
		{
			name: "Single history entry with nil PerformedAt",
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequencyTypeAdaptive,
				NextDueDate:   timePtr(now.Add(24 * time.Hour)),
			},
			completedDate: now,
			history: []*chModel.ChoreHistory{
				{PerformedAt: nil},
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "Empty history with NextDueDate",
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequencyTypeAdaptive,
				NextDueDate:   timePtr(now.Add(24 * time.Hour)),
			},
			completedDate: now,
			history:       []*chModel.ChoreHistory{},
			wantNil:       false,
			wantErr:       false,
		},
		{
			name: "Empty history without NextDueDate",
			chore: &chModel.Chore{
				FrequencyType: chModel.FrequencyTypeAdaptive,
				NextDueDate:   nil,
			},
			completedDate: now,
			history:       []*chModel.ChoreHistory{},
			wantNil:       true,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduleAdaptiveNextDueDate(tt.chore, tt.completedDate, tt.history)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}

// Test that the fix doesn't break normal adaptive scheduling
func TestScheduleAdaptiveNextDueDate_NormalCase(t *testing.T) {
	now := time.Now().UTC()

	chore := &chModel.Chore{
		FrequencyType: chModel.FrequencyTypeAdaptive,
	}

	history := []*chModel.ChoreHistory{
		{PerformedAt: timePtr(now.Add(-24 * time.Hour))},
		{PerformedAt: timePtr(now.Add(-48 * time.Hour))},
		{PerformedAt: timePtr(now.Add(-72 * time.Hour))},
	}

	got, err := scheduleAdaptiveNextDueDate(chore, now, history)

	assert.NoError(t, err)
	assert.NotNil(t, got)
	// The next due date should be approximately 24 hours from now
	// (based on the pattern in history)
	assert.InDelta(t, now.Add(24*time.Hour).Unix(), got.Unix(), 3600) // within 1 hour
}
