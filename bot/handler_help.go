package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logCommand(i, "start", "help command invoked")

	memberPermissions := int64(0)
	hasMember := i != nil && i.Member != nil
	if hasMember {
		memberPermissions = i.Member.Permissions
	}

	content, visibleCount := buildHelpResponse(commands, memberPermissions, hasMember)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: newHelpResponseData(content),
	}); err != nil {
		logCommand(i, "error", "failed to respond help command: %v", err)
		return
	}

	logCommand(i, "success", "help response sent visible_commands=%d", visibleCount)
}

func newHelpResponseData(content string) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	}
}

func buildHelpResponse(cmds []*discordgo.ApplicationCommand, memberPermissions int64, hasMember bool) (string, int) {
	var b strings.Builder
	visibleCount := 0

	b.WriteString("Available Commands")

	for _, cmd := range cmds {
		if !commandVisibleToMember(cmd, memberPermissions, hasMember) {
			continue
		}

		visibleCount++
		fmt.Fprintf(&b, "\n\n/%s - %s", cmd.Name, cmd.Description)

		for _, opt := range cmd.Options {
			fmt.Fprintf(
				&b,
				"\n  - `%s` (%s, %s%s)",
				opt.Name,
				optionTypeLabel(opt.Type),
				requiredLabel(opt.Required),
				autocompleteSuffix(opt.Autocomplete),
			)
		}
	}

	if visibleCount == 0 {
		b.WriteString("\nNo commands available with your current permissions.")
	}

	return truncateForDiscord(b.String(), discordMessageLimit), visibleCount
}

func commandVisibleToMember(cmd *discordgo.ApplicationCommand, memberPermissions int64, hasMember bool) bool {
	if cmd == nil || cmd.DefaultMemberPermissions == nil {
		return true
	}
	if !hasMember {
		return false
	}
	required := *cmd.DefaultMemberPermissions
	if required == 0 {
		return true
	}
	return memberPermissions&required == required
}

func optionTypeLabel(optionType discordgo.ApplicationCommandOptionType) string {
	switch optionType {
	case discordgo.ApplicationCommandOptionSubCommand:
		return "subcommand"
	case discordgo.ApplicationCommandOptionSubCommandGroup:
		return "subcommand-group"
	case discordgo.ApplicationCommandOptionString:
		return "string"
	case discordgo.ApplicationCommandOptionInteger:
		return "integer"
	case discordgo.ApplicationCommandOptionBoolean:
		return "boolean"
	case discordgo.ApplicationCommandOptionUser:
		return "user"
	case discordgo.ApplicationCommandOptionChannel:
		return "channel"
	case discordgo.ApplicationCommandOptionRole:
		return "role"
	case discordgo.ApplicationCommandOptionMentionable:
		return "mentionable"
	case discordgo.ApplicationCommandOptionNumber:
		return "number"
	case discordgo.ApplicationCommandOptionAttachment:
		return "attachment"
	default:
		return "unknown"
	}
}

func requiredLabel(required bool) string {
	if required {
		return "required"
	}
	return "optional"
}

func autocompleteSuffix(autocomplete bool) string {
	if autocomplete {
		return ", autocomplete"
	}
	return ""
}
