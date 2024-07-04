package bot

import "testing"

func TestConvertLinkToMarkdown(t *testing.T) {
	test := []struct {
		link     string
		expected string
		hasError bool
	}{
		{
			link:     "https://leetcode.com/problems/merge-nodes-in-between-zeros/description/",
			expected: "[merge nodes in between zeros](https://leetcode.com/problems/merge-nodes-in-between-zeros/description/)",
			hasError: false,
		},
		{
			link:     "https://leetcode.com/problems/two-sum/",
			expected: "[two sum](https://leetcode.com/problems/two-sum/)",
			hasError: false,
		},
		{
			link:     "https://leetcode.com/problems/",
			expected: "https://leetcode.com/problems/",
			hasError: true,
		},
		{
			link:     "https://example.com/invalid-link",
			expected: "https://example.com/invalid-link",
			hasError: true,
		},
	}

	for _, tc := range test {
		actual, err := ConvertLinkToMarkdown(tc.link)
		if tc.hasError {
			// TODO
		} else {
			if err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
			if actual != tc.expected {
				t.Errorf("Expected %s but got %s", tc.expected, actual)
			}
		}
	}
}
