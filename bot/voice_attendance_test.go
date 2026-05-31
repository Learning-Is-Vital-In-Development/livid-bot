package bot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

func TestVoiceChannelTransitionClassifiesMovement(t *testing.T) {
	tests := []struct {
		name       string
		before     *discordgo.VoiceState
		after      *discordgo.VoiceState
		wantBefore string
		wantAfter  string
		wantOK     bool
	}{
		{
			name:       "join",
			after:      &discordgo.VoiceState{ChannelID: "voice-1"},
			wantBefore: "",
			wantAfter:  "voice-1",
			wantOK:     true,
		},
		{
			name:       "leave",
			before:     &discordgo.VoiceState{ChannelID: "voice-1"},
			after:      &discordgo.VoiceState{ChannelID: ""},
			wantBefore: "voice-1",
			wantAfter:  "",
			wantOK:     true,
		},
		{
			name:       "move",
			before:     &discordgo.VoiceState{ChannelID: "voice-1"},
			after:      &discordgo.VoiceState{ChannelID: "voice-2"},
			wantBefore: "voice-1",
			wantAfter:  "voice-2",
			wantOK:     true,
		},
		{
			name:   "mute-only update",
			before: &discordgo.VoiceState{ChannelID: "voice-1"},
			after:  &discordgo.VoiceState{ChannelID: "voice-1", SelfMute: true},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after, ok := voiceChannelTransition(&discordgo.VoiceStateUpdate{
				VoiceState:   tt.after,
				BeforeUpdate: tt.before,
			})
			if ok != tt.wantOK {
				t.Fatalf("transition ok = %v, want %v (before=%q after=%q)", ok, tt.wantOK, before, after)
			}
			if ok && (before != tt.wantBefore || after != tt.wantAfter) {
				t.Fatalf("transition = before %q after %q, want before %q after %q",
					before, after, tt.wantBefore, tt.wantAfter)
			}
		})
	}
}

func TestParseVoiceStatsDateRangeUsesInclusiveEndDate(t *testing.T) {
	from, to, err := parseVoiceStatsDateRange("2026-05-01", "2026-05-03", time.UTC)
	if err != nil {
		t.Fatalf("parse date range: %v", err)
	}
	if !from.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected from: %s", from)
	}
	if !to.Equal(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected exclusive to: %s", to)
	}
}

func TestParseVoiceStatsDateRangeDefaultsEmptyToToday(t *testing.T) {
	loc := time.FixedZone("KST", 9*60*60)
	now := time.Date(2026, 5, 16, 13, 30, 0, 0, loc)

	from, to, err := parseVoiceStatsDateRangeWithDefault("2026-05-01", "", now, loc)
	if err != nil {
		t.Fatalf("parse date range with default to: %v", err)
	}
	if !from.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, loc)) {
		t.Fatalf("unexpected from: %s", from)
	}
	if !to.Equal(time.Date(2026, 5, 17, 0, 0, 0, 0, loc)) {
		t.Fatalf("expected default to to include today's KST date, got: %s", to)
	}
}

func TestBuildVoiceStatsResponseUsesDisplayNamesWithoutUserIDs(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	content := buildVoiceStatsResponse("스터디 음성방", from, to, []voiceStatsDisplayRow{
		{DisplayName: "하릴", SessionCount: 2, TotalSeconds: int64(90 * time.Minute / time.Second)},
	})

	if !strings.Contains(content, "스터디 음성방") || !strings.Contains(content, "하릴") {
		t.Fatalf("expected channel and display name in response, got: %s", content)
	}
	if !strings.Contains(content, "1시간 30분") || !strings.Contains(content, "2회") {
		t.Fatalf("expected duration and session count in response, got: %s", content)
	}
	if strings.Contains(content, "user-1") || strings.Contains(content, "<@") {
		t.Fatalf("response must not expose user IDs or mentions, got: %s", content)
	}
}

func TestBuildVoiceStatsResponseIncludesSessionDetails(t *testing.T) {
	loc := time.FixedZone("KST", 9*60*60)
	from := time.Date(2026, 5, 16, 0, 0, 0, 0, loc)
	to := time.Date(2026, 5, 17, 0, 0, 0, 0, loc)
	content := buildVoiceStatsResponse("스터디 음성방", from, to, []voiceStatsDisplayRow{
		{
			DisplayName:  "하릴",
			SessionCount: 2,
			TotalSeconds: int64((30*time.Minute + 75*time.Minute) / time.Second),
			Sessions: []voiceStatsDisplaySession{
				{
					JoinedAt:        time.Date(2026, 5, 16, 9, 5, 0, 0, loc),
					LeftAt:          time.Date(2026, 5, 16, 9, 35, 0, 0, loc),
					DurationSeconds: int64(30 * time.Minute / time.Second),
				},
				{
					JoinedAt:        time.Date(2026, 5, 16, 13, 0, 0, 0, loc),
					LeftAt:          time.Date(2026, 5, 16, 14, 15, 0, 0, loc),
					DurationSeconds: int64(75 * time.Minute / time.Second),
				},
			},
		},
	})

	checks := []string{
		"하릴 — 총 1시간 45분 (2회)",
		"2026-05-16 09:05 ~ 09:35 — 30분",
		"2026-05-16 13:00 ~ 14:15 — 1시간 15분",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Fatalf("expected response to include %q, got: %s", check, content)
		}
	}
}

func TestBuildVoiceStatsResponseBreaksMentionsFromDisplayNames(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	content := buildVoiceStatsResponse("@everyone", from, to, []voiceStatsDisplayRow{
		{DisplayName: "<@123456789>", SessionCount: 1, TotalSeconds: int64(time.Hour / time.Second)},
	})

	if strings.Contains(content, "@everyone") || strings.Contains(content, "<@123456789>") {
		t.Fatalf("expected mention-like text to be broken, got: %s", content)
	}
}

func TestVoiceMemberDisplayNamePrefersNickname(t *testing.T) {
	member := &discordgo.Member{
		Nick: "서버닉",
		User: &discordgo.User{Username: "username", GlobalName: "global"},
	}
	if got := voiceMemberDisplayName(member); got != "서버닉" {
		t.Fatalf("expected nickname, got %q", got)
	}

	member.Nick = ""
	if got := voiceMemberDisplayName(member); got != "global" {
		t.Fatalf("expected global display name, got %q", got)
	}
}

func TestVoiceStatsCommandIsAdminOnly(t *testing.T) {
	cmd := findCommandForTest("voice-stats")
	if cmd == nil {
		t.Fatal("expected voice-stats command to be registered")
	}
	if cmd.DefaultMemberPermissions == nil || *cmd.DefaultMemberPermissions != discordgo.PermissionAdministrator {
		t.Fatalf("expected voice-stats to be administrator-only, got %+v", cmd.DefaultMemberPermissions)
	}
	channelOption := findCommandOptionForTest(cmd, "channel")
	if channelOption == nil {
		t.Fatal("expected voice-stats channel option")
	}
	if len(channelOption.ChannelTypes) != 1 || channelOption.ChannelTypes[0] != discordgo.ChannelTypeGuildVoice {
		t.Fatalf("expected channel option to allow only guild voice channels, got %+v", channelOption.ChannelTypes)
	}
	toOption := findCommandOptionForTest(cmd, "to")
	if toOption == nil {
		t.Fatal("expected voice-stats to option")
	}
	if toOption.Required {
		t.Fatal("expected voice-stats to option to be optional so it can default to today")
	}
}

func TestVoiceStatsHandlerDefersBeforeLoadingStatsAndEditsOriginalResponse(t *testing.T) {
	responder := &fakeVoiceStatsResponder{}
	repo := &fakeVoiceSessionStore{
		listFunc: func(_ context.Context, guildID, channelID string, from, to time.Time, limit int) ([]study.VoiceChannelStat, error) {
			if responder.deferCalls != 1 {
				t.Fatalf("expected interaction to be deferred before DB/member work, defer calls=%d", responder.deferCalls)
			}
			if guildID != "guild-1" || channelID != "voice-1" || limit != 20 {
				t.Fatalf("unexpected query args guild=%q channel=%q limit=%d", guildID, channelID, limit)
			}
			if from.Format(voiceStatsDateLayout) != "2026-05-01" || to.Sub(from) != 24*time.Hour {
				t.Fatalf("unexpected query range from=%s to=%s", from, to)
			}
			return []study.VoiceChannelStat{{UserID: "user-1", SessionCount: 1, TotalSeconds: int64(time.Hour / time.Second)}}, nil
		},
	}
	resolver := &fakeVoiceStatsMemberResolver{names: map[string]string{"user-1": "하릴"}}
	handler := newVoiceStatsHandlerWithDeps(repo, responder, resolver)

	handler(nil, newVoiceStatsInteractionForTest("voice-1", "2026-05-01", "2026-05-01"))

	if responder.deferCalls != 1 {
		t.Fatalf("expected one defer call, got %d", responder.deferCalls)
	}
	if responder.editCalls != 1 {
		t.Fatalf("expected one edit call, got %d", responder.editCalls)
	}
	if !strings.Contains(responder.editedContent, "하릴") || !strings.Contains(responder.editedContent, "1시간") {
		t.Fatalf("expected final content to contain resolved stats, got: %s", responder.editedContent)
	}
}

func TestVoiceStatsHandlerLogsPhaseDurations(t *testing.T) {
	orig := slog.Default()
	logHandler := &recordingHandler{}
	slog.SetDefault(slog.New(logHandler))
	defer slog.SetDefault(orig)

	responder := &fakeVoiceStatsResponder{}
	repo := &fakeVoiceSessionStore{
		listFunc: func(context.Context, string, string, time.Time, time.Time, int) ([]study.VoiceChannelStat, error) {
			return nil, nil
		},
	}
	handler := newVoiceStatsHandlerWithDeps(repo, responder, &fakeVoiceStatsMemberResolver{})

	handler(nil, newVoiceStatsInteractionForTest("voice-1", "2026-05-01", "2026-05-01"))

	var success string
	for _, record := range logHandler.records {
		if record.Level == slog.LevelInfo && strings.Contains(record.Message, "voice-stats returned") {
			success = record.Message
			break
		}
	}
	if success == "" {
		t.Fatalf("expected voice-stats success log, got records: %+v", logHandler.records)
	}
	for _, field := range []string{"db_query_ms=", "member_resolve_ms=", "respond_edit_ms="} {
		if !strings.Contains(success, field) {
			t.Fatalf("expected success log to include %s, got: %s", field, success)
		}
	}
}

func TestVoiceStatsHandlerEditsDeferredErrorAndAuditsWhenStatsLoadFails(t *testing.T) {
	store := &fakeAuditStore{}
	setCommandAuditStore(store)
	defer setCommandAuditStore(nil)

	responder := &fakeVoiceStatsResponder{}
	repo := &fakeVoiceSessionStore{
		listFunc: func(context.Context, string, string, time.Time, time.Time, int) ([]study.VoiceChannelStat, error) {
			return nil, errors.New("database unavailable")
		},
	}
	handler := newVoiceStatsHandlerWithDeps(repo, responder, &fakeVoiceStatsMemberResolver{})

	handler(nil, newVoiceStatsInteractionForTest("voice-1", "2026-05-01", "2026-05-01"))

	if responder.deferCalls != 1 || responder.editCalls != 1 {
		t.Fatalf("expected defer then one error edit, defer=%d edit=%d", responder.deferCalls, responder.editCalls)
	}
	if responder.editedContent != "음성 출석 통계를 불러오지 못했습니다." {
		t.Fatalf("unexpected deferred error content: %s", responder.editedContent)
	}
	if len(store.errors) != 1 || !strings.Contains(store.errors[0].message, "음성 출석 통계를 불러오지 못했습니다") {
		t.Fatalf("expected command audit error, got %+v", store.errors)
	}
}

func TestCachedVoiceStatsMemberResolverCachesNamesAndFallsBackToSafeMention(t *testing.T) {
	calls := map[string]int{}
	resolver := newCachedVoiceStatsMemberResolver(
		func(_ *discordgo.Session, _ string, userID string) (*discordgo.Member, error) {
			calls[userID]++
			if userID == "123456789012345678" {
				return &discordgo.Member{Nick: "하릴", User: &discordgo.User{Username: "haril"}}, nil
			}
			return nil, errors.New("discord member lookup failed")
		},
		func() time.Time { return time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC) },
	)

	first, mention, err := resolver.displayName(nil, "guild-1", "123456789012345678")
	if err != nil {
		t.Fatalf("resolve first name: %v", err)
	}
	second, secondMention, err := resolver.displayName(nil, "guild-1", "123456789012345678")
	if err != nil {
		t.Fatalf("resolve cached name: %v", err)
	}
	if first != "하릴" || second != "하릴" || mention || secondMention {
		t.Fatalf("unexpected cached names first=(%q,%v) second=(%q,%v)", first, mention, second, secondMention)
	}
	if calls["123456789012345678"] != 1 {
		t.Fatalf("expected successful lookup to be cached, calls=%d", calls["123456789012345678"])
	}

	fallback, isMention, err := resolver.displayName(nil, "guild-1", "999999999999999999")
	if err == nil {
		t.Fatal("expected lookup error to be returned for logging")
	}
	if fallback != "<@999999999999999999>" || !isMention {
		t.Fatalf("expected safe mention fallback, got name=%q mention=%v", fallback, isMention)
	}
}

func TestVoiceStatsWebhookEditDisablesMentions(t *testing.T) {
	edit := voiceStatsWebhookEdit("hello <@999999999999999999>")
	if edit.Content == nil || *edit.Content == "" {
		t.Fatal("expected edit content to be set")
	}
	if edit.AllowedMentions == nil {
		t.Fatal("expected allowed_mentions to be set")
	}
	if len(edit.AllowedMentions.Parse) != 0 || len(edit.AllowedMentions.Users) != 0 || len(edit.AllowedMentions.Roles) != 0 || edit.AllowedMentions.RepliedUser {
		t.Fatalf("expected all mentions to be disabled, got %+v", edit.AllowedMentions)
	}
}

func findCommandForTest(name string) *discordgo.ApplicationCommand {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}

func findCommandOptionForTest(cmd *discordgo.ApplicationCommand, name string) *discordgo.ApplicationCommandOption {
	for _, opt := range cmd.Options {
		if opt.Name == name {
			return opt
		}
	}
	return nil
}

type fakeVoiceSessionStore struct {
	listFunc func(context.Context, string, string, time.Time, time.Time, int) ([]study.VoiceChannelStat, error)
}

func (f *fakeVoiceSessionStore) RecordVoiceTransition(context.Context, string, string, string, string, time.Time) error {
	return nil
}

func (f *fakeVoiceSessionStore) CloseOpenSessions(context.Context, time.Time, string) (int64, error) {
	return 0, nil
}

func (f *fakeVoiceSessionStore) ListChannelStats(ctx context.Context, guildID, channelID string, from, to time.Time, limit int) ([]study.VoiceChannelStat, error) {
	if f.listFunc == nil {
		return nil, nil
	}
	return f.listFunc(ctx, guildID, channelID, from, to, limit)
}

type fakeVoiceStatsResponder struct {
	deferCalls    int
	editCalls     int
	editedContent string
}

func (f *fakeVoiceStatsResponder) deferEphemeral(*discordgo.Session, *discordgo.InteractionCreate) error {
	f.deferCalls++
	return nil
}

func (f *fakeVoiceStatsResponder) editOriginal(_ *discordgo.Session, _ *discordgo.InteractionCreate, content string) error {
	f.editCalls++
	f.editedContent = content
	return nil
}

type fakeVoiceStatsMemberResolver struct {
	names map[string]string
}

func (f *fakeVoiceStatsMemberResolver) displayName(_ *discordgo.Session, _ string, userID string) (string, bool, error) {
	if f.names != nil {
		if name, ok := f.names[userID]; ok {
			return name, false, nil
		}
	}
	return "<@" + userID + ">", true, errors.New("missing fake member")
}

func newVoiceStatsInteractionForTest(channelID, from, to string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:        "interaction-1",
		AppID:     "app-1",
		Token:     "token-1",
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   "guild-1",
		ChannelID: "command-channel-1",
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "voice-stats",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: channelID},
				{Name: "from", Type: discordgo.ApplicationCommandOptionString, Value: from},
				{Name: "to", Type: discordgo.ApplicationCommandOptionString, Value: to},
			},
		},
	}}
}
