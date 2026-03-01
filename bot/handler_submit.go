package bot

import (
	"net/http"

	"github.com/bwmarrin/discordgo"
)

func handleSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, option := range options {
		optionMap[option.Name] = option
	}
	link := optionMap["link"].StringValue()
	logCommand(i, "start", "submit command invoked link=%s", link)

	// Markdown link conversion
	markdown, err := ConvertLinkToMarkdown(link)
	if err != nil {
		respondError(s, i, "Error converting link to markdown")
		return
	}

	// attachment
	attachmentID := optionMap["screenshot"].Value.(string)
	attachmentUrl := i.ApplicationCommandData().Resolved.Attachments[attachmentID].URL

	res, resError := http.DefaultClient.Get(attachmentUrl)
	if resError != nil {
		logCommand(i, "error", "failed to fetch attachment url=%s err=%v", attachmentUrl, resError)
		respondError(s, i, "Could not get response")
		return
	}
	defer res.Body.Close()

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title: "New Submission! 🚀",
					URL:   link,
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:  "Challenge",
							Value: markdown,
						},
					},
					Image: &discordgo.MessageEmbedImage{
						URL: attachmentUrl,
					},
					Author: interactionAuthor(i),
					Color: 0x9400D3,
				},
			},
		},
	}); err != nil {
		logCommand(i, "error", "failed to respond submit command: %v", err)
		return
	}
	logCommand(i, "success", "submit processed link=%s attachment=%s", link, attachmentID)
}
