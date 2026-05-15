package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
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
