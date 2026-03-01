package bot

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

var BotToken string
var ApplicationID string
var GuildID string

var commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
	"hello":  handleHello,
	"submit": handleSubmit,
}

func Run() {
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

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

	err = discord.Open()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer discord.Close()

	fmt.Println("Bot running.... Press CTRL + C to exit")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
