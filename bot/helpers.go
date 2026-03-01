package bot

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func checkNilErr(e error) {
	if e != nil {
		log.Fatal(e.Error())
	}
}

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func boolPtr(v bool) *bool {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}
