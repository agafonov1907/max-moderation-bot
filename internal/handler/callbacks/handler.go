package callbacks

import (
	"context"
	"log/slog"
	"max-moderation-bot/internal/broadcast"
	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/service"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"go.opentelemetry.io/otel/trace"
)

type CallbackHandler struct {
	logger        *slog.Logger
	svc           service.Service
	svcBroadcast  *broadcast.Service // ✅ Сервис рассылки
	bot           *maxbot.Api
	userStateRepo repository.UserStateRepository
	tracer        trace.Tracer
	statsRepo     *repository.PostgresRepository // ✅ Для мониторинга
	settingsRepo  repository.SettingsRepository  // ✅ Для настроек чатов
}

func NewCallbackHandler(
	logger *slog.Logger,
	svc service.Service,
	svcBroadcast *broadcast.Service, // ✅ Параметр конструктора
	bot *maxbot.Api,
	userStateRepo repository.UserStateRepository,
	tracer trace.Tracer,
	statsRepo *repository.PostgresRepository, // ✅ НОВЫЙ ПАРАМЕТР
	settingsRepo repository.SettingsRepository, // ✅ НОВЫЙ ПАРАМЕТР
) *CallbackHandler {
	return &CallbackHandler{
		logger:        logger,
		svc:           svc,
		svcBroadcast:  svcBroadcast,
		bot:           bot,
		userStateRepo: userStateRepo,
		tracer:        tracer,
		statsRepo:     statsRepo,    // ✅ ИНИЦИАЛИЗАЦИЯ НОВОГО ПОЛЯ
		settingsRepo:  settingsRepo, // ✅ ИНИЦИАЛИЗАЦИЯ settingsRepo
	}
}

// sendText — вспомогательный метод для отправки текстовых сообщений
func (h *CallbackHandler) sendText(ctx context.Context, userID int64, text string) {
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text", "error", err)
	}
}
