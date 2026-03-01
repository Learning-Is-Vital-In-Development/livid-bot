package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const helpAutocompleteMaxChoices = 25
const helpAutocompleteChoiceNameLimit = 100
const helpEmbedDescriptionLimit = 4000
const helpEmbedFieldLimit = 1000
const helpEmbedColor = 0x5865F2

func handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logCommand(i, "start", "help command invoked")

	memberPermissions, hasMember := helpMemberPermissions(i)
	visibleCommands := visibleCommandsForMember(commands, memberPermissions, hasMember)
	selectedCommand := selectedHelpCommandName(i.ApplicationCommandData().Options)

	var embed *discordgo.MessageEmbed
	if selectedCommand == "" {
		embed = buildHelpOverviewEmbed(visibleCommands)
	} else {
		cmd := findVisibleCommandByName(visibleCommands, selectedCommand)
		if cmd == nil {
			respondError(s, i, fmt.Sprintf("`/%s` 명령어를 찾을 수 없거나 사용할 권한이 없습니다.", selectedCommand))
			return
		}
		embed = buildHelpCommandDetailEmbed(cmd)
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: newHelpResponseData(embed),
	}); err != nil {
		logCommand(i, "error", "failed to respond help command: %v", err)
		return
	}

	logCommand(i, "success", "help response sent visible_commands=%d selected=%q", len(visibleCommands), selectedCommand)
}

func handleHelpAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	memberPermissions, hasMember := helpMemberPermissions(i)
	query := focusedStringOptionValue(i.ApplicationCommandData().Options, "command")
	visibleCommands := visibleCommandsForMember(commands, memberPermissions, hasMember)
	choices := buildHelpCommandAutocompleteChoices(visibleCommands, query, helpAutocompleteMaxChoices)
	logCommand(i, "start", "help autocomplete query=%q", query)
	respondAutocomplete(s, i, choices)
	logCommand(i, "success", "help autocomplete choices=%d", len(choices))
}

func newHelpResponseData(embed *discordgo.MessageEmbed) *discordgo.InteractionResponseData {
	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = append(embeds, embed)
	}

	return &discordgo.InteractionResponseData{
		Embeds: embeds,
		Flags:  discordgo.MessageFlagsEphemeral,
	}
}

func helpMemberPermissions(i *discordgo.InteractionCreate) (int64, bool) {
	if i == nil || i.Member == nil {
		return 0, false
	}
	return i.Member.Permissions, true
}

func visibleCommandsForMember(cmds []*discordgo.ApplicationCommand, memberPermissions int64, hasMember bool) []*discordgo.ApplicationCommand {
	visible := make([]*discordgo.ApplicationCommand, 0, len(cmds))
	for _, cmd := range cmds {
		if commandVisibleToMember(cmd, memberPermissions, hasMember) {
			visible = append(visible, cmd)
		}
	}
	return visible
}

func selectedHelpCommandName(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	for _, opt := range options {
		if opt.Name == "command" {
			return normalizeHelpCommandName(opt.StringValue())
		}
	}
	return ""
}

func normalizeHelpCommandName(raw string) string {
	name := strings.TrimSpace(strings.ToLower(raw))
	return strings.TrimPrefix(name, "/")
}

func findVisibleCommandByName(cmds []*discordgo.ApplicationCommand, name string) *discordgo.ApplicationCommand {
	normalized := normalizeHelpCommandName(name)
	for _, cmd := range cmds {
		if normalizeHelpCommandName(cmd.Name) == normalized {
			return cmd
		}
	}
	return nil
}

func buildHelpOverviewEmbed(cmds []*discordgo.ApplicationCommand) *discordgo.MessageEmbed {
	var b strings.Builder
	b.WriteString("`/help command:<명령어>` 를 입력하면 상세 카드를 볼 수 있습니다.\n\n")

	for _, cmd := range cmds {
		fmt.Fprintf(&b, "- `%s`\n", cmd.Name)
	}

	desc := strings.TrimSpace(b.String())
	if len(cmds) == 0 {
		desc = "현재 권한으로 사용할 수 있는 명령어가 없습니다."
	}

	return &discordgo.MessageEmbed{
		Title:       "도움말",
		Description: truncateForDiscord(desc, helpEmbedDescriptionLimit),
		Color:       helpEmbedColor,
	}
}

func buildHelpCommandDetailEmbed(cmd *discordgo.ApplicationCommand) *discordgo.MessageEmbed {
	optionsText := buildHelpOptionLines(cmd.Options)
	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "설명",
			Value:  truncateForDiscord(localizedCommandDescription(cmd), helpEmbedFieldLimit),
			Inline: false,
		},
		{
			Name:   "권한",
			Value:  helpPermissionLabel(cmd),
			Inline: true,
		},
		{
			Name:   "옵션",
			Value:  truncateForDiscord(optionsText, helpEmbedFieldLimit),
			Inline: false,
		},
	}

	return &discordgo.MessageEmbed{
		Title:  cmd.Name,
		Color:  helpEmbedColor,
		Fields: fields,
	}
}

func buildHelpOptionLines(options []*discordgo.ApplicationCommandOption) string {
	if len(options) == 0 {
		return "없음"
	}

	var b strings.Builder
	for idx, opt := range options {
		if idx > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(
			&b,
			"- `%s` (%s, %s%s)",
			opt.Name,
			optionTypeLabel(opt.Type),
			requiredLabel(opt.Required),
			autocompleteSuffix(opt.Autocomplete),
		)
	}
	return b.String()
}

func helpPermissionLabel(cmd *discordgo.ApplicationCommand) string {
	if cmd == nil || cmd.DefaultMemberPermissions == nil {
		return "모든 멤버"
	}
	return "관리자 전용"
}

func buildHelpCommandAutocompleteChoices(
	cmds []*discordgo.ApplicationCommand,
	query string,
	limit int,
) []*discordgo.ApplicationCommandOptionChoice {
	if limit <= 0 {
		return nil
	}

	filter := normalizeHelpCommandName(query)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(limit, len(cmds)))
	for _, cmd := range cmds {
		target := normalizeHelpCommandName(cmd.Name + " " + localizedCommandDescription(cmd))
		if filter != "" && !strings.Contains(target, filter) {
			continue
		}

		label := truncateForDiscord(
			cmd.Name,
			helpAutocompleteChoiceNameLimit,
		)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: cmd.Name,
		})
		if len(choices) >= limit {
			break
		}
	}

	sort.SliceStable(choices, func(i, j int) bool {
		return choices[i].Name < choices[j].Name
	})
	return choices
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
		return "서브커맨드"
	case discordgo.ApplicationCommandOptionSubCommandGroup:
		return "서브커맨드 그룹"
	case discordgo.ApplicationCommandOptionString:
		return "문자열"
	case discordgo.ApplicationCommandOptionInteger:
		return "정수"
	case discordgo.ApplicationCommandOptionBoolean:
		return "불리언"
	case discordgo.ApplicationCommandOptionUser:
		return "사용자"
	case discordgo.ApplicationCommandOptionChannel:
		return "채널"
	case discordgo.ApplicationCommandOptionRole:
		return "역할"
	case discordgo.ApplicationCommandOptionMentionable:
		return "멘션 가능 대상"
	case discordgo.ApplicationCommandOptionNumber:
		return "숫자"
	case discordgo.ApplicationCommandOptionAttachment:
		return "첨부파일"
	default:
		return "알 수 없음"
	}
}

func requiredLabel(required bool) string {
	if required {
		return "필수"
	}
	return "선택"
}

func autocompleteSuffix(autocomplete bool) string {
	if autocomplete {
		return ", 자동완성"
	}
	return ""
}

func localizedCommandDescription(cmd *discordgo.ApplicationCommand) string {
	if cmd == nil {
		return ""
	}

	switch cmd.Name {
	case "help":
		return "사용 가능한 명령어 안내"
	case "create-study":
		return "새 스터디 채널과 역할 생성"
	case "recruit":
		return "활성 스터디 모집 메시지 게시"
	case "archive-study":
		return "특정 스터디 아카이브"
	case "studies":
		return "분기/상태 기준 스터디 목록 조회"
	case "members":
		return "스터디 활성 멤버 조회"
	case "study-start":
		return "분기 모집 종료 및 스터디 시작 처리"
	case "archive-all":
		return "활성 스터디 전체 아카이브"
	default:
		return cmd.Description
	}
}
