package bot

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"livid-bot/internal/telemetry"
)

const botTracerName = "livid-bot/bot"

func startInteractionSpan(ctx context.Context, i *discordgo.InteractionCreate) (context.Context, trace.Span) {
	tracer := otel.Tracer(botTracerName)
	spanName, eventType := interactionSpanName(i)
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "discord"),
		attribute.String("discord.event_type", eventType),
	}
	attrs = appendInteractionAttributes(attrs, i)
	return tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attrs...))
}

func startReactionSpan(ctx context.Context, eventName string, r *discordgo.MessageReaction) (context.Context, trace.Span) {
	tracer := otel.Tracer(botTracerName)
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "discord"),
		attribute.String("discord.event_type", eventName),
	}
	if r != nil {
		if r.GuildID != "" {
			attrs = append(attrs, attribute.String("discord.guild_id", r.GuildID))
		}
		if r.ChannelID != "" {
			attrs = append(attrs, attribute.String("discord.channel_id", r.ChannelID))
		}
		if r.MessageID != "" {
			attrs = append(attrs, attribute.String("discord.message_id", r.MessageID))
		}
		if r.UserID != "" {
			attrs = append(attrs, attribute.String("discord.user_id", r.UserID))
		}
		if r.Emoji.Name != "" {
			attrs = append(attrs, attribute.String("discord.emoji", r.Emoji.Name))
		}
	}
	return tracer.Start(ctx, "discord."+eventName, trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attrs...))
}

func startVoiceStateSpan(ctx context.Context, v *discordgo.VoiceStateUpdate) (context.Context, trace.Span) {
	tracer := otel.Tracer(botTracerName)
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "discord"),
		attribute.String("discord.event_type", "voice_state.update"),
	}
	if v != nil {
		if v.GuildID != "" {
			attrs = append(attrs, attribute.String("discord.guild_id", v.GuildID))
		}
		if v.UserID != "" {
			attrs = append(attrs, attribute.String("discord.user_id", v.UserID))
		}
		if v.ChannelID != "" {
			attrs = append(attrs, attribute.String("discord.voice.channel_id", v.ChannelID))
		}
	}
	return tracer.Start(ctx, "discord.voice_state.update", trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attrs...))
}

func interactionSpanName(i *discordgo.InteractionCreate) (spanName, eventType string) {
	if i == nil {
		return "discord.interaction.unknown", "unknown"
	}

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		return fmt.Sprintf("discord.command.%s", interactionCommandName(i)), "application_command"
	case discordgo.InteractionApplicationCommandAutocomplete:
		return fmt.Sprintf("discord.autocomplete.%s", interactionCommandName(i)), "autocomplete"
	case discordgo.InteractionModalSubmit:
		return "discord.modal.submit", "modal_submit"
	case discordgo.InteractionMessageComponent:
		return "discord.component", "message_component"
	default:
		return "discord.interaction.unknown", "unknown"
	}
}

func appendInteractionAttributes(attrs []attribute.KeyValue, i *discordgo.InteractionCreate) []attribute.KeyValue {
	if i == nil || i.Interaction == nil {
		return attrs
	}
	if commandName := interactionCommandName(i); commandName != "unknown" {
		attrs = append(attrs, attribute.String("discord.command.name", commandName))
	}
	if i.ID != "" {
		attrs = append(attrs, attribute.String("discord.interaction.id", i.ID))
	}
	if i.GuildID != "" {
		attrs = append(attrs, attribute.String("discord.guild_id", i.GuildID))
	}
	if i.ChannelID != "" {
		attrs = append(attrs, attribute.String("discord.channel_id", i.ChannelID))
	}
	if userID := interactionUserID(i); userID != "unknown" {
		attrs = append(attrs, attribute.String("discord.user_id", userID))
	}
	return attrs
}

func traceLogAttrs(ctx context.Context) []any {
	traceID, spanID := telemetry.TraceIDsFromContext(ctx)
	if traceID == "" || spanID == "" {
		return nil
	}
	return []any{"trace_id", traceID, "span_id", spanID}
}
