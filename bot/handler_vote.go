package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

func newVoteHandler(proposalRepo *db.ProposalRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(i, "start", "vote command received")

		ctx := context.Background()
		period, err := proposalRepo.GetActivePeriod(ctx)
		if err != nil {
			respondError(s, i, "제안 기간 조회에 실패했습니다.")
			return
		}
		if period == nil {
			respondError(s, i, "활성 제안 기간이 없습니다.")
			return
		}

		proposals, err := proposalRepo.ListProposals(ctx, period.ID)
		if err != nil {
			respondError(s, i, "제안 목록 조회에 실패했습니다.")
			return
		}
		if len(proposals) == 0 {
			respondError(s, i, "등록된 제안이 없습니다.")
			return
		}

		options := make([]discordgo.SelectMenuOption, 0, len(proposals))
		for _, p := range proposals {
			label := p.Title
			if len(label) > 100 {
				label = label[:100]
			}
			options = append(options, discordgo.SelectMenuOption{
				Label: label,
				Value: strconv.FormatInt(p.ID, 10),
			})
		}

		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "투표할 스터디 주제를 선택해주세요:",
				Flags:   discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							&discordgo.SelectMenu{
								CustomID:    "vote_select",
								Placeholder: "주제 선택",
								Options:     options,
							},
						},
					},
				},
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond vote menu: %v", err)
		}
	}
}

func newVoteSelectHandler(proposalRepo *db.ProposalRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logCommand(i, "start", "vote select received")

		data := i.MessageComponentData()
		if len(data.Values) == 0 {
			respondError(s, i, "선택된 값이 없습니다.")
			return
		}
		proposalID, err := strconv.ParseInt(data.Values[0], 10, 64)
		if err != nil {
			respondError(s, i, "잘못된 제안 ID입니다.")
			return
		}

		userID := ""
		if i.Member != nil && i.Member.User != nil {
			userID = i.Member.User.ID
		}
		if userID == "" {
			respondError(s, i, "사용자 정보를 확인할 수 없습니다.")
			return
		}

		ctx := context.Background()
		voted, newCount, err := proposalRepo.ToggleVote(ctx, proposalID, userID)
		if err != nil {
			respondError(s, i, "투표 처리에 실패했습니다.")
			return
		}

		proposal, err := proposalRepo.GetProposal(ctx, proposalID)
		if err != nil || proposal == nil {
			respondError(s, i, "제안 조회에 실패했습니다.")
			return
		}

		// Update the channel message vote count
		if proposal.MessageID != "" && proposal.ChannelID != "" {
			msg, fetchErr := s.ChannelMessage(proposal.ChannelID, proposal.MessageID)
			if fetchErr == nil {
				oldContent := msg.Content
				newContent := updateVoteLine(oldContent, newCount)
				if _, editErr := s.ChannelMessageEdit(proposal.ChannelID, proposal.MessageID, newContent); editErr != nil {
					logCommand(i, "warn", "failed to edit proposal message: %v", editErr)
				}
			} else {
				logCommand(i, "warn", "failed to fetch proposal message: %v", fetchErr)
			}
		}

		// Auto-confirm at 3 votes
		if voted && newCount >= 3 && !proposal.Confirmed {
			if confirmErr := proposalRepo.MarkConfirmed(ctx, proposalID); confirmErr != nil {
				logCommand(i, "error", "failed to mark confirmed: %v", confirmErr)
			} else if proposal.ChannelID != "" {
				confirmMsg := fmt.Sprintf("🎉 스터디 개설 확정!\n**%s** 이 3표를 달성했습니다.\n운영자로 참여하실 분은 DM 또는 댓글로 알려주세요!", proposal.Title)
				if _, sendErr := s.ChannelMessageSend(proposal.ChannelID, confirmMsg); sendErr != nil {
					logCommand(i, "warn", "failed to send confirm message: %v", sendErr)
				}
			}
		}

		responseMsg := "투표 완료!"
		if !voted {
			responseMsg = "투표 취소!"
		}

		logCommand(i, "done", "vote toggled proposal=%d voted=%v count=%d", proposalID, voted, newCount)
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: responseMsg,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		}); err != nil {
			logCommand(i, "error", "failed to respond: %v", err)
		}
	}
}

// updateVoteLine replaces the last "🚀 N표" line in a message with the new count.
func updateVoteLine(content string, newCount int) string {
	lines := strings.Split(content, "\n")
	for idx := len(lines) - 1; idx >= 0; idx-- {
		if strings.HasPrefix(lines[idx], "🚀") {
			lines[idx] = fmt.Sprintf("🚀 %d표", newCount)
			return strings.Join(lines, "\n")
		}
	}
	return content + fmt.Sprintf("\n🚀 %d표", newCount)
}
