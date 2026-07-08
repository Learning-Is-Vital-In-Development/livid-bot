package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"livid-bot/study"
)

func TestMemberHasRole(t *testing.T) {
	if !memberHasRole(&discordgo.Member{Roles: []string{"role-1"}}, "role-1") {
		t.Fatal("expected matching role")
	}
	if memberHasRole(&discordgo.Member{Roles: []string{"role-2"}}, "role-1") {
		t.Fatal("did not expect missing role to match")
	}
	if memberHasRole(nil, "role-1") {
		t.Fatal("did not expect nil member to match")
	}
	if memberHasRole(&discordgo.Member{Roles: []string{""}}, "") {
		t.Fatal("did not expect empty role id to match")
	}
}

func TestCanInviteStudyMember(t *testing.T) {
	if !canInviteStudyMember(&discordgo.Member{Permissions: discordgo.PermissionAdministrator}, "role-1") {
		t.Fatal("expected admin to invite")
	}
	if !canInviteStudyMember(&discordgo.Member{Roles: []string{"role-1"}}, "role-1") {
		t.Fatal("expected study role member to invite")
	}
	if canInviteStudyMember(&discordgo.Member{Roles: []string{"role-2"}}, "role-1") {
		t.Fatal("did not expect non-member to invite")
	}
	if canInviteStudyMember(nil, "role-1") {
		t.Fatal("did not expect nil member to invite")
	}
}

func TestInviteTargetDisplayName(t *testing.T) {
	member := &discordgo.Member{Nick: "스터디닉", User: &discordgo.User{ID: "user-1", Username: "discord-name"}}
	if got := inviteTargetDisplayName(member.User, member); got != "스터디닉" {
		t.Fatalf("expected member display name, got %q", got)
	}
	if got := inviteTargetDisplayName(&discordgo.User{ID: "user-1", Username: "discord-name"}, nil); got != "discord-name" {
		t.Fatalf("expected username fallback, got %q", got)
	}
	if got := inviteTargetDisplayName(&discordgo.User{ID: "user-1"}, nil); got != "user-1" {
		t.Fatalf("expected user id fallback, got %q", got)
	}
}

func TestInviteStudyMember(t *testing.T) {
	st := study.Study{ID: 10, Name: "분산 시스템", RoleID: "role-1"}
	t.Run("success", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}

		if err := inviteStudyMember(context.Background(), client, members, "guild-1", st, &discordgo.User{ID: "user-1"}, "도진"); err != nil {
			t.Fatalf("invite member: %v", err)
		}
		if members.addedUserID != "user-1" || members.addedUsername != "도진" || members.addedStudyID != st.ID {
			t.Fatalf("unexpected add call: %+v", members)
		}
		if client.addedGuildID != "guild-1" || client.addedUserID != "user-1" || client.addedRoleID != "role-1" {
			t.Fatalf("unexpected role call: %+v", client)
		}
	})

	t.Run("db failure skips role add", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{addErr: errors.New("db down")}
		client := &fakeStudyInviteRoleClient{}

		if err := inviteStudyMember(context.Background(), client, members, "guild-1", st, &discordgo.User{ID: "user-1"}, "도진"); err == nil {
			t.Fatal("expected error")
		}
		if client.addCalls != 0 {
			t.Fatalf("expected no role add call, got %d", client.addCalls)
		}
	})

	t.Run("role failure rolls back db membership", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{addErr: errors.New("discord down")}

		err := inviteStudyMember(context.Background(), client, members, "guild-1", st, &discordgo.User{ID: "user-1"}, "도진")
		if err == nil || !strings.Contains(err.Error(), "add study role") {
			t.Fatalf("expected role add error, got %v", err)
		}
		if members.removedUserID != "user-1" || members.removedStudyID != st.ID {
			t.Fatalf("expected rollback remove call, got %+v", members)
		}
	})

	t.Run("rollback failure keeps original role error", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{removeErr: errors.New("rollback down")}
		client := &fakeStudyInviteRoleClient{addErr: errors.New("discord down")}

		err := inviteStudyMember(context.Background(), client, members, "guild-1", st, &discordgo.User{ID: "user-1"}, "도진")
		if err == nil || !strings.Contains(err.Error(), "discord down") {
			t.Fatalf("expected original role error, got %v", err)
		}
	})
}

func TestStudyInviteHandler(t *testing.T) {
	st := study.Study{ID: 10, Name: "분산 시스템", ChannelID: "study-channel", RoleID: "role-1", Status: "active"}

	t.Run("study member can invite human", func(t *testing.T) {
		store := &fakeStudyInviteStudyStore{study: st}
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}
		responder := &fakeStudyInviteResponder{}
		handler := newStudyInviteHandlerWithDeps(store, members, responder, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "actor"}, Roles: []string{"role-1"}}, &discordgo.User{ID: "target", Username: "newbie"}))

		if members.addedUserID != "target" || client.addedUserID != "target" {
			t.Fatalf("expected invite side effects, members=%+v client=%+v", members, client)
		}
		if !strings.Contains(responder.editedContent, "<@target>") || !strings.Contains(responder.editedContent, "분산 시스템") {
			t.Fatalf("unexpected success response: %q", responder.editedContent)
		}
	})

	t.Run("admin can invite", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}
		handler := newStudyInviteHandlerWithDeps(&fakeStudyInviteStudyStore{study: st}, members, &fakeStudyInviteResponder{}, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "admin"}, Permissions: discordgo.PermissionAdministrator}, &discordgo.User{ID: "target", Username: "newbie"}))

		if members.addCalls != 1 || client.addCalls != 1 {
			t.Fatalf("expected admin invite side effects, members=%d roles=%d", members.addCalls, client.addCalls)
		}
	})

	t.Run("non member is denied", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}
		responder := &fakeStudyInviteResponder{}
		handler := newStudyInviteHandlerWithDeps(&fakeStudyInviteStudyStore{study: st}, members, responder, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "outsider"}, Roles: []string{"other-role"}}, &discordgo.User{ID: "target", Username: "newbie"}))

		if members.addCalls != 0 || client.addCalls != 0 {
			t.Fatalf("expected no side effects, members=%d roles=%d", members.addCalls, client.addCalls)
		}
		if !strings.Contains(responder.editedContent, "멤버 또는 관리자") {
			t.Fatalf("expected permission error, got %q", responder.editedContent)
		}
	})

	t.Run("outside study channel is denied", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}
		responder := &fakeStudyInviteResponder{}
		handler := newStudyInviteHandlerWithDeps(&fakeStudyInviteStudyStore{findErr: errors.New("not found")}, members, responder, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "actor"}, Permissions: discordgo.PermissionAdministrator}, &discordgo.User{ID: "target", Username: "newbie"}))

		if members.addCalls != 0 || client.addCalls != 0 {
			t.Fatalf("expected no side effects, members=%d roles=%d", members.addCalls, client.addCalls)
		}
		if !strings.Contains(responder.editedContent, "스터디 채널") {
			t.Fatalf("expected study channel error, got %q", responder.editedContent)
		}
	})

	t.Run("bot target is denied", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{}
		responder := &fakeStudyInviteResponder{}
		handler := newStudyInviteHandlerWithDeps(&fakeStudyInviteStudyStore{study: st}, members, responder, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "actor"}, Roles: []string{"role-1"}}, &discordgo.User{ID: "bot", Username: "helper", Bot: true}))

		if members.addCalls != 0 || client.addCalls != 0 {
			t.Fatalf("expected no side effects, members=%d roles=%d", members.addCalls, client.addCalls)
		}
		if !strings.Contains(responder.editedContent, "bot 계정") {
			t.Fatalf("expected bot error, got %q", responder.editedContent)
		}
	})

	t.Run("role failure reports error and rolls back", func(t *testing.T) {
		members := &fakeStudyInviteMemberStore{}
		client := &fakeStudyInviteRoleClient{addErr: errors.New("discord down")}
		responder := &fakeStudyInviteResponder{}
		handler := newStudyInviteHandlerWithDeps(&fakeStudyInviteStudyStore{study: st}, members, responder, client)

		handler(context.Background(), nil, newStudyInviteInteractionForTest(&discordgo.Member{User: &discordgo.User{ID: "actor"}, Roles: []string{"role-1"}}, &discordgo.User{ID: "target", Username: "newbie"}))

		if members.removedUserID != "target" {
			t.Fatalf("expected rollback, got %+v", members)
		}
		if !strings.Contains(responder.editedContent, "실패") {
			t.Fatalf("expected failure response, got %q", responder.editedContent)
		}
	})
}

type fakeStudyInviteStudyStore struct {
	study   study.Study
	findErr error
}

func (f *fakeStudyInviteStudyStore) FindByChannelID(_ context.Context, channelID string) (study.Study, error) {
	if f.findErr != nil {
		return study.Study{}, f.findErr
	}
	if channelID != f.study.ChannelID {
		return study.Study{}, errors.New("not found")
	}
	return f.study, nil
}

type fakeStudyInviteMemberStore struct {
	addErr    error
	removeErr error

	addCalls      int
	addedStudyID  int64
	addedUserID   string
	addedUsername string

	removeCalls    int
	removedStudyID int64
	removedUserID  string
}

func (f *fakeStudyInviteMemberStore) AddMember(_ context.Context, studyID int64, userID, username string) error {
	f.addCalls++
	f.addedStudyID = studyID
	f.addedUserID = userID
	f.addedUsername = username
	return f.addErr
}

func (f *fakeStudyInviteMemberStore) RemoveMember(_ context.Context, studyID int64, userID string) error {
	f.removeCalls++
	f.removedStudyID = studyID
	f.removedUserID = userID
	return f.removeErr
}

type fakeStudyInviteRoleClient struct {
	addErr error

	addCalls     int
	addedGuildID string
	addedUserID  string
	addedRoleID  string
}

func (f *fakeStudyInviteRoleClient) GuildMemberRoleAdd(guildID, userID, roleID string, options ...discordgo.RequestOption) error {
	f.addCalls++
	f.addedGuildID = guildID
	f.addedUserID = userID
	f.addedRoleID = roleID
	return f.addErr
}

type fakeStudyInviteResponder struct {
	deferCalls    int
	editCalls     int
	editedContent string
}

func (f *fakeStudyInviteResponder) deferEphemeral(context.Context, *discordgo.Session, *discordgo.InteractionCreate) error {
	f.deferCalls++
	return nil
}

func (f *fakeStudyInviteResponder) editOriginal(_ context.Context, _ *discordgo.Session, _ *discordgo.InteractionCreate, content string) error {
	f.editCalls++
	f.editedContent = content
	return nil
}

func newStudyInviteInteractionForTest(actor *discordgo.Member, target *discordgo.User) *discordgo.InteractionCreate {
	if actor != nil && actor.User == nil {
		actor.User = &discordgo.User{ID: "actor"}
	}
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:        "interaction-1",
		AppID:     "app-1",
		Token:     "token-1",
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   "guild-1",
		ChannelID: "study-channel",
		Member:    actor,
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "study-invite",
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:  "user",
					Type:  discordgo.ApplicationCommandOptionUser,
					Value: target.ID,
				},
			},
			Resolved: &discordgo.ApplicationCommandInteractionDataResolved{
				Users: map[string]*discordgo.User{target.ID: target},
			},
		},
	}}
}
