package bot

import (
	"context"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"livid-bot/db"
)

const recruitBranchAutocompleteMaxChoices = 25

func newRecruitBranchAutocompleteHandler(studyRepo *db.StudyRepository) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		ctx := context.Background()
		branches, err := studyRepo.FindDistinctActiveBranches(ctx)
		if err != nil {
			log.Printf("Failed to load active branches for recruit autocomplete: %v", err)
			respondRecruitBranchAutocomplete(s, i, nil)
			return
		}

		query := focusedStringOptionValue(i.ApplicationCommandData().Options, "branch")
		choices := buildRecruitBranchAutocompleteChoices(branches, query, recruitBranchAutocompleteMaxChoices)
		respondRecruitBranchAutocomplete(s, i, choices)
	}
}

func buildRecruitBranchAutocompleteChoices(branches []string, query string, limit int) []*discordgo.ApplicationCommandOptionChoice {
	if limit <= 0 {
		return nil
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, min(limit, len(branches)))
	for _, branch := range branches {
		if normalizedQuery != "" && !strings.Contains(strings.ToLower(branch), normalizedQuery) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  branch,
			Value: branch,
		})
		if len(choices) >= limit {
			break
		}
	}
	return choices
}

func respondRecruitBranchAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}); err != nil {
		log.Printf("Failed to respond recruit branch autocomplete: %v", err)
	}
}
