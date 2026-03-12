package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	suggestionDeadlineLocation = time.FixedZone("Asia/Seoul", 9*60*60)
	errSuggestionDeadlinePast  = errors.New("suggestion deadline must be in the future")
)

func parseSuggestionDeadline(raw string, now time.Time) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), suggestionDeadlineLocation)
	if err != nil {
		return time.Time{}, err
	}

	closesAt := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, suggestionDeadlineLocation)
	if !closesAt.After(now.In(suggestionDeadlineLocation)) {
		return time.Time{}, errSuggestionDeadlinePast
	}

	return closesAt, nil
}

func suggestionDateLabel(t time.Time) string {
	return t.In(suggestionDeadlineLocation).Format("2006-01-02")
}

func buildSuggestionAnnouncement(closesAt time.Time) string {
	return fmt.Sprintf("📣 스터디 제안을 받습니다!\n마감일: %s 까지\n`/suggest` 로 익명 주제를 제안해주세요.", suggestionDateLabel(closesAt))
}

func buildSuggestionMessage(title, description string, voteCount int) string {
	if strings.TrimSpace(description) != "" {
		return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s\n설명: %s\n\n🚀 %d표", title, description, voteCount)
	}

	return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s\n\n🚀 %d표", title, voteCount)
}
