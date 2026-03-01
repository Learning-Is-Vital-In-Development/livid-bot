package bot

import (
	"fmt"
	"net/url"
	"strings"
)

func ConvertLinkToMarkdown(link string) (string, error) {
	parsed, err := url.Parse(link)
	if err != nil {
		return link, fmt.Errorf("invalid link: %w", err)
	}

	host := strings.ToLower(parsed.Host)
	if host != "leetcode.com" && host != "www.leetcode.com" {
		return link, fmt.Errorf("invalid link: unsupported host")
	}

	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] != "problems" || pathParts[1] == "" {
		return link, fmt.Errorf("invalid link: unsupported path")
	}

	titlePart := pathParts[1]
	title := strings.ReplaceAll(titlePart, "-", " ")
	markdown := fmt.Sprintf("[%s](%s)", title, link)
	return markdown, nil
}
