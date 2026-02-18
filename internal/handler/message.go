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

	h.logger.Debug("Dispatching message",
		"chat_id", upd.Message.Recipient.ChatId,
		"sender_id", upd.Message.Sender.UserId,
	)

	isPrivateChat := upd.Message.Recipient.ChatId > 0

	if isPrivateChat {
		h.handlePrivateMessage(ctx, upd)
	} else {
		h.handleGroupMessage(ctx, upd)
	}
}
