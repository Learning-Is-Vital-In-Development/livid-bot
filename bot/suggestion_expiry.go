package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const suggestionExpiryCheckInterval = time.Hour

type expiredSuggestionStore interface {
	ListExpiredOpenSuggestions(ctx context.Context) ([]*db.StudySuggestion, error)
	MarkExpired(ctx context.Context, suggestionID int64) error
}

type suggestionExpiryDiscordClient interface {
	ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

func startSuggestionExpiryNotifier(ctx context.Context, client suggestionExpiryDiscordClient, store expiredSuggestionStore) {
	if client == nil || store == nil {
		return
	}
	go func() {
		runSuggestionExpiryCheck(ctx, client, store)

		ticker := time.NewTicker(suggestionExpiryCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSuggestionExpiryCheck(ctx, client, store)
			}
		}
	}()
}

func runSuggestionExpiryCheck(ctx context.Context, client suggestionExpiryDiscordClient, store expiredSuggestionStore) {
	suggestions, err := store.ListExpiredOpenSuggestions(ctx)
	if err != nil {
		slog.Error("failed to list expired suggestions", "error", err)
		return
	}
	for _, suggestion := range suggestions {
		if suggestion == nil || suggestion.ID == 0 || suggestion.ChannelID == "" {
			continue
		}
		msg, err := client.ChannelMessageSend(suggestion.ChannelID, buildSuggestionExpiredMessage(), discordgo.WithContext(ctx))
		if err != nil {
			slog.Error("failed to send suggestion expiry notice", "suggestion_id", suggestion.ID, "channel_id", suggestion.ChannelID, "error", err)
			continue
		}
		messageID := ""
		if msg != nil {
			messageID = msg.ID
		}
		if err := store.MarkExpired(ctx, suggestion.ID); err != nil {
			slog.Error("failed to mark suggestion expired", "suggestion_id", suggestion.ID, "channel_id", suggestion.ChannelID, "message_id", messageID, "error", err)
			continue
		}
		slog.Info("sent suggestion expiry notice", "suggestion_id", suggestion.ID, "channel_id", suggestion.ChannelID, "message_id", messageID)
	}
}

func buildSuggestionExpiredMessage() string {
	return "⏰ 스터디 모집 기간이 종료되었습니다.\n\n필요하면 `/suggest`로 다시 신청하거나 운영진에게 문의해주세요."
}
