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
	actual := buildStudyChannelName("26-2", "algo")
	if actual != "26-2-algo" {
		t.Fatalf("expected 26-2-algo, got %s", actual)
	}
}
