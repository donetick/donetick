package chore

import (
	"testing"

	chModel "donetick.com/core/internal/chore/model"
	"github.com/go-playground/validator/v10"
)

func validateBoundsPayload(t *testing.T, freqType chModel.FrequencyType, bounds *chModel.ScheduleBounds) error {
	t.Helper()

	v := validator.New()
	v.RegisterStructValidation(ChoreReqStructLevelValidation, ChoreReq{})
	frequency := 1
	return v.Struct(ChoreReq{
		Name:              "chore",
		FrequencyType:     freqType,
		Frequency:         &frequency,
		AssignStrategy:    chModel.AssignmentStrategyKeepLastAssigned,
		FrequencyMetadata: &chModel.FrequencyMetadata{ScheduleBounds: bounds},
	})
}

func TestValidateRecurrenceBounds(t *testing.T) {
	tests := []struct {
		name     string
		freqType chModel.FrequencyType
		bounds   *chModel.ScheduleBounds
		wantErr  bool
	}{
		{
			name:   "valid starts with a completions stop",
			bounds: &chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15"), StopRepeating: &chModel.StopRepeating{Type: chModel.StopCompletions, CompletionLimit: intPtr(20)}},
		},
		{
			name:   "valid seasonal with no endpoints",
			bounds: &chModel.ScheduleBounds{ActivePeriod: seasonal("", "")},
		},
		{
			name:   "seasonal February 29",
			bounds: &chModel.ScheduleBounds{ActivePeriod: seasonal("02-29", "03-01")},
		},
		{
			name:   "incomplete selections are permitted",
			bounds: &chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopCompletions}},
		},
		{
			name:    "unknown period type",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: &chModel.ActivePeriod{Type: "someday"}},
			wantErr: true,
		},
		{
			name:    "unknown stop type",
			bounds:  &chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: "maybe"}},
			wantErr: true,
		},
		{
			name:    "always carrying a start",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: &chModel.ActivePeriod{Type: chModel.PeriodAlways, Start: "2026-03-15"}},
			wantErr: true,
		},
		{
			name:    "starts carrying an end",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: &chModel.ActivePeriod{Type: chModel.PeriodStarts, Start: "2026-03-15", End: "2026-05-01"}},
			wantErr: true,
		},
		{
			name:    "starts with a seasonal endpoint",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: starts("03-15")},
			wantErr: true,
		},
		{
			name:    "seasonal with a full date",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: seasonal("2026-06-01", "")},
			wantErr: true,
		},
		{
			name:    "malformed date",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: starts("2026-02-30")},
			wantErr: true,
		},
		{
			name:    "zero completion limit",
			bounds:  &chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopCompletions, CompletionLimit: intPtr(0)}},
			wantErr: true,
		},
		{
			name:    "never carrying a limit",
			bounds:  &chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopNever, CompletionLimit: intPtr(5)}},
			wantErr: true,
		},
		{
			name:    "date stop carrying a limit",
			bounds:  &chModel.ScheduleBounds{StopRepeating: &chModel.StopRepeating{Type: chModel.StopDate, Date: "2026-05-01", CompletionLimit: intPtr(5)}},
			wantErr: true,
		},
		{
			name:    "stop date before an absolute start",
			bounds:  &chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15"), StopRepeating: stopOn("2026-03-14")},
			wantErr: true,
		},
		{
			name:   "stop date on an absolute start",
			bounds: &chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15"), StopRepeating: stopOn("2026-03-15")},
		},
		{
			name:   "stop date before a seasonal start, because a season recurs",
			bounds: &chModel.ScheduleBounds{ActivePeriod: seasonal("06-01", "08-31"), StopRepeating: stopOn("2026-03-01")},
		},
		{
			name:     "bounds on a non-recurring type",
			freqType: chModel.FrequencyTypeOnce,
			bounds:   &chModel.ScheduleBounds{ActivePeriod: starts("2026-03-15")},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freqType := tt.freqType
			if freqType == "" {
				freqType = chModel.FrequencyTypeDaily
			}
			err := validateBoundsPayload(t, freqType, tt.bounds)
			if (err != nil) != tt.wantErr {
				t.Errorf("validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
