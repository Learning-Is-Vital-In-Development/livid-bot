package bot

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"net/http"
	"os"
	"os/signal"
)

var BotToken string
var ApplicationID string
var GuildID string

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "hello",
			Description: "Say Hello",
		},
		{
			Name:        "submit",
			Description: "Submit a link",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "link",
					Description: "Link to submit",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionAttachment,
					Name:        "screenshot",
					Description: "Screenshot of problem solution",
					Required:    true,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"hello": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Hello Command! 😃",
				},
			})
		},

		"submit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, option := range options {
				optionMap[option.Name] = option
			}

			// Markdown link conversion
			link := optionMap["link"].StringValue()
			markdown, err := ConvertLinkToMarkdown(link)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Error converting link to markdown",
					},
				})
				return
			}

			// attachment
			attachmentID := optionMap["screenshot"].Value.(string)
			attachmentUrl := i.ApplicationCommandData().Resolved.Attachments[attachmentID].URL

			res, resError := http.DefaultClient.Get(attachmentUrl)
			defer res.Body.Close()
			if resError != nil {
				log.Println(errors.New("could not get response from code explain bot"))
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Could not get response",
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: markdown,
					Files: []*discordgo.File{
						{
							Name:        "screenshot.png",
							ContentType: "image/png",
							Reader:      res.Body,
						},
					},
				},
			})
		},
	}
)

func checkNilErr(e error) {
	if e != nil {
		log.Fatal(e.Error())
	}
}

func Run() {

	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	// add a event handler
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, command := range commands {
		cmd, err := discord.ApplicationCommandCreate(ApplicationID, GuildID, command)
		checkNilErr(err)
		registeredCommands[i] = cmd
	}

	// open session
	err = discord.Open()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer discord.Close() // close session, after function termination

	// keep bot running untill there is NO os interruption (ctrl + C)
	fmt.Println("Bot running.... Press CTRL + C to exit")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
