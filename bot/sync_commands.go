package bot

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func syncCommands(s *discordgo.Session, appID, guildID string) error {
	if _, err := s.ApplicationCommandBulkOverwrite(appID, guildID, commands); err != nil {
		return fmt.Errorf("bulk overwrite commands: %w", err)
	}
	return nil
}
