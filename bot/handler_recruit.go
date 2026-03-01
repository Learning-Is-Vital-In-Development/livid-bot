package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

var numberEmojis = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣", "🔟"}

func newRecruitHandler(studyRepo *db.StudyRepository, recruitRepo *db.RecruitRepository, reactionHandler *ReactionHandler) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
		for _, opt := range options {
			optionMap[opt.Name] = opt
		}

		channelID := optionMap["channel"].ChannelValue(nil).ID
		branch := optionMap["branch"].StringValue()
		fromStr := optionMap["from"].StringValue()
		toStr := optionMap["to"].StringValue()

		if !isValidBranch(branch) {
			respondError(s, i, "Invalid branch format. Use YY-Q with Q in 1~4 (e.g. 26-2).")
			return
		}

		// Validate dates
		fromDate, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			respondError(s, i, fmt.Sprintf("Invalid 'from' date format. Use YYYY-MM-DD. (got: %s)", fromStr))
			return
		}
		toDate, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			respondError(s, i, fmt.Sprintf("Invalid 'to' date format. Use YYYY-MM-DD. (got: %s)", toStr))
			return
		}
		if !toDate.After(fromDate) {
			respondError(s, i, "'to' date must be after 'from' date.")
			return
		}

		ctx := context.Background()
		studies, err := studyRepo.FindAllActiveByBranch(ctx, branch)
		if err != nil {
			log.Printf("Failed to find active studies for branch %s: %v", branch, err)
			respondError(s, i, "Failed to load studies.")
			return
		}

		if len(studies) == 0 {
			respondError(s, i, fmt.Sprintf("No active studies found in branch %s.", branch))
			return
		}

		if len(studies) > len(numberEmojis) {
			respondError(s, i, fmt.Sprintf("Too many active studies (%d). Maximum is %d.", len(studies), len(numberEmojis)))
			return
		}

		// Build message
		content := buildRecruitMessage(branch, studies, fromDate, toDate)

		// Send message to specified channel
		msg, err := s.ChannelMessageSend(channelID, content)
		if err != nil {
			log.Printf("Failed to send recruit message: %v", err)
			respondError(s, i, "Failed to send recruitment message.")
			return
		}

		// Add reactions
		for idx := range studies {
			if err := s.MessageReactionAdd(channelID, msg.ID, numberEmojis[idx]); err != nil {
				log.Printf("Failed to add reaction %s: %v", numberEmojis[idx], err)
			}
		}

		// Save to DB
		mappings := make([]study.RecruitMapping, len(studies))
		for idx, st := range studies {
			mappings[idx] = study.RecruitMapping{
				Emoji:   numberEmojis[idx],
				StudyID: st.ID,
				RoleID:  st.RoleID,
			}
		}

		if err := recruitRepo.SaveRecruitMessage(ctx, msg.ID, channelID, mappings); err != nil {
			log.Printf("Failed to save recruit message to DB: %v", err)
		}

		// Update in-memory mapping
		emojiRoleMap := make(map[string]emojiStudyInfo, len(studies))
		for idx, st := range studies {
			emojiRoleMap[numberEmojis[idx]] = emojiStudyInfo{
				RoleID:  st.RoleID,
				StudyID: st.ID,
			}
		}
		reactionHandler.Track(msg.ID, emojiRoleMap)

		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Recruitment message posted in <#%s>!", channelID),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
	}
}

func buildRecruitMessage(branch string, studies []study.Study, from, to time.Time) string {
	var b strings.Builder
	b.WriteString("@everyone 스터디 모집이 시작되었습니다!\n")
	b.WriteString(fmt.Sprintf("대상 분기: **%s**\n", branch))
	b.WriteString("참여를 원하시면 이모지로 참여 의사를 표현해주세요!\n\n")

	for idx, st := range studies {
		desc := ""
		if st.Description != "" {
			desc = fmt.Sprintf(" — %s", st.Description)
		}
		b.WriteString(fmt.Sprintf("%s **%s**%s\n", numberEmojis[idx], st.Name, desc))
	}

	b.WriteString(fmt.Sprintf("\n📅 모집 기간: %s ~ %s\n",
		from.Format("2006-01-02"),
		to.Format("2006-01-02")))
	b.WriteString("\n이모지 반응으로 스터디 역할이 자동 부여됩니다.")

	return b.String()
}
