package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"max-moderation-bot/internal/config"
	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/service"

	"max-moderation-bot/internal/handler/callbacks"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	logger          *slog.Logger
	svc             service.Service
	bot             *maxbot.Api
	userStateRepo   repository.UserStateRepository
	tracer          trace.Tracer
	config          *config.Config
	callbackHandler *callbacks.CallbackHandler
}

func NewHandler(logger *slog.Logger, svc service.Service, bot *maxbot.Api, userStateRepo repository.UserStateRepository, cfg *config.Config) *Handler {
	return &Handler{
		logger:          logger,
		svc:             svc,
		bot:             bot,
		userStateRepo:   userStateRepo,
		tracer:          otel.Tracer("handler"),
		config:          cfg,
		callbackHandler: callbacks.NewCallbackHandler(logger, svc, bot, userStateRepo, otel.Tracer("callbacks")),
	}
}

func (h *Handler) HandleUpdate(ctx context.Context, upd schemes.UpdateInterface) {
	var span trace.Span
	if h.config.EnableTelemetry {
		ctx, span = h.tracer.Start(ctx, "HandleUpdate")
		defer span.End()
	}

	raw, _ := json.Marshal(upd)
	h.logger.Info("Raw update received", "json", string(raw))

	switch u := upd.(type) {
	case *schemes.MessageCreatedUpdate:
		if h.config.EnableTelemetry {
			span.SetAttributes(attribute.String("update_type", "message_created"))
		}
		h.handleMessageCreated(ctx, u)
	case *schemes.MessageCallbackUpdate:
		if h.config.EnableTelemetry {
			span.SetAttributes(attribute.String("update_type", "message_callback"))
		}
		h.handleCallback(ctx, u)
	case *schemes.BotStartedUpdate:
		if h.config.EnableTelemetry {
			span.SetAttributes(attribute.String("update_type", "bot_started"))
		}
		h.handleBotStarted(ctx, u)
	default:
		h.logger.Debug("Received unhandled update type", "type", fmt.Sprintf("%T", u))
	}
}
