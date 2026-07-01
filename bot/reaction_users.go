package bot

import (
	"context"

	"github.com/bwmarrin/discordgo"
)

const discordReactionPageLimit = 100

type reactionUserClient interface {
	MessageReactions(channelID, messageID, emojiID string, limit int, beforeID, afterID string, options ...discordgo.RequestOption) ([]*discordgo.User, error)
}

func botUserID(s *discordgo.Session) string {
	if s == nil || s.State == nil || s.State.User == nil {
		return ""
	}
	return s.State.User.ID
}

func reactionDisplayName(user *discordgo.User) string {
	if user == nil {
		return ""
	}
	if name := user.DisplayName(); name != "" {
		return name
	}
	if user.Username != "" {
		return user.Username
	}
	return user.ID
}

func fetchAllReactionUsers(ctx context.Context, client reactionUserClient, channelID, messageID, emoji string) ([]*discordgo.User, error) {
	var allUsers []*discordgo.User
	afterID := ""

	for {
		users, err := client.MessageReactions(
			channelID,
			messageID,
			emoji,
			discordReactionPageLimit,
			"",
			afterID,
			discordgo.WithContext(ctx),
		)
		if err != nil {
			return nil, err
		}
		if len(users) == 0 {
			return allUsers, nil
		}

		allUsers = append(allUsers, users...)
		if len(users) < discordReactionPageLimit {
			return allUsers, nil
		}

		last := users[len(users)-1]
		if last == nil || last.ID == "" || last.ID == afterID {
			return allUsers, nil
		}
		afterID = last.ID
	}
}
