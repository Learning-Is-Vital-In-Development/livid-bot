package bot

import "github.com/bwmarrin/discordgo"

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "help",
		Description: "사용 가능한 명령어 안내",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "command",
				Description:  "상세 도움말을 볼 명령어 (자동완성)",
				Required:     false,
				Autocomplete: true,
			},
		},
	},
	{
		Name:                     "archive-study",
		Description:              "Archive a specific study",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "channel",
				Description:  "Study channel to archive (autocomplete)",
				Required:     true,
				Autocomplete: true,
			},
		},
	},
	{
		Name:                     "studies",
		Description:              "List studies by branch/status",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "branch",
				Description: "Branch filter (YY-Q)",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "status",
				Description: "Study status",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "active",
						Value: "active",
					},
					{
						Name:  "archived",
						Value: "archived",
					},
				},
			},
		},
	},
	{
		Name:        "members",
		Description: "List active members of a study",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "channel",
				Description:  "Study channel (autocomplete)",
				Required:     true,
				Autocomplete: true,
			},
		},
	},
	{
		Name:                     "archive-all",
		Description:              "Archive all active studies",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "dry-run",
				Description: "Preview archive-all without moving channels or changing DB",
				Required:    false,
			},
		},
	},
	{
		Name:        "suggest",
		Description: "스터디를 제안합니다",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "visibility",
				Description: "제안자 표시 여부",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "익명으로 제안", Value: "anonymous"},
					{Name: "제안자 공개", Value: "public"},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "threshold",
				Description: "자동 개설 기준 인원 (기본 3명)",
				Required:    false,
				MinValue:    float64Ptr(1),
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "duration_days",
				Description: "제안 유효기간 일수 (기본 14일)",
				Required:    false,
				MinValue:    float64Ptr(1),
				MaxValue:    90,
			},
		},
	},
	{
		Name:                     "study-nudge",
		Description:              "open 상태의 스터디 제안을 공지 채널에 알립니다",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
	},
}
