package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

const (
	voiceStatsDefaultLimit       = 20
	voiceStatsMaxLimit           = 25
	voiceStatsDateLayout         = "2006-01-02"
	voiceStatsMemberNameCacheTTL = 10 * time.Minute
)

var errInvalidVoiceStatsDateRange = errors.New("invalid voice stats date range")

type VoiceSessionStore interface {
	RecordVoiceTransition(ctx context.Context, guildID, userID, beforeChannelID, afterChannelID string, occurredAt time.Time) error
	CloseOpenSessions(ctx context.Context, endedAt time.Time, reason string) (int64, error)
	ListChannelStats(ctx context.Context, guildID, channelID string, from, to time.Time, limit int) ([]study.VoiceChannelStat, error)
}

type voiceStatsInteractionResponder interface {
	deferEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) error
	editOriginal(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error
}

type voiceStatsMemberNameResolver interface {
	displayName(s *discordgo.Session, guildID, userID string) (string, bool, error)
}

type voiceStatsMemberFetcher func(s *discordgo.Session, guildID, userID string) (*discordgo.Member, error)

type cachedVoiceStatsMemberResolver struct {
	mu      sync.Mutex
	cache   map[string]cachedVoiceStatsMemberName
	fetch   voiceStatsMemberFetcher
	nowFunc func() time.Time
}

type cachedVoiceStatsMemberName struct {
	displayName string
	expiresAt   time.Time
}

type discordVoiceStatsResponder struct{}

type voiceStatsDisplayRow struct {
	DisplayName          string
	DisplayNameIsMention bool
	SessionCount         int64
	TotalSeconds         int64
	Sessions             []voiceStatsDisplaySession
}

type voiceStatsDisplaySession struct {
	JoinedAt        time.Time
	LeftAt          time.Time
	DurationSeconds int64
	IsOpen          bool
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
	return newVoiceStatsHandlerWithDeps(
		repo,
		discordVoiceStatsResponder{},
		newCachedVoiceStatsMemberResolver(nil, nil),
	)
}

func newVoiceStatsHandlerWithDeps(
	repo VoiceSessionStore,
	responder voiceStatsInteractionResponder,
	memberResolver voiceStatsMemberNameResolver,
) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if responder == nil {
		responder = discordVoiceStatsResponder{}
	}
	if memberResolver == nil {
		memberResolver = newCachedVoiceStatsMemberResolver(nil, nil)
	}

	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if repo == nil {
			respondError(s, i, "음성 출석 저장소가 설정되어 있지 않습니다.")
			return
		}

		data := i.ApplicationCommandData()
		channelOpt := data.GetOption("channel")
		fromOpt := data.GetOption("from")
		toOpt := data.GetOption("to")
		if channelOpt == nil || fromOpt == nil {
			respondError(s, i, "channel, from 옵션은 필수입니다.")
			return
		}

		channelID := channelOpt.ChannelValue(nil).ID
		toRaw := ""
		if toOpt != nil {
			toRaw = toOpt.StringValue()
		}
		from, to, err := parseVoiceStatsDateRangeWithDefault(fromOpt.StringValue(), toRaw, time.Now(), voiceStatsLocation())
		if err != nil {
			respondError(s, i, "날짜 형식이 올바르지 않습니다. from/to는 YYYY-MM-DD 형식이며 to는 from 이후여야 합니다. to 생략 시 오늘(KST)로 처리됩니다.")
			return
		}
		limit := voiceStatsLimitFromOption(data.GetOption("limit"))

		logCommand(i, "start", "voice-stats requested channel=%s from=%s to=%s limit=%d", channelID, from.Format(time.RFC3339), to.Format(time.RFC3339), limit)

		if err := responder.deferEphemeral(s, i); err != nil {
			logCommand(i, "error", "failed to defer voice-stats command: %v", err)
			return
		}

		dbQueryStart := time.Now()
		stats, err := repo.ListChannelStats(context.Background(), i.GuildID, channelID, from, to, limit)
		dbQueryMs := time.Since(dbQueryStart).Milliseconds()
		if err != nil {
			slog.Error("failed to load voice stats", "guild_id", i.GuildID, "channel_id", channelID, "error", err)
			editVoiceStatsDeferredError(responder, s, i, "음성 출석 통계를 불러오지 못했습니다.")
			return
		}

		memberResolveStart := time.Now()
		rows := resolveVoiceStatsDisplayRowsWithResolver(s, i.GuildID, stats, memberResolver)
		memberResolveMs := time.Since(memberResolveStart).Milliseconds()

		channel := channelOpt.ChannelValue(s)
		content := buildVoiceStatsResponse(voiceChannelDisplayName(channel), from, to, rows)
		respondEditStart := time.Now()
		if err := responder.editOriginal(s, i, content); err != nil {
			logCommand(i, "error", "failed to edit voice-stats response: %v db_query_ms=%d member_resolve_ms=%d", err, dbQueryMs, memberResolveMs)
			return
		}
		respondEditMs := time.Since(respondEditStart).Milliseconds()
		logCommand(i, "success", "voice-stats returned count=%d channel=%s db_query_ms=%d member_resolve_ms=%d respond_edit_ms=%d", len(rows), channelID, dbQueryMs, memberResolveMs, respondEditMs)
	}
}

func editVoiceStatsDeferredError(responder voiceStatsInteractionResponder, s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(i, "error", "%s", message)
	if err := responder.editOriginal(s, i, message); err != nil {
		logCommand(i, "error", "failed to edit voice-stats error response: %v", err)
	}
}

func (discordVoiceStatsResponder) deferEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	})
}

func (discordVoiceStatsResponder) editOriginal(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, voiceStatsWebhookEdit(content))
	return err
}

func voiceStatsWebhookEdit(content string) *discordgo.WebhookEdit {
	return &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func newCachedVoiceStatsMemberResolver(fetch voiceStatsMemberFetcher, nowFunc func() time.Time) *cachedVoiceStatsMemberResolver {
	if fetch == nil {
		fetch = defaultVoiceStatsMemberFetcher
	}
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &cachedVoiceStatsMemberResolver{
		cache:   make(map[string]cachedVoiceStatsMemberName),
		fetch:   fetch,
		nowFunc: nowFunc,
	}
}

func defaultVoiceStatsMemberFetcher(s *discordgo.Session, guildID, userID string) (*discordgo.Member, error) {
	if s == nil {
		return nil, errors.New("discord session is nil")
	}
	return s.GuildMember(guildID, userID)
}

func (r *cachedVoiceStatsMemberResolver) displayName(s *discordgo.Session, guildID, userID string) (string, bool, error) {
	fallbackName, fallbackIsMention := voiceStatsFallbackDisplayName(userID)
	if guildID == "" || userID == "" {
		return fallbackName, fallbackIsMention, errors.New("guild id and user id are required")
	}
	if r == nil {
		return fallbackName, fallbackIsMention, errors.New("voice stats member resolver is nil")
	}
	if r.nowFunc == nil {
		r.nowFunc = time.Now
	}
	if r.fetch == nil {
		r.fetch = defaultVoiceStatsMemberFetcher
	}
	if r.cache == nil {
		r.cache = make(map[string]cachedVoiceStatsMemberName)
	}

	key := guildID + ":" + userID
	now := r.nowFunc()
	r.mu.Lock()
	cached, ok := r.cache[key]
	if ok && now.Before(cached.expiresAt) {
		r.mu.Unlock()
		return cached.displayName, false, nil
	}
	r.mu.Unlock()

	member, err := r.fetch(s, guildID, userID)
	if err != nil {
		return fallbackName, fallbackIsMention, err
	}
	displayName := voiceMemberDisplayName(member)
	if displayName == "" || displayName == "알 수 없는 사용자" {
		return fallbackName, fallbackIsMention, errors.New("resolved member has no display name")
	}

	r.mu.Lock()
	r.cache[key] = cachedVoiceStatsMemberName{
		displayName: displayName,
		expiresAt:   now.Add(voiceStatsMemberNameCacheTTL),
	}
	r.mu.Unlock()
	return displayName, false, nil
}

func voiceStatsFallbackDisplayName(userID string) (string, bool) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "알 수 없는 사용자", false
	}
	return fmt.Sprintf("<@%s>", userID), true
}

func parseVoiceStatsDateRange(fromRaw, toRaw string, loc *time.Location) (time.Time, time.Time, error) {
	return parseVoiceStatsDateRangeWithDefault(fromRaw, toRaw, time.Time{}, loc)
}

func parseVoiceStatsDateRangeWithDefault(fromRaw, toRaw string, now time.Time, loc *time.Location) (time.Time, time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	from, err := time.ParseInLocation(voiceStatsDateLayout, strings.TrimSpace(fromRaw), loc)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: parse from: %w", errInvalidVoiceStatsDateRange, err)
	}

	toRaw = strings.TrimSpace(toRaw)
	if toRaw == "" {
		if now.IsZero() {
			now = time.Now()
		}
		toRaw = now.In(loc).Format(voiceStatsDateLayout)
	}
	toInclusive, err := time.ParseInLocation(voiceStatsDateLayout, toRaw, loc)
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

func resolveVoiceStatsDisplayRowsWithResolver(s *discordgo.Session, guildID string, stats []study.VoiceChannelStat, resolver voiceStatsMemberNameResolver) []voiceStatsDisplayRow {
	if resolver == nil {
		resolver = newCachedVoiceStatsMemberResolver(nil, nil)
	}
	rows := make([]voiceStatsDisplayRow, 0, len(stats))
	for _, stat := range stats {
		displayName, isMention, err := resolver.displayName(s, guildID, stat.UserID)
		if err != nil {
			slog.Warn("failed to resolve voice stats member display name", "guild_id", guildID, "user_id", stat.UserID, "error", err)
		}
		rows = append(rows, voiceStatsDisplayRow{
			DisplayName:          displayName,
			DisplayNameIsMention: isMention,
			SessionCount:         stat.SessionCount,
			TotalSeconds:         stat.TotalSeconds,
			Sessions:             resolveVoiceStatsDisplaySessions(stat.Sessions),
		})
	}
	return rows
}

func resolveVoiceStatsDisplaySessions(sessions []study.VoiceChannelSession) []voiceStatsDisplaySession {
	if len(sessions) == 0 {
		return nil
	}
	displaySessions := make([]voiceStatsDisplaySession, 0, len(sessions))
	for _, session := range sessions {
		displaySessions = append(displaySessions, voiceStatsDisplaySession{
			JoinedAt:        session.JoinedAt,
			LeftAt:          session.LeftAt,
			DurationSeconds: session.DurationSeconds,
			IsOpen:          session.IsOpen,
		})
	}
	return displaySessions
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
		fmt.Fprintf(&b, "%d. %s — 총 %s (%d회)\n", idx+1, formatVoiceStatsDisplayName(row), formatVoiceDuration(row.TotalSeconds), row.SessionCount)
		for _, session := range row.Sessions {
			fmt.Fprintf(&b, "   • %s — %s\n", formatVoiceSessionWindow(session, from.Location()), formatVoiceDuration(session.DurationSeconds))
		}
	}
	return truncateForDiscord(strings.TrimSuffix(b.String(), "\n"), discordMessageLimit)
}

func formatVoiceSessionWindow(session voiceStatsDisplaySession, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	joinedAt := session.JoinedAt.In(loc)
	leftAt := session.LeftAt.In(loc)
	startLabel := joinedAt.Format("2006-01-02 15:04")
	endLabel := leftAt.Format("15:04")
	if session.IsOpen {
		endLabel = "현재"
	} else if joinedAt.Format(voiceStatsDateLayout) != leftAt.Format(voiceStatsDateLayout) {
		endLabel = leftAt.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("%s ~ %s", startLabel, endLabel)
}

func formatVoiceStatsDisplayName(row voiceStatsDisplayRow) string {
	if row.DisplayNameIsMention && isDiscordUserMention(row.DisplayName) {
		return row.DisplayName
	}
	return sanitizeVoiceStatsText(row.DisplayName)
}

func isDiscordUserMention(value string) bool {
	if !strings.HasPrefix(value, "<@") || !strings.HasSuffix(value, ">") {
		return false
	}
	id := strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(value, "<@"), ">"), "!")
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
