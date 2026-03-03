package bot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParseArchiveCategoryNumber(t *testing.T) {
	testCases := []struct {
		name     string
		expected int
		ok       bool
	}{
		{name: "archive1", expected: 1, ok: true},
		{name: "archive12", expected: 12, ok: true},
		{name: "archive01", expected: 0, ok: false},
		{name: "archive0", expected: 0, ok: false},
		{name: "archive", expected: 0, ok: false},
		{name: "active", expected: 0, ok: false},
	}

	for _, tc := range testCases {
		actual, ok := parseArchiveCategoryNumber(tc.name)
		if ok != tc.ok {
			t.Fatalf("name=%q: expected ok=%v but got %v", tc.name, tc.ok, ok)
		}
		if actual != tc.expected {
			t.Fatalf("name=%q: expected number=%d but got %d", tc.name, tc.expected, actual)
		}
	}
}

func TestMergeReadOnlyOverwrite(t *testing.T) {
	currentAllow := int64(discordgo.PermissionViewChannel |
		discordgo.PermissionAddReactions |
		discordgo.PermissionSendMessages |
		discordgo.PermissionCreatePublicThreads)
	currentDeny := int64(discordgo.PermissionManageMessages)

	newAllow, newDeny := mergeReadOnlyOverwrite(currentAllow, currentDeny)

	if newAllow&discordgo.PermissionSendMessages != 0 {
		t.Fatal("expected send message allow to be removed")
	}
	if newAllow&discordgo.PermissionCreatePublicThreads != 0 {
		t.Fatal("expected create public threads allow to be removed")
	}
	if newAllow&discordgo.PermissionViewChannel == 0 {
		t.Fatal("expected unrelated allow bit to be preserved")
	}
	if newAllow&discordgo.PermissionAddReactions == 0 {
		t.Fatal("expected add reactions allow bit to be preserved")
	}

	if newDeny&archiveReadOnlyDenyMask != archiveReadOnlyDenyMask {
		t.Fatal("expected read-only deny mask to be fully set")
	}
	if newDeny&discordgo.PermissionManageMessages == 0 {
		t.Fatal("expected existing deny bits to be preserved")
	}
}

func TestPlanArchiveCategoryAssignments(t *testing.T) {
	existing := []archiveCategorySlot{
		{Number: 1, Name: "archive1", ChannelCount: 50},
		{Number: 3, Name: "archive3", ChannelCount: 49},
	}

	plan := planArchiveCategoryAssignments(existing, 3)

	if len(plan.Assignments) != 3 {
		t.Fatalf("expected 3 assignments, got %d", len(plan.Assignments))
	}
	if plan.Assignments[0] != "archive3" {
		t.Fatalf("expected first assignment to archive3, got %s", plan.Assignments[0])
	}
	if plan.Assignments[1] != "archive4" || plan.Assignments[2] != "archive4" {
		t.Fatalf("expected overflow assignments to archive4, got %v", plan.Assignments)
	}
	if len(plan.CreatedCategories) != 1 || plan.CreatedCategories[0] != "archive4" {
		t.Fatalf("expected archive4 creation plan, got %v", plan.CreatedCategories)
	}
	if plan.CategoryUseCounts["archive3"] != 1 || plan.CategoryUseCounts["archive4"] != 2 {
		t.Fatalf("unexpected category use counts: %+v", plan.CategoryUseCounts)
	}
}

func TestArchiveCategoryReservationCommitAndRelease(t *testing.T) {
	slot := archiveCategorySlot{ChannelCount: 10}
	reservation := &archiveCategoryReservation{slot: &slot}

	reservation.Release()
	if slot.ChannelCount != 10 {
		t.Fatalf("release before commit should not change count, got %d", slot.ChannelCount)
	}

	reservation.Commit()
	reservation.Commit()
	if slot.ChannelCount != 11 {
		t.Fatalf("commit should increase count once, got %d", slot.ChannelCount)
	}

	reservation.Release()
	reservation.Release()
	if slot.ChannelCount != 10 {
		t.Fatalf("release should decrease count once after commit, got %d", slot.ChannelCount)
	}
}
