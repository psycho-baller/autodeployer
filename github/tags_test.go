package gh

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

func TestFindAndReplaceTag(t *testing.T) {
    testCases := []struct {
        contentStr string
        oldTag     string
        newTag     string
        expected   string
    }{
        {"This is a test string with tag 1.2.3", "1.2.3", "2.0.0", "This is a test string with tag 2.0.0"},
        {"This is a test string with tag 1.2.2", "1.2.3", "1.2.4-rc1", "This is a test string with tag 1.2.4-rc1"},
		{"This is a test string with tag 1.2.3", "1.3.0", "1.3.0-rc1", "This is a test string with tag 1.3.0-rc1"},
        {"This is a test string with tag 1.2.2", "1.2.3-rc1", "1.2.4-rc2", "This is a test string with tag 1.2.4-rc2"},
        {"This is a test string with tag 1.2.3-rc1", "1.2.3-rc2", "1.2.4-rc3", "This is a test string with tag 1.2.4-rc3"},
		// {"This is a test string with tag 1.2.3", "2.0.0", "2.0.1-rc1", "This is a test string with tag 2.0.1-rc1"},
    }

    for _, tc := range testCases {
        actual := findAndReplaceTag(tc.contentStr, tc.oldTag, tc.newTag)
        if actual != tc.expected {
            t.Errorf("Expected '%s' but got '%s'", tc.expected, actual)
        }
    }
}