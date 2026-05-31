package bot

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestStartInteractionSpanNamesCommandAndAddsDiscordAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:        "interaction-1",
		Type:      discordgo.InteractionApplicationCommand,
		GuildID:   "guild-1",
		ChannelID: "channel-1",
		Member:    &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: "studies",
		},
	}}

	_, span := startInteractionSpan(context.Background(), interaction)
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
	spanSnapshot := spans[0]
	if spanSnapshot.Name != "discord.command.studies" {
		t.Fatalf("expected command span name, got %q", spanSnapshot.Name)
	}
	attrs := map[string]string{}
	for _, attr := range spanSnapshot.Attributes {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}
	for key, want := range map[string]string{
		"messaging.system":       "discord",
		"discord.event_type":     "application_command",
		"discord.command.name":   "studies",
		"discord.interaction.id": "interaction-1",
		"discord.guild_id":       "guild-1",
		"discord.channel_id":     "channel-1",
		"discord.user_id":        "user-1",
	} {
		if attrs[key] != want {
			t.Fatalf("expected attr %s=%q, got %q", key, want, attrs[key])
		}
	}
}
