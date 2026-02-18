package callbacks

import (
	"log/slog"

	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/service"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"go.opentelemetry.io/otel/trace"
)

type CallbackHandler struct {
	logger        *slog.Logger
	svc           service.Service
	bot           *maxbot.Api
	userStateRepo repository.UserStateRepository
	tracer        trace.Tracer
}

func NewCallbackHandler(logger *slog.Logger, svc service.Service, bot *maxbot.Api, userStateRepo repository.UserStateRepository, tracer trace.Tracer) *CallbackHandler {
	return &CallbackHandler{
		logger:        logger,
		svc:           svc,
		bot:           bot,
		userStateRepo: userStateRepo,
		tracer:        tracer,
	}
}
