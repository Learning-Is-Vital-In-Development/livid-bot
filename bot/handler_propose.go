package bot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newProposeStartHandler(proposalRepo *db.ProposalRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(i, "start", "propose-start command received")

		// Check 운영진 role
		roles, err := s.GuildRoles(i.GuildID)
		if err != nil {
			respondError(s, i, "서버 역할 조회에 실패했습니다.")
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
			respondError(s, i, "운영진 역할을 찾을 수 없습니다.")
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
			respondError(s, i, "운영진만 사용할 수 있는 명령어입니다.")
			return
		}

		// Parse 마감일 option
		options := i.ApplicationCommandData().Options
		var closesAtStr string
		for _, opt := range options {
			if opt.Name == "마감일" {
				closesAtStr = opt.StringValue()
			}
		}
		closesAt, err := parseProposalDeadline(closesAtStr, time.Now())
		switch {
		case err == nil:
		case errors.Is(err, errProposalDeadlinePast):
			respondError(s, i, "마감일은 미래 날짜여야 합니다.")
			return
		default:
			respondError(s, i, fmt.Sprintf("마감일 형식이 올바르지 않습니다 (YYYY-MM-DD): %s", closesAtStr))
			return
		}

		ctx := context.Background()

		// Find 운영진-자유채팅 channel
		channels, err := s.GuildChannels(i.GuildID)
		if err != nil {
			respondError(s, i, "채널 목록 조회에 실패했습니다.")
			return
		}
		var targetChannelID string
		for _, ch := range channels {
			if ch.Name == "운영진-자유채팅" {
				targetChannelID = ch.ID
				break
			}
		}
		if targetChannelID == "" {
			respondError(s, i, "운영진-자유채팅 채널을 찾을 수 없습니다.")
			return
		}

		// Create period
		period, err := proposalRepo.CreatePeriod(ctx, targetChannelID, closesAt)
		if err != nil {
			if errors.Is(err, db.ErrActiveProposalPeriodExists) {
				existing, getErr := proposalRepo.GetActivePeriod(ctx)
				if getErr == nil && existing != nil {
					respondError(s, i, fmt.Sprintf("이미 활성 제안 기간이 있습니다 (마감: %s).", proposalDateLabel(existing.ClosesAt)))
					return
				}
				respondError(s, i, "이미 활성 제안 기간이 있습니다.")
				return
			}
			respondError(s, i, "제안 기간 생성에 실패했습니다.")
			return
		}

		// Post announcement
		content := buildProposalAnnouncement(period.ClosesAt)
		if _, err := s.ChannelMessageSend(targetChannelID, content); err != nil {
			logCommand(i, "warn", "failed to send announcement: %v", err)
		}

		logCommand(i, "done", "proposal period created id=%d closes_at=%s", period.ID, proposalDateLabel(period.ClosesAt))
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("✅ 스터디 제안 기간이 시작되었습니다. 마감: %s", proposalDateLabel(period.ClosesAt)),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond: %v", err)
		}
	}
}

func newProposeHandler(proposalRepo *db.ProposalRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(i, "start", "propose command received")

		ctx := context.Background()
		period, err := proposalRepo.GetActivePeriod(ctx)
		if err != nil {
			respondError(s, i, "제안 기간 조회에 실패했습니다.")
			return
		}
		if period == nil {
			respondError(s, i, "현재 활성 제안 기간이 없습니다.")
			return
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: "propose_modal",
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
		}); err != nil {
			logCommand(i, "error", "failed to respond modal: %v", err)
		}
	}
}

func newProposeModalHandler(proposalRepo *db.ProposalRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(i, "start", "propose modal submit received")

		ctx := context.Background()
		period, err := proposalRepo.GetActivePeriod(ctx)
		if err != nil {
			respondError(s, i, "제안 기간 조회에 실패했습니다.")
			return
		}
		if period == nil {
			respondError(s, i, "현재 활성 제안 기간이 없습니다.")
			return
		}
		if period.ChannelID == "" {
			respondError(s, i, "제안 채널 정보를 확인할 수 없습니다.")
			return
		}

		data := i.ModalSubmitData()
		title := data.Components[0].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value
		description := data.Components[1].(*discordgo.ActionsRow).Components[0].(*discordgo.TextInput).Value

		targetChannelID := period.ChannelID
		content := buildProposalMessage(title, description, 0)

		msg, err := s.ChannelMessageSend(targetChannelID, content)
		if err != nil {
			logCommand(i, "error", "failed to send proposal message: %v", err)
			respondError(s, i, "제안 메시지 게시에 실패했습니다.")
			return
		}

		proposal, err := proposalRepo.CreateProposal(ctx, period.ID, title, description, msg.ID, targetChannelID)
		if err != nil {
			if deleteErr := s.ChannelMessageDelete(targetChannelID, msg.ID); deleteErr != nil {
				logCommand(i, "warn", "failed to delete proposal message after DB error: %v", deleteErr)
			}
			if errors.Is(err, db.ErrProposalClosed) {
				respondError(s, i, "제안 기간이 마감되었습니다.")
				return
			}
			respondError(s, i, "제안 등록에 실패했습니다.")
			return
		}

		if err := s.MessageReactionAdd(targetChannelID, msg.ID, "🚀"); err != nil {
			logCommand(i, "warn", "failed to add reaction: %v", err)
		}

		logCommand(i, "done", "proposal created id=%d", proposal.ID)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "제안이 등록되었습니다!",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond: %v", err)
		}
	}
}
