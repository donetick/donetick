package migrations

import (
	"encoding/json"
	"reflect"
	"testing"
)

func metaJSON(t *testing.T, m map[string]interface{}) *string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	return &s
}

func TestConvertLegacyFrequency(t *testing.T) {
	tests := []struct {
		name      string
		row       unifyChoreRow
		wantType  string
		wantFreq  int
		wantChng  bool
		checkMeta func(t *testing.T, m map[string]interface{})
	}{
		{
			name:     "interval hours -> hourly",
			row:      unifyChoreRow{FrequencyType: "interval", Frequency: 6, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"unit": "hours", "time": "x"})},
			wantType: "hourly", wantFreq: 6, wantChng: true,
			checkMeta: func(t *testing.T, m map[string]interface{}) {
				if _, ok := m["unit"]; ok {
					t.Error("unit should be removed")
				}
			},
		},
		{
			name:     "interval weeks -> weekly",
			row:      unifyChoreRow{FrequencyType: "interval", Frequency: 4, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"unit": "weeks"})},
			wantType: "weekly", wantFreq: 4, wantChng: true,
		},
		{
			name:     "days_of_the_week every_week -> weekly",
			row:      unifyChoreRow{FrequencyType: "days_of_the_week", Frequency: 1, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"days": []string{"monday"}, "weekPattern": "every_week"})},
			wantType: "weekly", wantFreq: 1, wantChng: true,
		},
		{
			name:     "days_of_the_week week_of_month -> monthly + setPos",
			row:      unifyChoreRow{FrequencyType: "days_of_the_week", Frequency: 1, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"days": []string{"sunday"}, "weekPattern": "week_of_month", "occurrences": []int{1}})},
			wantType: "monthly", wantFreq: 1, wantChng: true,
			checkMeta: func(t *testing.T, m map[string]interface{}) {
				if !reflect.DeepEqual(toIntSlice(m["setPos"]), []int{1}) {
					t.Errorf("setPos = %v, want [1]", m["setPos"])
				}
				if m["dayToken"] != "specific" {
					t.Errorf("dayToken = %v, want specific", m["dayToken"])
				}
				if _, ok := m["weekPattern"]; ok {
					t.Error("weekPattern should be removed")
				}
			},
		},
		{
			name:     "days_of_the_week week_of_quarter -> monthly interval 3",
			row:      unifyChoreRow{FrequencyType: "days_of_the_week", Frequency: 1, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"days": []string{"friday"}, "weekPattern": "week_of_quarter", "occurrences": []int{1}})},
			wantType: "monthly", wantFreq: 3, wantChng: true,
		},
		{
			name:     "day_of_the_month all months -> monthly",
			row:      unifyChoreRow{FrequencyType: "day_of_the_month", Frequency: 15, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"months": []string{"january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"}})},
			wantType: "monthly", wantFreq: 1, wantChng: true,
			checkMeta: func(t *testing.T, m map[string]interface{}) {
				if !reflect.DeepEqual(toIntSlice(m["monthDays"]), []int{15}) {
					t.Errorf("monthDays = %v, want [15]", m["monthDays"])
				}
				if _, ok := m["months"]; ok {
					t.Error("months should be removed for all-months case")
				}
			},
		},
		{
			name:     "day_of_the_month subset -> yearly",
			row:      unifyChoreRow{FrequencyType: "day_of_the_month", Frequency: 15, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"months": []string{"january"}})},
			wantType: "yearly", wantFreq: 1, wantChng: true,
			checkMeta: func(t *testing.T, m map[string]interface{}) {
				if !reflect.DeepEqual(toIntSlice(m["monthDays"]), []int{15}) {
					t.Errorf("monthDays = %v, want [15]", m["monthDays"])
				}
			},
		},
		{
			name:     "daily unchanged",
			row:      unifyChoreRow{FrequencyType: "daily", Frequency: 1},
			wantType: "daily", wantFreq: 1, wantChng: false,
		},
		{
			name:     "monthly unchanged",
			row:      unifyChoreRow{FrequencyType: "monthly", Frequency: 1, FrequencyMetadataV2: metaJSON(t, map[string]interface{}{"monthDays": []int{5}})},
			wantType: "monthly", wantFreq: 1, wantChng: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotFreq, meta, changed := convertLegacyFrequency(tt.row)
			if gotType != tt.wantType || gotFreq != tt.wantFreq || changed != tt.wantChng {
				t.Fatalf("convertLegacyFrequency() = (%q, %d, %v), want (%q, %d, %v)",
					gotType, gotFreq, changed, tt.wantType, tt.wantFreq, tt.wantChng)
			}
			if tt.checkMeta != nil {
				tt.checkMeta(t, meta)
			}
		})
	}
}

// toIntSlice normalizes a JSON-decoded numeric slice (which may be []interface{}
// of float64, or []int) into []int for comparison.
func toIntSlice(v interface{}) []int {
	switch s := v.(type) {
	case []int:
		return s
	case []interface{}:
		out := make([]int, 0, len(s))
		for _, e := range s {
			if f, ok := e.(float64); ok {
				out = append(out, int(f))
			}
		}
		return out
	}
	return nil
}
