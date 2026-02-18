package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/utils"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *CallbackHandler) handleViewStats(ctx context.Context, chatID int64, userID int64) {
	if !h.verifyAccess(ctx, userID, chatID) {
		return
	}

	stats, err := h.svc.GetChatStats(ctx, chatID)
	if err != nil {
		h.logger.Error("Failed to get chat stats", "chat_id", chatID, "error", err)
		return
	}

	label := fmt.Sprintf("%d", chatID)
	if chat, err := h.bot.Chats.GetChat(ctx, chatID); err == nil && chat.Title != "" {
		label = chat.Title
	}

	violationForms := [3]string{"нарушение", "нарушения", "нарушений"}
	muteForms := [3]string{"нарушитель", "нарушителя", "нарушителей"}

	text := ""
	_, activeMutesCount, err := h.svc.GetActiveMutesPaginated(ctx, chatID, 1)
	if err != nil {
		h.logger.Warn("Failed to get active mutes count for stats", "error", err)
		activeMutesCount = stats.MuteCount
	}

	text = fmt.Sprintf(messages.MsgChatStatistics,
		label,
		chatID,
		time.Now().Format("02.01.2006 15:04"),
		utils.Plural(stats.WordViolations, violationForms),
		utils.Plural(stats.LinkViolations, violationForms),
		utils.Plural(stats.ImageViolations, violationForms),
		utils.Plural(stats.VideoViolations, violationForms),
		utils.Plural(stats.AudioViolations, violationForms),
		utils.Plural(stats.FileViolations, violationForms),
		utils.Plural(activeMutesCount, muteForms),
	)

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("manage_%d", chatID))

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send stats message", "error", err)
	}
}
