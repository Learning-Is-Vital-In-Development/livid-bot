package bot

import (
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

func syncCommands(s *discordgo.Session, appID, guildID string) error {
	desired := make(map[string]*discordgo.ApplicationCommand, len(commands))
	for _, cmd := range commands {
		desired[cmd.Name] = cmd
	}

	registered, err := s.ApplicationCommands(appID, guildID)
	if err != nil {
		return fmt.Errorf("fetch registered commands: %w", err)
	}

	for _, cmd := range registered {
		if _, ok := desired[cmd.Name]; !ok {
			slog.Info("deleting stale command", "command", cmd.Name)
			if err := s.ApplicationCommandDelete(appID, guildID, cmd.ID); err != nil {
				return fmt.Errorf("delete stale command %q: %w", cmd.Name, err)
			}
		}
	}

	for _, cmd := range commands {
		if _, err := s.ApplicationCommandCreate(appID, guildID, cmd); err != nil {
			return fmt.Errorf("register command %q: %w", cmd.Name, err)
		}
	}

	return nil
}
