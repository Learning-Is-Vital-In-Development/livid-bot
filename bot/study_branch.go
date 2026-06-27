package bot

import (
	"regexp"
	"strings"
	"unicode"
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

	const maxChannelNameRunes = 100
	var b strings.Builder
	runeCount := 0
	for _, r := range name {
		switch {
		case unicode.IsSpace(r):
			r = '-'
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			// Keep Discord-friendly letters and numbers from any script.
		default:
			continue
		}

		if runeCount >= maxChannelNameRunes {
			break
		}
		b.WriteRune(r)
		runeCount++
	}

	return b.String()
}
