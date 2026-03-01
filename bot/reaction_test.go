package bot

import "testing"

func TestReactionHandlerTrackAndLookup(t *testing.T) {
	h := NewReactionHandler(nil)

	h.Track("msg-1", map[string]emojiStudyInfo{
		"1️⃣": {RoleID: "role-1", StudyID: 101},
	})

	info, ok := h.lookup("msg-1", "1️⃣")
	if !ok {
		t.Fatal("expected mapping to exist")
	}
	if info.RoleID != "role-1" || info.StudyID != 101 {
		t.Fatalf("unexpected mapping: %+v", info)
	}

	if _, ok := h.lookup("msg-1", "2️⃣"); ok {
		t.Fatal("expected unknown emoji mapping to be absent")
	}
	if _, ok := h.lookup("msg-unknown", "1️⃣"); ok {
		t.Fatal("expected unknown message mapping to be absent")
	}
}

func TestReactionHandlerTrackReplacesMessageMapping(t *testing.T) {
	h := NewReactionHandler(nil)

	h.Track("msg-1", map[string]emojiStudyInfo{
		"1️⃣": {RoleID: "role-1", StudyID: 101},
	})
	h.Track("msg-1", map[string]emojiStudyInfo{
		"2️⃣": {RoleID: "role-2", StudyID: 202},
	})

	if _, ok := h.lookup("msg-1", "1️⃣"); ok {
		t.Fatal("expected old mapping to be replaced for same message")
	}
	info, ok := h.lookup("msg-1", "2️⃣")
	if !ok {
		t.Fatal("expected new mapping to exist")
	}
	if info.RoleID != "role-2" || info.StudyID != 202 {
		t.Fatalf("unexpected mapping: %+v", info)
	}
}
