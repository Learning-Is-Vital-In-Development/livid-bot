package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

const (
	voiceStatsDefaultLimit = 20
	voiceStatsMaxLimit     = 25
	voiceStatsDateLayout   = "2006-01-02"
)

var errInvalidVoiceStatsDateRange = errors.New("invalid voice stats date range")

type VoiceSessionStore interface {
	RecordVoiceTransition(ctx context.Context, guildID, userID, beforeChannelID, afterChannelID string, occurredAt time.Time) error
	CloseOpenSessions(ctx context.Context, endedAt time.Time, reason string) (int64, error)
	ListChannelStats(ctx context.Context, guildID, channelID string, from, to time.Time, limit int) ([]study.VoiceChannelStat, error)
}

type voiceStatsDisplayRow struct {
	DisplayName  string
	SessionCount int64
	TotalSeconds int64
}

func newVoiceStateHandler(repo VoiceSessionStore, configuredGuildID string) func(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	return func(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
		if repo == nil || v == nil || v.VoiceState == nil {
			return
		}
		if configuredGuildID != "" && v.GuildID != configuredGuildID {
			return
		}
		if v.UserID == "" {
			return
		}
		if s != nil && s.State != nil && s.State.User != nil && s.State.User.ID == v.UserID {
			return
		}

		beforeChannelID, afterChannelID, ok := voiceChannelTransition(v)
		if !ok {
			return
		}

		if err := repo.RecordVoiceTransition(
			context.Background(),
			v.GuildID,
			v.UserID,
			beforeChannelID,
			afterChannelID,
			time.Now().UTC(),
		); err != nil {
			slog.Error("failed to record voice channel transition",
				"guild_id", v.GuildID,
				"before_channel_id", beforeChannelID,
				"after_channel_id", afterChannelID,
				"error", err,
			)
			return
		}

		slog.Info("recorded voice channel transition",
			"guild_id", v.GuildID,
			"before_channel_id", beforeChannelID,
			"after_channel_id", afterChannelID,
		)
	}
}

func voiceChannelTransition(v *discordgo.VoiceStateUpdate) (beforeChannelID, afterChannelID string, ok bool) {
	if v == nil || v.VoiceState == nil {
		return "", "", false
	}
	if v.BeforeUpdate != nil {
		beforeChannelID = v.BeforeUpdate.ChannelID
	}
	afterChannelID = v.ChannelID
	if beforeChannelID == afterChannelID {
		return beforeChannelID, afterChannelID, false
	}
	if beforeChannelID == "" && afterChannelID == "" {
		return beforeChannelID, afterChannelID, false
	}
	return beforeChannelID, afterChannelID, true
}

func newVoiceStatsHandler(repo VoiceSessionStore) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if repo == nil {
			respondError(s, i, "음성 출석 저장소가 설정되어 있지 않습니다.")
			return
		}

		data := i.ApplicationCommandData()
		channelOpt := data.GetOption("channel")
		fromOpt := data.GetOption("from")
		toOpt := data.GetOption("to")
		if channelOpt == nil || fromOpt == nil || toOpt == nil {
			respondError(s, i, "channel, from, to 옵션은 필수입니다.")
			return
		}

		channel := channelOpt.ChannelValue(s)
		channelID := channel.ID
		from, to, err := parseVoiceStatsDateRange(fromOpt.StringValue(), toOpt.StringValue(), voiceStatsLocation())
		if err != nil {
			respondError(s, i, "날짜 형식이 올바르지 않습니다. from/to는 YYYY-MM-DD 형식이며 to는 from 이후여야 합니다.")
			return
		}
		limit := voiceStatsLimitFromOption(data.GetOption("limit"))

		logCommand(i, "start", "voice-stats requested channel=%s from=%s to=%s limit=%d", channelID, from.Format(time.RFC3339), to.Format(time.RFC3339), limit)

		stats, err := repo.ListChannelStats(context.Background(), i.GuildID, channelID, from, to, limit)
		if err != nil {
			slog.Error("failed to load voice stats", "guild_id", i.GuildID, "channel_id", channelID, "error", err)
			respondError(s, i, "음성 출석 통계를 불러오지 못했습니다.")
			return
		}

		rows := resolveVoiceStatsDisplayRows(s, i.GuildID, stats)
		content := buildVoiceStatsResponse(voiceChannelDisplayName(channel), from, to, rows)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:         content,
				Flags:           discordgo.MessageFlagsEphemeral,
				AllowedMentions: &discordgo.MessageAllowedMentions{},
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond voice-stats command: %v", err)
			return
		}
		logCommand(i, "success", "voice-stats returned count=%d channel=%s", len(rows), channelID)
	}
}

func parseVoiceStatsDateRange(fromRaw, toRaw string, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	from, err := time.ParseInLocation(voiceStatsDateLayout, strings.TrimSpace(fromRaw), loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: parse from: %w", errInvalidVoiceStatsDateRange, err)
	}
	toInclusive, err := time.ParseInLocation(voiceStatsDateLayout, strings.TrimSpace(toRaw), loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: parse to: %w", errInvalidVoiceStatsDateRange, err)
	}
	toExclusive := toInclusive.AddDate(0, 0, 1)
	if !from.Before(toExclusive) {
		return time.Time{}, time.Time{}, errInvalidVoiceStatsDateRange
	}
	return from, toExclusive, nil
}

func voiceStatsLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.Local
	}
	return loc
}

func voiceStatsLimitFromOption(opt *discordgo.ApplicationCommandInteractionDataOption) int {
	if opt == nil {
		return voiceStatsDefaultLimit
	}
	limit := int(opt.IntValue())
	if limit < 1 {
		return 1
	}
	if limit > voiceStatsMaxLimit {
		return voiceStatsMaxLimit
	}
	return limit
}

func resolveVoiceStatsDisplayRows(s *discordgo.Session, guildID string, stats []study.VoiceChannelStat) []voiceStatsDisplayRow {
	rows := make([]voiceStatsDisplayRow, 0, len(stats))
	for _, stat := range stats {
		displayName := "알 수 없는 사용자"
		if s != nil && guildID != "" && stat.UserID != "" {
			member, err := s.GuildMember(guildID, stat.UserID)
			if err != nil {
				slog.Warn("failed to resolve voice stats member display name", "guild_id", guildID, "error", err)
			} else {
				displayName = voiceMemberDisplayName(member)
			}
		}
		rows = append(rows, voiceStatsDisplayRow{
			DisplayName:  displayName,
			SessionCount: stat.SessionCount,
			TotalSeconds: stat.TotalSeconds,
		})
	}
	return rows
}

func voiceMemberDisplayName(member *discordgo.Member) string {
	if member == nil {
		return "알 수 없는 사용자"
	}
	if member.Nick != "" {
		return member.Nick
	}
	if member.User == nil {
		return "알 수 없는 사용자"
	}
	if displayName := member.User.DisplayName(); displayName != "" {
		return displayName
	}
	return "알 수 없는 사용자"
}

func voiceChannelDisplayName(channel *discordgo.Channel) string {
	if channel == nil {
		return "알 수 없는 음성채널"
	}
	if channel.Name != "" {
		return channel.Name
	}
	return fmt.Sprintf("<#%s>", channel.ID)
}

func buildVoiceStatsResponse(channelName string, from, toExclusive time.Time, rows []voiceStatsDisplayRow) string {
	toInclusive := toExclusive.AddDate(0, 0, -1)
	var b strings.Builder
	fmt.Fprintf(&b, "🎙️ **%s** 음성 출석 통계\n", sanitizeVoiceStatsText(channelName))
	fmt.Fprintf(&b, "기간: %s ~ %s\n", from.Format(voiceStatsDateLayout), toInclusive.Format(voiceStatsDateLayout))

	if len(rows) == 0 {
		b.WriteString("기록이 없습니다.")
		return b.String()
	}

	for idx, row := range rows {
		fmt.Fprintf(&b, "%d. %s — %s (%d회)\n", idx+1, sanitizeVoiceStatsText(row.DisplayName), formatVoiceDuration(row.TotalSeconds), row.SessionCount)
	}
	return truncateForDiscord(strings.TrimSuffix(b.String(), "\n"), discordMessageLimit)
}

func sanitizeVoiceStatsText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "알 수 없음"
	}
	replacer := strings.NewReplacer(
		"\r", " ",
		"\n", " ",
		"@", "@\u200b",
	)
	return replacer.Replace(value)
}

func formatVoiceDuration(seconds int64) string {
	if seconds < 60 {
		return "1분 미만"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%d시간 %d분", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d시간", hours)
	default:
		return fmt.Sprintf("%d분", minutes)
	}
}
