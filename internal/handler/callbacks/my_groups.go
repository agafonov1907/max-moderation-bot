package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/metrics"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *CallbackHandler) handleAddGroup(ctx context.Context, userID int64) {
	token, err := h.svc.GenerateLinkToken(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to generate token", "error", err)
		return
	}
	text := fmt.Sprintf(messages.MsgTokenGenerated, token, token)
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, "main_menu")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send token message", "error", err)
	} else {
		metrics.IncBotAction("add_group")
	}
}

func (h *CallbackHandler) handleMyGroups(ctx context.Context, userID int64, page int) {
	if page < 1 {
		page = 1
	}
	chats, total, err := h.svc.GetManagedChatsPaginated(ctx, userID, page)
	if err != nil {
		h.logger.Error("Failed to get managed chats", "error", err)
		return
	}
	if len(chats) == 0 && page == 1 {
		kb := h.bot.Messages.NewKeyboardBuilder()
		kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, "main_menu")
		msg := maxbot.NewMessage()
		msg.SetUser(userID)
		msg.SetText(messages.MsgNoManagedGroups)
		msg.SetFormat("markdown")
		msg.AddKeyboard(kb)
		if err := h.bot.Messages.Send(ctx, msg); err != nil {
			h.logger.Error("Failed to send no groups message", "error", err)
		}
		return
	}

	totalPages := (int(total) + 9) / 10
	kb := h.bot.Messages.NewKeyboardBuilder()
	for _, id := range chats {
		label := fmt.Sprintf(messages.MsgGroupDefaultLabel, id)
		if chat, err := h.bot.Chats.GetChat(ctx, id); err == nil && chat.Title != "" {
			label = chat.Title
		}
		kb.AddRow().AddCallback(label, schemes.POSITIVE, fmt.Sprintf("manage_%d", id))
	}

	if totalPages > 1 {
		navRow := kb.AddRow()
		if page > 1 {
			navRow.AddCallback(messages.BtnPrevPage, schemes.DEFAULT, fmt.Sprintf("my_groups_%d", page-1))
		}
		if page < totalPages {
			navRow.AddCallback(messages.BtnNextPage, schemes.DEFAULT, fmt.Sprintf("my_groups_%d", page+1))
		}
	}

	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, "main_menu")
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf(messages.MsgGroupListTitle, page, totalPages))
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send select group message", "error", err)
	}
}
