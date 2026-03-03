package bot

import "testing"

func TestIsValidBranch(t *testing.T) {
	testCases := []struct {
		branch string
		valid  bool
	}{
		{branch: "26-1", valid: true},
		{branch: "26-4", valid: true},
		{branch: "26-0", valid: false},
		{branch: "26-5", valid: false},
		{branch: "2026-1", valid: false},
		{branch: "aa-1", valid: false},
		{branch: "26-a", valid: false},
	}

	for _, tc := range testCases {
		if isValidBranch(tc.branch) != tc.valid {
			t.Fatalf("branch=%q expected valid=%v", tc.branch, tc.valid)
		}
	}
}

func TestNormalizeStudyName(t *testing.T) {
	testCases := []struct {
		name     string
		expected string
	}{
		{name: "26-2-algo", expected: "algo"},
		{name: "26-3- backend", expected: "backend"},
		{name: "algo", expected: "algo"},
		{name: "  algo  ", expected: "algo"},
		{name: "26-4-", expected: ""},
	}

	for _, tc := range testCases {
		if actual := normalizeStudyName(tc.name); actual != tc.expected {
			t.Fatalf("name=%q expected=%q got=%q", tc.name, tc.expected, actual)
		}
	}
}

func TestBuildStudyChannelName(t *testing.T) {
	testCases := []struct {
		branch   string
		name     string
		expected string
	}{
		{branch: "26-2", name: "algo", expected: "26-2-algo"},
		{branch: "26-1", name: "System Design", expected: "26-1-system-design"},
		{branch: "26-3", name: "C++", expected: "26-3-c"},
		{branch: "26-2", name: "네트워크", expected: "26-2-"},
	}

	for _, tc := range testCases {
		actual := buildStudyChannelName(tc.branch, tc.name)
		if actual != tc.expected {
			t.Fatalf("branch=%q name=%q expected=%q got=%q", tc.branch, tc.name, tc.expected, actual)
		}
	}
}

func TestSanitizeChannelName_TruncatesAt100(t *testing.T) {
	long := "26-2-" + string(make([]byte, 200))
	result := sanitizeChannelName(long)
	if len(result) > 100 {
		t.Fatalf("expected max 100 chars, got %d", len(result))
	}
}
