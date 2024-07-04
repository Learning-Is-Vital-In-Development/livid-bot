package bot

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strings"
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
			Name:        "options",
			Description: "Command for demonstrating options",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "input",
					Description: "Input your name",
					Required:    true,
				},
			},
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
		"options": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, option := range options {
				optionMap[option.Name] = option
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Hello %s! 😃", optionMap["input"].StringValue()),
				},
			})
		},

		"submit": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
			for _, option := range options {
				optionMap[option.Name] = option
			}

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

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: markdown,
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
	discord.AddHandler(newMessage)
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

func newMessage(session *discordgo.Session, message *discordgo.MessageCreate) {

	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == session.State.User.ID {
		return
	}

	// respond to user message if it contains `!help` or `!bye`
	switch {
	case strings.Contains(message.Content, "!help"):
		session.ChannelMessageSend(message.ChannelID, "Hello World😃")
	case strings.Contains(message.Content, "!bye"):
		session.ChannelMessageSend(message.ChannelID, "Good Bye👋")
		// add more cases if required
	}
}
