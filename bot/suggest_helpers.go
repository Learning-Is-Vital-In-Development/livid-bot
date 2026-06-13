package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	suggestionDeadlineLocation = time.FixedZone("Asia/Seoul", 9*60*60)
	errSuggestionDeadlinePast  = errors.New("suggestion deadline must be in the future")
)

const (
	suggestionDiscussionChannelName  = "신규-스터디-논의"
	suggestionAnnouncementThreadName = "스터디 제안 안내"
	suggestionDefaultThreadName      = "익명 스터디 제안"
	suggestionThreadNameLimit        = 100
)

type suggestionDiscordClient interface {
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
	ForumThreadStartComplex(channelID string, threadData *discordgo.ThreadStart, messageData *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Channel, error)
}

type suggestionMessageRef struct {
	ChannelID string
	MessageID string
}

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
	_ = voteCount
	if strings.TrimSpace(description) != "" {
		return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s\n설명: %s", title, description)
	}

	return fmt.Sprintf("📬 익명 스터디 제안\n\n**주제**: %s", title)
}

func findSuggestionDiscussionChannel(channels []*discordgo.Channel) *discordgo.Channel {
	for _, ch := range channels {
		if ch != nil && ch.Name == suggestionDiscussionChannelName {
			return ch
		}
	}
	return nil
}

func publishSuggestionAnnouncement(ctx context.Context, client suggestionDiscordClient, channelID string, closesAt time.Time) (suggestionMessageRef, error) {
	return publishSuggestionContent(ctx, client, channelID, suggestionAnnouncementThreadName, buildSuggestionAnnouncement(closesAt))
}

func publishSuggestionMessage(ctx context.Context, client suggestionDiscordClient, channelID, title, description string, voteCount int) (suggestionMessageRef, error) {
	return publishSuggestionContent(ctx, client, channelID, buildSuggestionThreadName(title), buildSuggestionMessage(title, description, voteCount))
}

func publishSuggestionContent(ctx context.Context, client suggestionDiscordClient, channelID, threadName, content string) (suggestionMessageRef, error) {
	ch, err := client.Channel(channelID, discordgo.WithContext(ctx))
	if err != nil {
		return suggestionMessageRef{}, fmt.Errorf("load suggestion channel: %w", err)
	}

	if isForumStyleChannel(ch.Type) {
		thread, err := client.ForumThreadStartComplex(
			channelID,
			&discordgo.ThreadStart{Name: buildSuggestionThreadName(threadName)},
			&discordgo.MessageSend{Content: content},
			discordgo.WithContext(ctx),
		)
		if err != nil {
			return suggestionMessageRef{}, fmt.Errorf("create suggestion forum post: %w", err)
		}
		if thread == nil || thread.ID == "" || thread.LastMessageID == "" {
			return suggestionMessageRef{}, errors.New("created suggestion forum post without message reference")
		}
		return suggestionMessageRef{ChannelID: thread.ID, MessageID: thread.LastMessageID}, nil
	}

	msg, err := client.ChannelMessageSend(channelID, content, discordgo.WithContext(ctx))
	if err != nil {
		return suggestionMessageRef{}, fmt.Errorf("send suggestion message: %w", err)
	}
	if msg == nil || msg.ID == "" {
		return suggestionMessageRef{}, errors.New("sent suggestion message without message reference")
	}
	return suggestionMessageRef{ChannelID: channelID, MessageID: msg.ID}, nil
}

func isForumStyleChannel(channelType discordgo.ChannelType) bool {
	return channelType == discordgo.ChannelTypeGuildForum || channelType == discordgo.ChannelTypeGuildMedia
}

func buildSuggestionThreadName(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		trimmed = suggestionDefaultThreadName
	}
	return truncateRunes(trimmed, suggestionThreadNameLimit)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
