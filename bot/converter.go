package bot

import (
	"fmt"
	"strings"
)

func ConvertLinkToMarkdown(link string) (string, error) {
	parts := strings.Split(link, "/")
	if len(parts) < 5 {
		return link, fmt.Errorf("Invalid link")
	}
	titlePart := parts[4]
	title := strings.ReplaceAll(titlePart, "-", " ")
	markdown := fmt.Sprintf("[%s](%s)", title, link)
	return markdown, nil
}
