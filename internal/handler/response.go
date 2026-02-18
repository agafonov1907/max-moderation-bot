package handler

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/metrics"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *Handler) sendWarningWithMention(ctx context.Context, chatID int64, user schemes.User, reason string) {
	name := user.Name
	if name == "" {
		name = "User"
	}
	userLink := fmt.Sprintf("[%s](max://max.ru/%%%d%%)", name, user.UserId)
	text := fmt.Sprintf(messages.MsgProhibitedContent, userLink, reason)
	msg := maxbot.NewMessage()
	msg.SetChat(chatID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	respMsg, err := h.bot.Messages.SendWithResult(ctx, msg)
	if err != nil {
		h.logger.Error("Failed to send warning message", "chat_id", chatID, "user_id", user.UserId, "error", err)
	} else {
		h.logger.Info("Sent warning with mention", "chat_id", chatID, "user_id", user.UserId)
		metrics.IncBotAction("warning")

		if respMsg != nil && respMsg.Body.Mid != "" {
			if err := h.svc.ScheduleDeletion(ctx, chatID, respMsg.Body.Mid, 1*time.Minute); err != nil {
				h.logger.Error("Failed to schedule warning deletion", "error", err)
			}
		}
	}
}
func (h *Handler) deleteMessage(ctx context.Context, messageID string, reason string) error {
	if _, err := h.bot.Messages.DeleteMessage(ctx, messageID); err != nil {
		h.logger.Error("Failed to delete message", "message_id", messageID, "error", err)
		return err
	}
	h.logger.Info("Deleted message", "message_id", messageID, "reason", reason)
	metrics.IncDeletedMessages(reason)
	return nil
}

func (h *Handler) SendTemporaryMessage(ctx context.Context, chatID int64, text string, duration time.Duration) {
	msg := maxbot.NewMessage()
	msg.SetChat(chatID)
	msg.SetText(text)
	msg.SetFormat("markdown")

	respMsg, err := h.bot.Messages.SendWithResult(ctx, msg)
	if err != nil {
		h.logger.Error("Failed to send temporary message", "error", err)
		return
	}

	if respMsg != nil && respMsg.Body.Mid != "" {
		if err := h.svc.ScheduleDeletion(ctx, chatID, respMsg.Body.Mid, duration); err != nil {
			h.logger.Error("Failed to schedule deletion in DB", "error", err)
		}
	} else {
		h.logger.Warn("Message sent but ID is missing in response")
	}
}

func (h *Handler) SendAutoDeleteMessage(ctx context.Context, chatID int64, text string) {
	h.SendTemporaryMessage(ctx, chatID, text, 1*time.Minute)
}

func (h *Handler) sendGroupMessage(ctx context.Context, chatID int64, text string) {
	msg := maxbot.NewMessage()
	msg.SetChat(chatID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send group message", "chat_id", chatID, "error", err)
	} else {
		h.logger.Info("Sent group message", "chat_id", chatID)
	}
}
