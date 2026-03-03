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
	return sanitizeChannelName(branch + "-" + name)
}

func sanitizeChannelName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")

	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}

	result := b.String()
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}
