package bot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(i, "error", "%s", message)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		logCommand(i, "error", "failed to respond error message: %v", err)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func logCommand(i *discordgo.InteractionCreate, stage, format string, args ...interface{}) {
	commandName := interactionCommandName(i)
	guildID := "unknown"
	userID := "unknown"
	if i != nil {
		if i.GuildID != "" {
			guildID = i.GuildID
		}
		userID = interactionUserID(i)
	}

	prefix := fmt.Sprintf("[cmd=%s stage=%s guild=%s user=%s]", commandName, stage, guildID, userID)
	if format == "" {
		log.Printf("%s", prefix)
		return
	}
	log.Printf("%s %s", prefix, fmt.Sprintf(format, args...))
}

func interactionCommandName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Type != discordgo.InteractionApplicationCommand && i.Type != discordgo.InteractionApplicationCommandAutocomplete {
		return "unknown"
	}
	data := i.ApplicationCommandData()
	if data.Name == "" {
		return "unknown"
	}
	return data.Name
}

func interactionAuthor(i *discordgo.InteractionCreate) *discordgo.MessageEmbedAuthor {
	var user *discordgo.User
	if i.Member != nil && i.Member.User != nil {
		user = i.Member.User
	} else if i.User != nil {
		user = i.User
	}
	if user == nil {
		return &discordgo.MessageEmbedAuthor{Name: "Unknown"}
	}
	return &discordgo.MessageEmbedAuthor{
		Name:    user.Username,
		URL:     "https://discord.com/users/" + user.ID,
		IconURL: user.AvatarURL(""),
	}
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "unknown"
	}
	if i.Member != nil && i.Member.User != nil && i.Member.User.ID != "" {
		return i.Member.User.ID
	}
	if i.User != nil && i.User.ID != "" {
		return i.User.ID
	}
	return "unknown"
}
