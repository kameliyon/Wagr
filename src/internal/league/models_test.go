package league

import (
	"encoding/json"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// TestPayoutEntry_JSONRoundTrip verifies the new BonusType and Criteria fields
// serialize and deserialize correctly.
func TestPayoutEntry_JSONRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		entry PayoutEntry
	}{
		{
			name: "placement entry unchanged",
			entry: PayoutEntry{
				Type:        "placement",
				Label:       "1st Place",
				Place:       1,
				AmountCents: 5000,
			},
		},
		{
			name: "weekly_high_score bonus",
			entry: PayoutEntry{
				Type:        "weekly",
				BonusType:   "weekly_high_score",
				Label:       "Weekly High Score",
				AmountCents: 1000,
				Weeks:       14,
			},
		},
		{
			name: "score_threshold bonus with criteria",
			entry: PayoutEntry{
				Type:        "weekly",
				BonusType:   "score_threshold",
				Label:       "Score Threshold",
				AmountCents: 500,
				Weeks:       14,
				Criteria:    &BonusCriteria{Threshold: ptr(150.0)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.entry)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}

			var got PayoutEntry
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}

			if got.Type != tt.entry.Type {
				t.Errorf("Type: got %q, want %q", got.Type, tt.entry.Type)
			}
			if got.BonusType != tt.entry.BonusType {
				t.Errorf("BonusType: got %q, want %q", got.BonusType, tt.entry.BonusType)
			}
			if got.Label != tt.entry.Label {
				t.Errorf("Label: got %q, want %q", got.Label, tt.entry.Label)
			}
			if got.AmountCents != tt.entry.AmountCents {
				t.Errorf("AmountCents: got %d, want %d", got.AmountCents, tt.entry.AmountCents)
			}
			if got.Weeks != tt.entry.Weeks {
				t.Errorf("Weeks: got %d, want %d", got.Weeks, tt.entry.Weeks)
			}

			if tt.entry.Criteria == nil {
				if got.Criteria != nil {
					t.Errorf("Criteria: expected nil, got %+v", got.Criteria)
				}
			} else {
				if got.Criteria == nil {
					t.Fatal("Criteria: expected non-nil, got nil")
				}
				if tt.entry.Criteria.Threshold == nil {
					if got.Criteria.Threshold != nil {
						t.Errorf("Criteria.Threshold: expected nil, got %v", *got.Criteria.Threshold)
					}
				} else {
					if got.Criteria.Threshold == nil {
						t.Fatal("Criteria.Threshold: expected non-nil, got nil")
					}
					if *got.Criteria.Threshold != *tt.entry.Criteria.Threshold {
						t.Errorf("Criteria.Threshold: got %v, want %v", *got.Criteria.Threshold, *tt.entry.Criteria.Threshold)
					}
				}
			}
		})
	}
}

// TestPayoutEntry_BackwardCompat verifies that old JSON records without
// bonus_type or criteria unmarshal without error and produce zero values.
func TestPayoutEntry_BackwardCompat(t *testing.T) {
	old := `{"type":"weekly","label":"Weekly High Score","amount_cents":1000,"weeks":14}`

	var entry PayoutEntry
	if err := json.Unmarshal([]byte(old), &entry); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if entry.BonusType != "" {
		t.Errorf("expected empty BonusType for old record, got %q", entry.BonusType)
	}
	if entry.Criteria != nil {
		t.Errorf("expected nil Criteria for old record, got %+v", entry.Criteria)
	}
	if entry.Label != "Weekly High Score" {
		t.Errorf("Label: got %q, want %q", entry.Label, "Weekly High Score")
	}
}

// TestPayoutEntry_CriteriaOmitEmpty verifies that Criteria and BonusType are
// omitted from JSON when empty/nil.
func TestPayoutEntry_CriteriaOmitEmpty(t *testing.T) {
	entry := PayoutEntry{
		Type:        "placement",
		Label:       "1st Place",
		Place:       1,
		AmountCents: 5000,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal to map failed: %v", err)
	}

	if _, ok := m["bonus_type"]; ok {
		t.Error("bonus_type should be omitted when empty")
	}
	if _, ok := m["criteria"]; ok {
		t.Error("criteria should be omitted when nil")
	}
}

// TestBonusCriteria_ThresholdOmitEmpty verifies Threshold is omitted when nil.
func TestBonusCriteria_ThresholdOmitEmpty(t *testing.T) {
	c := BonusCriteria{}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if _, ok := m["threshold"]; ok {
		t.Error("threshold should be omitted when nil")
	}
}
