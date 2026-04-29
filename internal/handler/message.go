package handler

import (
	"context"
	"max-moderation-bot/internal/metrics"
	"time"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel/attribute"
)

func (h *Handler) handleMessageCreated(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	start := time.Now()
	defer func() {
		metrics.ObserveUpdateProcessing("message_created", time.Since(start).Seconds(), nil)
	}()

	ctx, span := h.tracer.Start(ctx, "handleMessageCreated")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("chat_id", upd.Message.Recipient.ChatId),
		attribute.Int64("user_id", upd.Message.Sender.UserId),
	)

	// 🔍 ОТЛАДОЧНЫЙ ЛОГ: входящее сообщение
	h.logger.Info("🔍 DEBUG [message.go]: handleMessageCreated",
		"user_id", upd.Message.Sender.UserId,
		"chat_id", upd.Message.Recipient.ChatId,
		"text", upd.Message.Body.Text,
		"text_len", len(upd.Message.Body.Text),
		"is_private", upd.Message.Recipient.ChatId > 0,
	)

	h.logger.Debug("Dispatching message",
		"chat_id", upd.Message.Recipient.ChatId,
		"sender_id", upd.Message.Sender.UserId,
	)

	isPrivateChat := upd.Message.Recipient.ChatId > 0

	if isPrivateChat {
		// 🔍 ЛОГ: направляем в приватный обработчик
		h.logger.Info("🔍 DEBUG [message.go]: Routing to handlePrivateMessage",
			"user_id", upd.Message.Sender.UserId,
		)
		h.handlePrivateMessage(ctx, upd) // ← Здесь уже есть вызов handleBroadcastInputIfWaiting
	} else {
		// 🔍 ЛОГ: направляем в групповой обработчик
		h.logger.Info("🔍 DEBUG [message.go]: Routing to handleGroupMessage",
			"chat_id", upd.Message.Recipient.ChatId,
		)
		h.handleGroupMessage(ctx, upd)
	}
}
