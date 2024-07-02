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
	discord.AddHandler(handleSlashCommand)

	discord.ApplicationCommandCreate(ApplicationID, GuildID, &discordgo.ApplicationCommand{
		Name:        "hello",
		Description: "Say Hello",
	})

	// open session
	err = discord.Open()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer discord.Close() // close session, after function termination

	// keep bot running untill there is NO os interruption (ctrl + C)
	fmt.Println("Bot running....")
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

// slash command
func handleSlashCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	switch interaction.ApplicationCommandData().Name {
	case "hello":
		session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Hello Slash Command! 😃",
			},
		})
	}
}
