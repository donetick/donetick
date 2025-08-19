package model

import (
	"testing"
)

func TestFrequencyType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ft   FrequencyType
		want bool
	}{
		{
			name: "valid - once",
			ft:   FrequencyTypeOnce,
			want: true,
		},
		{
			name: "valid - daily",
			ft:   FrequencyTypeDaily,
			want: true,
		},
		{
			name: "valid - weekly",
			ft:   FrequencyTypeWeekly,
			want: true,
		},
		{
			name: "valid - monthly",
			ft:   FrequencyTypeMonthly,
			want: true,
		},
		{
			name: "valid - yearly",
			ft:   FrequencyTypeYearly,
			want: true,
		},
		{
			name: "valid - adaptive",
			ft:   FrequencyTypeAdaptive,
			want: true,
		},
		{
			name: "valid - interval",
			ft:   FrequencyTypeInterval,
			want: true,
		},
		{
			name: "valid - days_of_the_week",
			ft:   FrequencyTypeDayOfTheWeek,
			want: true,
		},
		{
			name: "valid - day_of_the_month",
			ft:   FrequencyTypeDayOfTheMonth,
			want: true,
		},
		{
			name: "valid - trigger",
			ft:   FrequencyTypeTrigger,
			want: true,
		},
		{
			name: "valid - no_repeat",
			ft:   FrequencyTypeNoRepeat,
			want: true,
		},
		{
			name: "invalid - empty string",
			ft:   "",
			want: false,
		},
		{
			name: "invalid - random string",
			ft:   "random",
			want: false,
		},
		{
			name: "invalid - DAILY uppercase",
			ft:   "DAILY",
			want: false,
		},
		{
			name: "invalid - Daily with capital",
			ft:   "Daily",
			want: false,
		},
		{
			name: "invalid - one_time (not once)",
			ft:   "one_time",
			want: false,
		},
		{
			name: "invalid - days-of-the-week (with dashes)",
			ft:   "days-of-the-week",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ft.IsValid(); got != tt.want {
				t.Errorf("FrequencyType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}