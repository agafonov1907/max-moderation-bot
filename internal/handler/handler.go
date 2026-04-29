package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"max-moderation-bot/internal/broadcast"
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
	broadcastSvc    *broadcast.Service
	statsRepo       *repository.PostgresRepository // ✅ Для мониторинга
	settingsRepo    repository.SettingsRepository  // ✅ Для настроек чатов
}

func NewHandler(
	logger *slog.Logger,
	svc service.Service,
	bot *maxbot.Api,
	userStateRepo repository.UserStateRepository,
	cfg *config.Config,
	broadcastSvc *broadcast.Service,
	statsRepo *repository.PostgresRepository, // ✅ 7-й параметр
	settingsRepo repository.SettingsRepository, // ✅ 8-й параметр
) *Handler {
	return &Handler{
		logger:        logger,
		svc:           svc,
		bot:           bot,
		userStateRepo: userStateRepo,
		tracer:        otel.Tracer("handler"),
		config:        cfg,
		broadcastSvc:  broadcastSvc,
		statsRepo:     statsRepo,    // ✅ Инициализация
		settingsRepo:  settingsRepo, // ✅ Инициализация
		callbackHandler: callbacks.NewCallbackHandler(
			logger,
			svc,
			broadcastSvc,
			bot,
			userStateRepo,
			otel.Tracer("callbacks"),
			statsRepo,    // ✅ 7-й аргумент
			settingsRepo, // ✅ 8-й аргумент
		),
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

// ============================================================================
// ДОБАВИТЬ В КОНЕЦ ФАЙЛА: internal/handler/handler.go
// ============================================================================

// handleBroadcastInputIfWaiting — вспомогательная функция для обработки текста рассылки
// (если пользователь ввел текст во время ожидания ввода для рассылки)
func (h *Handler) handleBroadcastInputIfWaiting(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	userID := upd.Message.Sender.UserId

	// 🔍 ОТЛАДОЧНЫЙ ЛОГ #1: Проверяем, вызвалась ли функция
	h.logger.Info("🔍 DEBUG [handler.go]: handleBroadcastInputIfWaiting START",
		"user_id", userID,
		"text_len", len(upd.Message.Body.Text),
	)

	// Проверяем состояние пользователя
	state, err := h.userStateRepo.GetState(userID)

	// 🔍 ОТЛАДОЧНЫЙ ЛОГ #2: Что в состоянии пользователя?
	h.logger.Info("🔍 DEBUG [handler.go]: UserState check",
		"user_id", userID,
		"state_exists", state != nil,
		"state_action", func() string {
			if state != nil {
				return state.Action
			}
			return "nil"
		}(),
		"state_metadata", func() string {
			if state != nil {
				return state.Metadata
			}
			return "nil"
		}(),
		"error", err,
	)

	if err != nil || state == nil {
		h.logger.Warn("⚠️ No state found, skipping broadcast input", "user_id", userID)
		return // Не ждем ввода
	}

	// 🔍 ОТЛАДОЧНЫЙ ЛОГ #3: Проверяем, подходит ли действие
	h.logger.Info("🔍 DEBUG [handler.go]: Checking action prefix",
		"action", state.Action,
		"prefix_match", strings.HasPrefix(state.Action, "broadcast_wait_text"),
	)

	// Если ждем текст для рассылки — передаем в обработчик
	if strings.HasPrefix(state.Action, "broadcast_wait_text") {
		h.logger.Info("✅ DEBUG [handler.go]: Calling ProcessBroadcastInput", "user_id", userID)
		h.callbackHandler.ProcessBroadcastInput(ctx, userID, upd.Message.Body.Text)
	} else {
		h.logger.Warn("⚠️ DEBUG [handler.go]: Action does not match broadcast_wait_text", "action", state.Action)
	}
}
