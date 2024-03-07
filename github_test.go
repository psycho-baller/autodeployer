package main

import (
	"testing"
)

func TestGetNewTag(t *testing.T) {
	testCases := []struct {
		oldTag            string
		versionChangeType VersionChangeType
		expected          string
	}{
		{"1.0.1", Minor, "1.0.2-rc1"},
		{"1.0.1-rc1", Minor, "1.0.1-rc2"},
	}

	for _, tc := range testCases {
		actual, err := getNewTag(tc.oldTag, tc.versionChangeType)
		if err != nil {
			t.Errorf("Error returned from getNewTag: %v", err)
		}

		if actual != tc.expected {
			t.Errorf("Expected %s but got %s", tc.expected, actual)
		}
	}
}