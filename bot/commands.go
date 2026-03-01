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
}
