package bot

import "github.com/bwmarrin/discordgo"

var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "hello",
		Description: "Say Hello",
	},
	{
		Name:        "submit",
		Description: "Submit a link",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionAttachment,
				Name:        "screenshot",
				Description: "Screenshot of problem solution",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "link",
				Description: "Link to submit",
				Required:    true,
			},
		},
	},
	{
		Name:                     "create-study",
		Description:              "Create a new study channel and role",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "branch",
				Description: "Study branch in YY-Q format (Q: 1~4). ex) 26-2",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Study name",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "description",
				Description: "Study description or reference link",
				Required:    false,
			},
		},
	},
	{
		Name:                     "recruit",
		Description:              "Post a recruitment message for active studies",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel to post the recruitment message",
				Required:    true,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "branch",
				Description:  "Target branch in YY-Q format (Q: 1~4). ex) 26-2",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "from",
				Description: "Recruitment start date (YYYY-MM-DD)",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "to",
				Description: "Recruitment end date (YYYY-MM-DD)",
				Required:    true,
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
		Name:                     "study-start",
		Description:              "Close recruitment and start studies for a branch",
		DefaultMemberPermissions: int64Ptr(discordgo.PermissionAdministrator),
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "branch",
				Description:  "Target branch (YY-Q)",
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
}
