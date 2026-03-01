package bot

import (
	"regexp"
	"strings"
)

var branchPattern = regexp.MustCompile(`^[0-9]{2}-[1-4]$`)
var prefixedStudyNamePattern = regexp.MustCompile(`^[0-9]{2}-[1-4]-(.*)$`)

func isValidBranch(branch string) bool {
	return branchPattern.MatchString(strings.TrimSpace(branch))
}

func normalizeStudyName(name string) string {
	trimmed := strings.TrimSpace(name)
	if match := prefixedStudyNamePattern.FindStringSubmatch(trimmed); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return trimmed
}

func buildStudyChannelName(branch, name string) string {
	return branch + "-" + name
}
