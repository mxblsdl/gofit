package models

import (
	"encoding/json"
	"testing"
)

func TestHeartRateDataParsing(t *testing.T) {
	// Test JSON data
	testJSON := `{
        "activities-heart": [{
            "dateTime": "2023-09-21",
            "value": {
                "customHeartRateZones": [{
                    "caloriesOut": 100.5,
                    "max": 100,
                    "min": 50,
                    "minutes": 30,
                    "name": "Custom Zone"
                }],
                "heartRateZones": [{
                    "caloriesOut": 200.5,
                    "max": 160,
                    "min": 100,
                    "minutes": 45,
                    "name": "Fat Burn"
                }],
                "restingHeartRate": 65
            }
        }]
    }`

	var heartData ActivitiesHeartList
	err := json.Unmarshal([]byte(testJSON), &heartData)
	if err != nil {
		t.Fatalf("Failed to parse heart rate JSON: %v", err)
	}

	// Verify parsed data
	if len(heartData.ActivitiesHeart) != 1 {
		t.Error("Expected 1 heart rate activity")
	}

	activity := heartData.ActivitiesHeart[0]
	if activity.DateTime != "2023-09-21" {
		t.Errorf("Expected date 2023-09-21, got %s", activity.DateTime)
	}

	if activity.Value.RestingHeartRate != 65 {
		t.Errorf("Expected resting heart rate 65, got %d", activity.Value.RestingHeartRate)
	}
}
