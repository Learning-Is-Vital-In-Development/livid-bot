package bot

import "github.com/bwmarrin/discordgo"

func handleHello(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logCommand(i, "start", "hello command invoked")
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Hello Command! 😃",
		},
	}); err != nil {
		logCommand(i, "error", "failed to respond hello command: %v", err)
		return
	}
	logCommand(i, "success", "hello response sent")
}
