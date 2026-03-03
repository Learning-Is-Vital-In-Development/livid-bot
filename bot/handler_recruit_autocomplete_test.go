package bot

import "testing"

func TestBuildRecruitBranchAutocompleteChoices(t *testing.T) {
	branches := []string{"26-1", "26-2", "26-3", "27-1"}

	filtered := buildRecruitBranchAutocompleteChoices(branches, "26-", 25)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered choices, got %d", len(filtered))
	}
	if filtered[0].Value != "26-1" {
		t.Fatalf("expected first choice value 26-1, got %v", filtered[0].Value)
	}

	limited := buildRecruitBranchAutocompleteChoices(branches, "", 2)
	if len(limited) != 2 {
		t.Fatalf("expected limited choices count=2, got %d", len(limited))
	}

	none := buildRecruitBranchAutocompleteChoices(branches, "99-", 25)
	if len(none) != 0 {
		t.Fatalf("expected no choices for unmatched query, got %d", len(none))
	}
}
