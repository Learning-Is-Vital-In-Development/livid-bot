package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	suggestionModalPrefix            = "suggest_modal"
	suggestionVisibilityAnonymous    = "anonymous"
	suggestionVisibilityPublic       = "public"
	suggestionDefaultDurationDays    = 14
	suggestionMaxDurationDays        = 90
)

type suggestionModalOptions struct {
	Visibility   string
	Threshold    int
	DurationDays int
	ChannelID    string
}

type suggestionPostOptions struct {
	Visibility     string
	ProposerUserID string
	Threshold      int
	ExpiresAt      time.Time
}

type suggestionDiscordClient interface {
	Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error)
	ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error)
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

func buildSuggestionMessage(title, description string, opts suggestionPostOptions) string {
	proposer := "익명"
	if opts.Visibility == suggestionVisibilityPublic && opts.ProposerUserID != "" {
		proposer = fmt.Sprintf("<@%s>", opts.ProposerUserID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "📬 스터디 제안\n\n제안자: %s\n**주제**: %s\n", proposer, title)
	if strings.TrimSpace(description) != "" {
		fmt.Fprintf(&b, "설명: %s\n", description)
	}
	fmt.Fprintf(&b, "\n🚀를 누르면 실제 참여 의사로 집계됩니다.\n🚀 %d명 이상이 모이면 스터디 채널이 자동 개설됩니다.\n마감: %s", opts.Threshold, suggestionDateLabel(opts.ExpiresAt))
	return b.String()
}

func buildSuggestModalCustomID(opts suggestionModalOptions) string {
	return fmt.Sprintf("%s:%s:%d:%d:%s", suggestionModalPrefix, opts.Visibility, opts.Threshold, opts.DurationDays, opts.ChannelID)
}

func parseSuggestModalCustomID(customID string) (suggestionModalOptions, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 5 || parts[0] != suggestionModalPrefix {
		return suggestionModalOptions{}, errors.New("invalid suggest modal custom id")
	}
	threshold, err := strconv.Atoi(parts[2])
	if err != nil || threshold < 1 {
		return suggestionModalOptions{}, errors.New("invalid suggest threshold")
	}
	durationDays, err := strconv.Atoi(parts[3])
	if err != nil || durationDays < 1 || durationDays > suggestionMaxDurationDays {
		return suggestionModalOptions{}, errors.New("invalid suggest duration")
	}
	visibility := parts[1]
	if visibility != suggestionVisibilityAnonymous && visibility != suggestionVisibilityPublic {
		return suggestionModalOptions{}, errors.New("invalid suggest visibility")
	}
	if parts[4] == "" {
		return suggestionModalOptions{}, errors.New("empty suggestion channel")
	}
	return suggestionModalOptions{Visibility: visibility, Threshold: threshold, DurationDays: durationDays, ChannelID: parts[4]}, nil
}

func suggestionExpiresAtFromDuration(now time.Time, days int) time.Time {
	if days < 1 {
		days = suggestionDefaultDurationDays
	}
	d := now.In(suggestionDeadlineLocation).AddDate(0, 0, days)
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, suggestionDeadlineLocation)
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

func publishSuggestionMessage(ctx context.Context, client suggestionDiscordClient, channelID, title, description string, opts suggestionPostOptions) (suggestionMessageRef, error) {
	return publishSuggestionContent(ctx, client, channelID, buildSuggestionThreadName(title), buildSuggestionMessage(title, description, opts))
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
			&discordgo.MessageSend{Content: content, AllowedMentions: &discordgo.MessageAllowedMentions{}},
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

	msg, err := client.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{Content: content, AllowedMentions: &discordgo.MessageAllowedMentions{}}, discordgo.WithContext(ctx))
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
