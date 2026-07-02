package bot

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

type suggestionStore interface {
	CreateSuggestion(ctx context.Context, params db.CreateSuggestionParams) (*db.StudySuggestion, error)
}

type suggestionModalDiscordClient interface {
	suggestionDiscordClient
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error
	GuildChannels(guildID string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error)
	MessageReactionAdd(channelID, messageID, emojiID string, options ...discordgo.RequestOption) error
}

type suggestInteractionResponder interface {
	deferEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
	editOriginal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error
}

type discordSuggestResponder struct{}

func newSuggestHandler(suggestionRepo suggestionStore, suggestionChannelID string) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	suggestionChannelID = strings.TrimSpace(suggestionChannelID)
	if suggestionChannelID == "" {
		suggestionChannelID = suggestionChannelAuto
	}

	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(ctx, i, "start", "suggest command received")

		modalOpts := parseSuggestCommandOptions(i.ApplicationCommandData().Options)
		modalOpts.ChannelID = suggestionChannelID

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: buildSuggestModalCustomID(modalOpts),
				Title:    "스터디 제안",
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:  "title",
							Label:     "주제",
							Style:     discordgo.TextInputShort,
							Required:  true,
							MaxLength: 100,
						},
					}},
					discordgo.ActionsRow{Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:  "description",
							Label:     "한줄 설명",
							Style:     discordgo.TextInputShort,
							Required:  false,
							MaxLength: 200,
						},
					}},
				},
			},
		}, discordgo.WithContext(ctx)); err != nil {
			logCommand(ctx, i, "error", "failed to respond modal: %v", err)
		}
	}
}

func parseSuggestCommandOptions(options []*discordgo.ApplicationCommandInteractionDataOption) suggestionModalOptions {
	parsed := suggestionModalOptions{Visibility: suggestionVisibilityAnonymous, Threshold: db.SuggestionConfirmVoteThreshold, DurationDays: suggestionDefaultDurationDays}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		switch opt.Name {
		case "visibility":
			parsed.Visibility = opt.StringValue()
		case "threshold":
			parsed.Threshold = int(opt.IntValue())
		case "duration_days":
			parsed.DurationDays = int(opt.IntValue())
		}
	}
	if parsed.Threshold < 1 {
		parsed.Threshold = db.SuggestionConfirmVoteThreshold
	}
	if parsed.DurationDays < 1 || parsed.DurationDays > suggestionMaxDurationDays {
		parsed.DurationDays = suggestionDefaultDurationDays
	}
	if parsed.Visibility != suggestionVisibilityPublic {
		parsed.Visibility = suggestionVisibilityAnonymous
	}
	return parsed
}

func newSuggestModalHandler(suggestionRepo suggestionStore) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return newSuggestModalHandlerWithDeps(suggestionRepo, discordSuggestResponder{}, nil)
}

func newSuggestModalHandlerWithDeps(
	suggestionRepo suggestionStore,
	responder suggestInteractionResponder,
	client suggestionModalDiscordClient,
) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	if responder == nil {
		responder = discordSuggestResponder{}
	}

	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		activeClient := client
		if activeClient == nil {
			activeClient = s
		}
		handleSuggestModalSubmit(ctx, activeClient, responder, suggestionRepo, s, i)
	}
}

func handleSuggestModalSubmit(
	ctx context.Context,
	client suggestionModalDiscordClient,
	responder suggestInteractionResponder,
	suggestionRepo suggestionStore,
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
) {
	logCommand(ctx, i, "start", "suggest modal submit received")
	if err := responder.deferEphemeral(ctx, s, i); err != nil {
		logCommand(ctx, i, "error", "failed to defer suggest modal response: %v", err)
		return
	}

	modalOpts, err := parseSuggestModalCustomID(i.ModalSubmitData().CustomID)
	if err != nil {
		editSuggestDeferredError(ctx, responder, s, i, "제안 설정을 확인할 수 없습니다. 다시 시도해주세요.")
		return
	}
	channelID, err := resolveSuggestionChannelID(ctx, client, i.GuildID, modalOpts.ChannelID)
	if err != nil {
		logCommand(ctx, i, "error", "failed to resolve suggestion channel: %v", err)
		editSuggestDeferredError(ctx, responder, s, i, "제안 채널을 찾을 수 없습니다.")
		return
	}

	data := i.ModalSubmitData()
	title := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	description := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	expiresAt := suggestionExpiresAtFromDuration(time.Now(), modalOpts.DurationDays)
	proposerUserID, proposerDisplayName := suggestProposer(i)

	postRef, err := publishSuggestionMessage(ctx, client, channelID, title, description, suggestionPostOptions{
		Visibility:     modalOpts.Visibility,
		ProposerUserID: proposerUserID,
		Threshold:      modalOpts.Threshold,
		ExpiresAt:      expiresAt,
	})
	if err != nil {
		logCommand(ctx, i, "error", "failed to publish suggestion message: %v", err)
		editSuggestDeferredError(ctx, responder, s, i, "제안 메시지 게시에 실패했습니다.")
		return
	}

	suggestion, err := suggestionRepo.CreateSuggestion(ctx, db.CreateSuggestionParams{
		Title:               title,
		Description:         description,
		MessageID:           postRef.MessageID,
		ChannelID:           postRef.ChannelID,
		Visibility:          modalOpts.Visibility,
		ProposerUserID:      proposerUserID,
		ProposerDisplayName: proposerDisplayName,
		Threshold:           modalOpts.Threshold,
		ExpiresAt:           expiresAt,
	})
	if err != nil {
		if deleteErr := client.ChannelMessageDelete(postRef.ChannelID, postRef.MessageID, discordgo.WithContext(ctx)); deleteErr != nil {
			logCommand(ctx, i, "warn", "failed to delete suggestion message after DB error: %v", deleteErr)
		}
		if errors.Is(err, db.ErrSuggestionClosed) {
			editSuggestDeferredError(ctx, responder, s, i, "제안 기간이 마감되었습니다.")
			return
		}
		editSuggestDeferredError(ctx, responder, s, i, "제안 등록에 실패했습니다.")
		return
	}

	if err := client.MessageReactionAdd(postRef.ChannelID, postRef.MessageID, "🚀", discordgo.WithContext(ctx)); err != nil {
		logCommand(ctx, i, "warn", "failed to add reaction: %v", err)
	}

	logCommand(ctx, i, "done", "suggestion created id=%d", suggestion.ID)
	if err := responder.editOriginal(ctx, s, i, "제안이 등록되었습니다!"); err != nil {
		logCommand(ctx, i, "error", "failed to edit suggest modal response: %v", err)
	}
}

func resolveSuggestionChannelID(ctx context.Context, client suggestionModalDiscordClient, guildID, channelID string) (string, error) {
	if channelID != "" && channelID != suggestionChannelAuto {
		return channelID, nil
	}
	channels, err := client.GuildChannels(guildID, discordgo.WithContext(ctx))
	if err != nil {
		return "", err
	}
	target := findSuggestionDiscussionChannel(channels)
	if target == nil {
		return "", errors.New("suggestion discussion channel not found")
	}
	return target.ID, nil
}

func suggestProposer(i *discordgo.InteractionCreate) (string, string) {
	if i == nil {
		return "", ""
	}
	if i.Member != nil && i.Member.User != nil {
		name := i.Member.DisplayName()
		if name == "" {
			name = i.Member.User.Username
		}
		return i.Member.User.ID, name
	}
	if i.User != nil {
		return i.User.ID, i.User.Username
	}
	return "", ""
}

func editSuggestDeferredError(ctx context.Context, responder suggestInteractionResponder, s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(ctx, i, "error", "%s", message)
	if err := responder.editOriginal(ctx, s, i, message); err != nil {
		logCommand(ctx, i, "error", "failed to edit deferred suggest response: %v", err)
	}
}

func (discordSuggestResponder) deferEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	}, discordgo.WithContext(ctx))
}

func (discordSuggestResponder) editOriginal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}, discordgo.WithContext(ctx))
	return err
}
