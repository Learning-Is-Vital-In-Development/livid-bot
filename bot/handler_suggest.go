package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

type suggestionStore interface {
	CreatePeriod(ctx context.Context, channelID string, closesAt time.Time) (*db.SuggestionPeriod, error)
	GetActivePeriod(ctx context.Context) (*db.SuggestionPeriod, error)
	CreateSuggestion(ctx context.Context, periodID int64, title, description, messageID, channelID string) (*db.StudySuggestion, error)
}

type suggestionModalDiscordClient interface {
	suggestionDiscordClient
	ChannelMessageDelete(channelID, messageID string, options ...discordgo.RequestOption) error
	MessageReactionAdd(channelID, messageID, emojiID string, options ...discordgo.RequestOption) error
}

type suggestInteractionResponder interface {
	deferEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
	editOriginal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error
	respondEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error
}

type discordSuggestResponder struct{}

func newSuggestStartHandler(suggestionRepo suggestionStore) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return newSuggestStartHandlerWithDeps(suggestionRepo, discordSuggestResponder{})
}

func newSuggestStartHandlerWithDeps(
	suggestionRepo suggestionStore,
	responder suggestInteractionResponder,
) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	if responder == nil {
		responder = discordSuggestResponder{}
	}

	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(ctx, i, "start", "suggest-start command received")
		if err := responder.deferEphemeral(ctx, s, i); err != nil {
			logCommand(ctx, i, "error", "failed to defer suggest-start command: %v", err)
			return
		}

		// Check 운영진 role
		roles, err := s.GuildRoles(i.GuildID, discordgo.WithContext(ctx))
		if err != nil {
			editSuggestDeferredError(ctx, responder, s, i, "서버 역할 조회에 실패했습니다.")
			return
		}
		var adminRoleID string
		for _, r := range roles {
			if r.Name == "운영진" {
				adminRoleID = r.ID
				break
			}
		}
		if adminRoleID == "" {
			editSuggestDeferredError(ctx, responder, s, i, "운영진 역할을 찾을 수 없습니다.")
			return
		}
		hasRole := false
		if i.Member != nil {
			for _, rid := range i.Member.Roles {
				if rid == adminRoleID {
					hasRole = true
					break
				}
			}
		}
		if !hasRole {
			editSuggestDeferredError(ctx, responder, s, i, "운영진만 사용할 수 있는 명령어입니다.")
			return
		}

		// Parse 마감일 option
		options := i.ApplicationCommandData().Options
		var closesAtStr string
		for _, opt := range options {
			if opt.Name == "deadline" {
				closesAtStr = opt.StringValue()
			}
		}
		closesAt, err := parseSuggestionDeadline(closesAtStr, time.Now())
		switch {
		case err == nil:
		case errors.Is(err, errSuggestionDeadlinePast):
			editSuggestDeferredError(ctx, responder, s, i, "마감일은 미래 날짜여야 합니다.")
			return
		default:
			editSuggestDeferredError(ctx, responder, s, i, fmt.Sprintf("마감일 형식이 올바르지 않습니다 (YYYY-MM-DD): %s", closesAtStr))
			return
		}

		// Find the fixed suggestion discussion channel.
		channels, err := s.GuildChannels(i.GuildID, discordgo.WithContext(ctx))
		if err != nil {
			editSuggestDeferredError(ctx, responder, s, i, "채널 목록 조회에 실패했습니다.")
			return
		}
		targetChannel := findSuggestionDiscussionChannel(channels)
		if targetChannel == nil {
			editSuggestDeferredError(ctx, responder, s, i, fmt.Sprintf("%s 채널을 찾을 수 없습니다.", suggestionDiscussionChannelName))
			return
		}
		targetChannelID := targetChannel.ID

		// Create period
		period, err := suggestionRepo.CreatePeriod(ctx, targetChannelID, closesAt)
		if err != nil {
			if errors.Is(err, db.ErrActiveSuggestionPeriodExists) {
				existing, getErr := suggestionRepo.GetActivePeriod(ctx)
				if getErr == nil && existing != nil {
					editSuggestDeferredError(ctx, responder, s, i, fmt.Sprintf("이미 활성 제안 기간이 있습니다 (마감: %s).", suggestionDateLabel(existing.ClosesAt)))
					return
				}
				editSuggestDeferredError(ctx, responder, s, i, "이미 활성 제안 기간이 있습니다.")
				return
			}
			editSuggestDeferredError(ctx, responder, s, i, "제안 기간 생성에 실패했습니다.")
			return
		}

		// Post announcement
		if _, err := publishSuggestionAnnouncement(ctx, s, targetChannelID, period.ClosesAt); err != nil {
			logCommand(ctx, i, "warn", "failed to send announcement: %v", err)
		}

		logCommand(ctx, i, "done", "suggestion period created id=%d closes_at=%s", period.ID, suggestionDateLabel(period.ClosesAt))
		if err := responder.editOriginal(ctx, s, i, fmt.Sprintf("✅ 스터디 제안 기간이 시작되었습니다. 마감: %s", suggestionDateLabel(period.ClosesAt))); err != nil {
			logCommand(ctx, i, "error", "failed to edit suggest-start response: %v", err)
		}
	}
}

func newSuggestHandler(suggestionRepo suggestionStore) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(ctx, i, "start", "suggest command received")

		period, err := suggestionRepo.GetActivePeriod(ctx)
		if err != nil {
			respondError(ctx, s, i, "제안 기간 조회에 실패했습니다.")
			return
		}
		if period == nil {
			respondError(ctx, s, i, "현재 활성 제안 기간이 없습니다.")
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "suggest_modal",
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

	period, err := suggestionRepo.GetActivePeriod(ctx)
	if err != nil {
		editSuggestDeferredError(ctx, responder, s, i, "제안 기간 조회에 실패했습니다.")
		return
	}
	if period == nil {
		editSuggestDeferredError(ctx, responder, s, i, "현재 활성 제안 기간이 없습니다.")
		return
	}
	if period.ChannelID == "" {
		editSuggestDeferredError(ctx, responder, s, i, "제안 채널 정보를 확인할 수 없습니다.")
		return
	}

	data := i.ModalSubmitData()
	title := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
	description := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

	targetChannelID := period.ChannelID
	postRef, err := publishSuggestionMessage(ctx, client, targetChannelID, title, description, 0)
	if err != nil {
		logCommand(ctx, i, "error", "failed to publish suggestion message: %v", err)
		editSuggestDeferredError(ctx, responder, s, i, "제안 메시지 게시에 실패했습니다.")
		return
	}

	suggestion, err := suggestionRepo.CreateSuggestion(ctx, period.ID, title, description, postRef.MessageID, postRef.ChannelID)
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

func (discordSuggestResponder) respondEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:         content,
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	}, discordgo.WithContext(ctx))
}
