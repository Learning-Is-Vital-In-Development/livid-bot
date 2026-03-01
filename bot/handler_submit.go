package bot

import (
	"errors"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

func handleSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options

	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, option := range options {
		optionMap[option.Name] = option
	}

	// Markdown link conversion
	link := optionMap["link"].StringValue()
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
		log.Println(errors.New("could not get response from code explain bot"))
		respondError(s, i, "Could not get response")
		return
	}
	defer res.Body.Close()

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
					Author: &discordgo.MessageEmbedAuthor{
						Name:    i.Member.User.Username,
						URL:     "https://discord.com/users/" + i.Member.User.ID,
						IconURL: i.Member.User.AvatarURL(""),
					},
					Color: 0x9400D3,
				},
			},
		},
	})
}
