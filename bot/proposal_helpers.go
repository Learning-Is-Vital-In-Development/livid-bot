package bot

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	proposalDeadlineLocation = time.FixedZone("Asia/Seoul", 9*60*60)
	errProposalDeadlinePast  = errors.New("proposal deadline must be in the future")
)

func parseProposalDeadline(raw string, now time.Time) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), proposalDeadlineLocation)
	if err != nil {
		return time.Time{}, err
	}

	closesAt := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, proposalDeadlineLocation)
	if !closesAt.After(now.In(proposalDeadlineLocation)) {
		return time.Time{}, errProposalDeadlinePast
	}

	return closesAt, nil
}

func proposalDateLabel(t time.Time) string {
	return t.In(proposalDeadlineLocation).Format("2006-01-02")
}

func buildProposalAnnouncement(closesAt time.Time) string {
	return fmt.Sprintf("📣 스터디 제안을 받습니다!\n마감일: %s 까지\n/제안 으로 익명 주제를 제안해주세요.", proposalDateLabel(closesAt))
}

func buildProposalMessage(title, description string, voteCount int) string {
	if strings.TrimSpace(description) != "" {
		return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s\n설명: %s\n\n🚀 %d표", title, description, voteCount)
	}

	return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s\n\n🚀 %d표", title, voteCount)
}
