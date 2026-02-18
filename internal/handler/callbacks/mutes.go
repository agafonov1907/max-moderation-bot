package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *CallbackHandler) handleListMutes(ctx context.Context, chatID int64, userID int64, page int) {
	if !h.verifyAccess(ctx, userID, chatID) {
		return
	}
	if page < 1 {
		page = 1
	}
	mutes, total, err := h.svc.GetActiveMutesPaginated(ctx, chatID, page)
	if err != nil {
		h.logger.Error("Failed to get mutes", "error", err)
		return
	}

	label := fmt.Sprintf("%d", chatID)
	if chat, err := h.bot.Chats.GetChat(ctx, chatID); err == nil && chat.Title != "" {
		label = chat.Title
	}

	if len(mutes) == 0 && page == 1 {
		kb := h.bot.Messages.NewKeyboardBuilder()
		kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("manage_%d", chatID))
		msg := maxbot.NewMessage()
		msg.SetUser(userID)
		msg.SetText(messages.MsgNoActiveMutes)
		msg.SetFormat("markdown")
		msg.AddKeyboard(kb)
		if err := h.bot.Messages.Send(ctx, msg); err != nil {
			h.logger.Error("Failed to send no mutes message", "error", err)
		}
		return
	}

	totalPages := (int(total) + 9) / 10
	text := fmt.Sprintf(messages.MsgMuteListTitle, label, page, totalPages)

	kb := h.bot.Messages.NewKeyboardBuilder()
	for _, m := range mutes {
		name := m.UserName
		if name == "" {
			name = fmt.Sprintf("User %d", m.UserID)
		}
		userLabel := fmt.Sprintf("👤 %s", name)
		kb.AddRow().AddCallback(userLabel, schemes.POSITIVE, fmt.Sprintf("vm_%d_%d_%d", chatID, m.UserID, page))
	}

	if totalPages > 1 {
		navRow := kb.AddRow()
		if page > 1 {
			navRow.AddCallback(messages.BtnPrevPage, schemes.DEFAULT, fmt.Sprintf("lm_%d_%d", chatID, page-1))
		}
		if page < totalPages {
			navRow.AddCallback(messages.BtnNextPage, schemes.DEFAULT, fmt.Sprintf("lm_%d_%d", chatID, page+1))
		}
	}

	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("manage_%d", chatID))

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send mutes list", "error", err)
	}
}

func (h *CallbackHandler) handleViewMute(ctx context.Context, chatID int64, userID int64, targetUserID int64, page int) {
	if !h.verifyAccess(ctx, userID, chatID) {
		return
	}

	targetMute, err := h.svc.GetMute(ctx, chatID, targetUserID)
	if err != nil {
		h.logger.Error("Failed to get mute for detail", "error", err)
		return
	}

	if targetMute == nil {
		h.handleListMutes(ctx, chatID, userID, page)
		return
	}

	label := fmt.Sprintf("%d", chatID)
	if chat, err := h.bot.Chats.GetChat(ctx, chatID); err == nil && chat.Title != "" {
		label = chat.Title
	}

	userName := targetMute.UserName
	if userName == "" {
		userName = fmt.Sprintf("User %d", targetMute.UserID)
	}

	text := fmt.Sprintf(messages.MsgMuteDetail,
		label,
		userName,
		targetMute.UserID,
		targetMute.ExpiresAt.Format("02.01.2006 15:04:05"),
	)

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnUnmute, schemes.NEGATIVE, fmt.Sprintf("um_%d_%d_%d", chatID, targetUserID, page))
	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("lm_%d_%d", chatID, page))

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send mute detail", "error", err)
	}
}

func (h *CallbackHandler) handleUnmute(ctx context.Context, chatID int64, adminID int64, targetUserID int64, page int) {
	if err := h.svc.UnmuteUser(ctx, chatID, adminID, targetUserID); err != nil {
		h.logger.Error("Failed to unmute", "error", err)
		return
	}
	h.sendText(ctx, adminID, fmt.Sprintf(messages.MsgUnmutedSuccess, targetUserID))
	h.handleListMutes(ctx, chatID, adminID, page)
}

func (h *CallbackHandler) sendText(ctx context.Context, userID int64, text string) {
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text message", "error", err)
	}
}
