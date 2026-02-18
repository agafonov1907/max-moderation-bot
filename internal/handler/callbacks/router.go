package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel/attribute"
)

func (h *CallbackHandler) Handle(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	payload := upd.Callback.Payload
	chatID := upd.Callback.GetChatID()
	if chatID == 0 && upd.Message != nil {
		chatID = upd.Message.Recipient.ChatId
	}
	ctx, span := h.tracer.Start(ctx, "handleCallback")
	defer span.End()

	span.SetAttributes(
		attribute.String("payload", payload),
		attribute.Int64("chat_id", chatID),
		attribute.Int64("user_id", upd.Callback.GetUserID()),
	)

	h.logger.Info("Received callback", "payload", payload, "chat_id", chatID, "user_id", upd.Callback.GetUserID())

	if upd.Message != nil {
		go func() {
			bgCtx := context.Background()
			if _, err := h.bot.Messages.DeleteMessage(bgCtx, upd.Message.Body.Mid); err != nil {
				h.logger.Warn("Failed to delete callback message", "error", err)
			}
		}()
	}

	switch {
	case payload == "add_group":
		h.handleAddGroup(ctx, upd.Callback.User.UserId)
	case strings.HasPrefix(payload, "my_groups"):
		var page int
		if _, err := fmt.Sscanf(payload, "my_groups_%d", &page); err == nil {
			h.handleMyGroups(ctx, upd.Callback.User.UserId, page)
		} else {
			h.handleMyGroups(ctx, upd.Callback.User.UserId, 1)
		}
	case payload == "main_menu":
		h.sendMainMenu(ctx, upd.Callback.User.UserId)
	case strings.HasPrefix(payload, "manage_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "manage_%d", &groupID); err == nil {
			h.HandleManageGroup(ctx, groupID, upd.Callback.User.UserId)
		} else {
			h.logger.Error("Invalid manage payload", "payload", payload)
		}
	case strings.HasPrefix(payload, "toggle_"):
		h.handleToggleSetting(ctx, payload, upd.Callback.User.UserId)
	case strings.HasPrefix(payload, "prompt_words_"):
		h.handlePromptInput(ctx, payload, upd.Callback.User.UserId, "add_words")
	case strings.HasPrefix(payload, "prompt_domains_"):
		h.handlePromptInput(ctx, payload, upd.Callback.User.UserId, "add_domains")
	case strings.HasPrefix(payload, "prompt_import_words_"):
		h.handlePromptInput(ctx, payload, upd.Callback.User.UserId, "import_words")
	case strings.HasPrefix(payload, "clear_words_"):
		h.handleClearBlocked(ctx, payload, upd.Callback.User.UserId, "clear_words")
	case strings.HasPrefix(payload, "clear_domains_"):
		h.handleClearBlocked(ctx, payload, upd.Callback.User.UserId, "clear_domains")
	case strings.HasPrefix(payload, "lm_"):
		var groupID int64
		var page int
		if _, err := fmt.Sscanf(payload, "lm_%d_%d", &groupID, &page); err == nil {
			h.handleListMutes(ctx, groupID, upd.Callback.User.UserId, page)
		} else if _, err := fmt.Sscanf(payload, "lm_%d", &groupID); err == nil {
			h.handleListMutes(ctx, groupID, upd.Callback.User.UserId, 1)
		}
	case strings.HasPrefix(payload, "um_"):
		var groupID, targetUserID int64
		var page int
		if _, err := fmt.Sscanf(payload, "um_%d_%d_%d", &groupID, &targetUserID, &page); err == nil {
			h.handleUnmute(ctx, groupID, upd.Callback.User.UserId, targetUserID, page)
		}
	case strings.HasPrefix(payload, "vm_"):
		var groupID, targetUserID int64
		var page int
		if _, err := fmt.Sscanf(payload, "vm_%d_%d_%d", &groupID, &targetUserID, &page); err == nil {
			h.handleViewMute(ctx, groupID, upd.Callback.User.UserId, targetUserID, page)
		}
	case strings.HasPrefix(payload, "stats_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "stats_%d", &groupID); err == nil {
			h.handleViewStats(ctx, groupID, upd.Callback.User.UserId)
		}
	default:
		h.logger.Warn("Unknown callback payload", "payload", payload)
	}
}

func (h *CallbackHandler) sendMainMenu(ctx context.Context, userID int64) {
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("Мои чаты", schemes.DEFAULT, "my_groups")
	kb.AddRow().AddCallback("Добавить чат", schemes.POSITIVE, "add_group")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(messages.MsgMainMenu)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send main menu", "error", err)
	}
}
