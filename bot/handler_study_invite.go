package bot

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
	"livid-bot/study"
)

type studyInviteStudyStore interface {
	FindByChannelID(ctx context.Context, channelID string) (study.Study, error)
}

type studyInviteMemberStore interface {
	AddMember(ctx context.Context, studyID int64, userID, username string) error
	RemoveMember(ctx context.Context, studyID int64, userID string) error
}

type studyInviteRoleClient interface {
	GuildMemberRoleAdd(guildID, userID, roleID string, options ...discordgo.RequestOption) error
}

type studyInviteResponder interface {
	deferEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error
	editOriginal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error
}

type discordStudyInviteResponder struct{}

func newStudyInviteHandler(studyRepo *db.StudyRepository, memberRepo *db.MemberRepository) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	return newStudyInviteHandlerWithDeps(studyRepo, memberRepo, discordStudyInviteResponder{}, nil)
}

func newStudyInviteHandlerWithDeps(
	studyStore studyInviteStudyStore,
	memberStore studyInviteMemberStore,
	responder studyInviteResponder,
	client studyInviteRoleClient,
) func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
	if responder == nil {
		responder = discordStudyInviteResponder{}
	}
	return func(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) {
		activeClient := client
		if activeClient == nil {
			activeClient = s
		}
		handleStudyInvite(ctx, activeClient, responder, studyStore, memberStore, s, i)
	}
}

func handleStudyInvite(
	ctx context.Context,
	client studyInviteRoleClient,
	responder studyInviteResponder,
	studyStore studyInviteStudyStore,
	memberStore studyInviteMemberStore,
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
) {
	logCommand(ctx, i, "start", "study-invite command received")
	if err := responder.deferEphemeral(ctx, s, i); err != nil {
		logCommand(ctx, i, "error", "failed to defer study-invite response: %v", err)
		return
	}

	st, err := studyStore.FindByChannelID(ctx, i.ChannelID)
	if err != nil || st.Status != "active" {
		editStudyInviteError(ctx, responder, s, i, "이 명령은 스터디 채널에서만 사용할 수 있어요.")
		return
	}
	if !canInviteStudyMember(i.Member, st.RoleID) {
		editStudyInviteError(ctx, responder, s, i, "이 스터디 멤버 또는 관리자만 초대할 수 있어요.")
		return
	}

	target, targetMember := studyInviteTarget(i, s)
	if target == nil || target.ID == "" {
		editStudyInviteError(ctx, responder, s, i, "초대할 사용자를 확인할 수 없어요.")
		return
	}
	if target.Bot {
		editStudyInviteError(ctx, responder, s, i, "bot 계정은 스터디원으로 추가하지 않아요.")
		return
	}

	if err := inviteStudyMember(ctx, client, memberStore, i.GuildID, st, target, inviteTargetDisplayName(target, targetMember)); err != nil {
		slog.Error("failed to invite study member", "study_id", st.ID, "study_name", st.Name, "user_id", target.ID, "error", err)
		editStudyInviteError(ctx, responder, s, i, "스터디원 추가에 실패했어요.")
		return
	}

	content := fmt.Sprintf("✅ <@%s> 님을 **%s** 스터디에 추가했어요.", target.ID, st.Name)
	if err := responder.editOriginal(ctx, s, i, content); err != nil {
		logCommand(ctx, i, "error", "failed to respond study-invite success: %v", err)
		return
	}
	logCommand(ctx, i, "success", "study member invited study_id=%d user=%s", st.ID, target.ID)
}

func inviteStudyMember(ctx context.Context, client studyInviteRoleClient, members studyInviteMemberStore, guildID string, st study.Study, user *discordgo.User, username string) error {
	if err := members.AddMember(ctx, st.ID, user.ID, username); err != nil {
		return fmt.Errorf("add study member: %w", err)
	}
	if err := client.GuildMemberRoleAdd(guildID, user.ID, st.RoleID, discordgo.WithContext(ctx)); err != nil {
		if rollbackErr := members.RemoveMember(ctx, st.ID, user.ID); rollbackErr != nil {
			slog.Warn("failed to rollback study member after role add failure", "study_id", st.ID, "user_id", user.ID, "error", rollbackErr)
		}
		return fmt.Errorf("add study role: %w", err)
	}
	return nil
}

func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil || roleID == "" {
		return false
	}
	for _, role := range member.Roles {
		if role == roleID {
			return true
		}
	}
	return false
}

func canInviteStudyMember(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	return member.Permissions&discordgo.PermissionAdministrator != 0 || memberHasRole(member, roleID)
}

func inviteTargetDisplayName(user *discordgo.User, member *discordgo.Member) string {
	if member != nil && member.Nick != "" {
		return member.Nick
	}
	if user != nil {
		if user.DisplayName() != "" {
			return user.DisplayName()
		}
		return user.ID
	}
	return ""
}

func studyInviteTarget(i *discordgo.InteractionCreate, s *discordgo.Session) (*discordgo.User, *discordgo.Member) {
	data := i.ApplicationCommandData()
	var userID string
	for _, opt := range data.Options {
		if opt == nil || opt.Name != "user" || opt.Type != discordgo.ApplicationCommandOptionUser {
			continue
		}
		if value, ok := opt.Value.(string); ok {
			userID = value
		}
		break
	}
	if userID == "" {
		return nil, nil
	}

	var member *discordgo.Member
	if data.Resolved != nil {
		member = data.Resolved.Members[userID]
		if user := data.Resolved.Users[userID]; user != nil {
			return user, member
		}
	}
	for _, opt := range data.Options {
		if opt != nil && opt.Name == "user" && opt.Type == discordgo.ApplicationCommandOptionUser {
			return opt.UserValue(s), member
		}
	}
	return nil, member
}

func editStudyInviteError(ctx context.Context, responder studyInviteResponder, s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	logCommand(ctx, i, "error", "%s", message)
	if err := responder.editOriginal(ctx, s, i, message); err != nil {
		logCommand(ctx, i, "error", "failed to edit study-invite error response: %v", err)
	}
}

func (discordStudyInviteResponder) deferEphemeral(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:           discordgo.MessageFlagsEphemeral,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	}, discordgo.WithContext(ctx))
}

func (discordStudyInviteResponder) editOriginal(ctx context.Context, s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content:         &content,
		AllowedMentions: &discordgo.MessageAllowedMentions{Users: []string{}},
	}, discordgo.WithContext(ctx))
	return err
}
