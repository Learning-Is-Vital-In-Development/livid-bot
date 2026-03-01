package bot

import (
	"strings"
	"testing"
)

func TestBuildStudyStartSummary(t *testing.T) {
	tests := []struct {
		name         string
		started      []string
		archived     []string
		errors       []string
		wantContains []string
	}{
		{
			name:         "only started",
			started:      []string{"algo", "backend"},
			wantContains: []string{"Started **2** studies: algo, backend"},
		},
		{
			name:         "only archived",
			archived:     []string{"frontend"},
			wantContains: []string{"Archived **1** studies (< 3 members): frontend"},
		},
		{
			name:         "started and archived",
			started:      []string{"algo"},
			archived:     []string{"frontend"},
			wantContains: []string{"Started **1** studies: algo", "Archived **1** studies"},
		},
		{
			name:         "with errors",
			started:      []string{"algo"},
			errors:       []string{"backend: failed to load members"},
			wantContains: []string{"Started **1** studies: algo", "Errors: backend: failed to load members"},
		},
		{
			name:         "all empty",
			wantContains: []string{"No studies were processed."},
		},
		{
			name:         "archive with warning",
			archived:     []string{"frontend (role deletion failed)"},
			wantContains: []string{"Archived **1** studies (< 3 members): frontend (role deletion failed)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStudyStartSummary(tt.started, tt.archived, tt.errors)
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("expected summary to contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestBuildStudyStartSummaryTruncation(t *testing.T) {
	started := make([]string, 100)
	for i := range started {
		started[i] = "very-long-study-name-that-takes-up-space-" + string(rune('a'+i%26))
	}

	result := buildStudyStartSummary(started, nil, nil)
	if len([]rune(result)) > discordMessageLimit {
		t.Errorf("expected summary to be truncated to %d chars, got %d", discordMessageLimit, len([]rune(result)))
	}
}
