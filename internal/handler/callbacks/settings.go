package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/metrics"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *CallbackHandler) verifyAccess(ctx context.Context, userID, targetChatID int64) bool {
	managedChats, err := h.svc.GetManagedChats(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get managed chats for verification", "user_id", userID, "error", err)
		return false
	}
	for _, id := range managedChats {
		if id == targetChatID {
			return true
		}
	}
	return false
}

func (h *CallbackHandler) HandleManageGroup(ctx context.Context, chatID int64, userID int64) {
	if !h.verifyAccess(ctx, userID, chatID) {
		h.logger.Warn("Access denied", "user_id", userID, "chat_id", chatID)
		return
	}

	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Warn("Failed to delete user state in manage group", "error", err)
	}

	settings, err := h.svc.GetChatSettings(ctx, chatID)
	if err != nil {
		h.logger.Error("Failed to get settings", "chat_id", chatID, "error", err)
		msg := maxbot.NewMessage()
		msg.SetUser(userID)
		msg.SetText(messages.MsgFailedToLoadSettings)
		msg.SetFormat("markdown")
		if err := h.bot.Messages.Send(ctx, msg); err != nil {
			h.logger.Error("Failed to send error message", "error", err)
		}
		return
	}
	status := func(enabled bool) string {
		if enabled {
			return "✅"
		}
		return "❌"
	}
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnAutoDelete, status(settings.EnableAutoDelete)), schemes.POSITIVE, fmt.Sprintf("toggle_autodelete_%d", chatID))

	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnWordFilter, status(settings.EnableWordFilter)), schemes.POSITIVE, fmt.Sprintf("toggle_words_%d", chatID))
	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnLinkFilter, status(settings.EnableLinkFilter)), schemes.POSITIVE, fmt.Sprintf("toggle_links_%d", chatID))

	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnRestrictImage, status(settings.RestrictImage)), schemes.POSITIVE, fmt.Sprintf("toggle_image_%d", chatID))
	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnRestrictVideo, status(settings.RestrictVideo)), schemes.POSITIVE, fmt.Sprintf("toggle_video_%d", chatID))
	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnRestrictAudio, status(settings.RestrictAudio)), schemes.POSITIVE, fmt.Sprintf("toggle_audio_%d", chatID))
	kb.AddRow().AddCallback(fmt.Sprintf(messages.BtnRestrictFile, status(settings.RestrictFile)), schemes.POSITIVE, fmt.Sprintf("toggle_file_%d", chatID))

	kb.AddRow().AddCallback(messages.BtnAddWords, schemes.DEFAULT, fmt.Sprintf("prompt_words_%d", chatID))
	kb.AddRow().AddCallback(messages.BtnImportWords, schemes.DEFAULT, fmt.Sprintf("prompt_import_words_%d", chatID))
	kb.AddRow().AddCallback(messages.BtnClearWords, schemes.NEGATIVE, fmt.Sprintf("clear_words_%d", chatID))

	kb.AddRow().AddCallback(messages.BtnAddDomains, schemes.DEFAULT, fmt.Sprintf("prompt_domains_%d", chatID))
	kb.AddRow().AddCallback(messages.BtnClearDomains, schemes.NEGATIVE, fmt.Sprintf("clear_domains_%d", chatID))

	kb.AddRow().AddCallback(messages.BtnMutesManagement, schemes.DEFAULT, fmt.Sprintf("lm_%d", chatID))
	kb.AddRow().AddCallback(messages.BtnStatistics, schemes.DEFAULT, fmt.Sprintf("stats_%d", chatID))

	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, "my_groups")

	label := fmt.Sprintf("%d", chatID)
	if chat, err := h.bot.Chats.GetChat(ctx, chatID); err == nil && chat.Title != "" {
		label = chat.Title
	}

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf(messages.MsgSettingsForGroup, label))
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send settings message", "error", err)
	}
}

func (h *CallbackHandler) handleToggleSetting(ctx context.Context, payload string, userID int64) {
	parts := strings.Split(payload, "_")
	if len(parts) < 3 {
		return
	}
	setting := parts[1]
	var chatID int64
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &chatID); err != nil {
		h.logger.Error("Invalid chat ID in toggle", "payload", payload)
		return
	}

	if !h.verifyAccess(ctx, userID, chatID) {
		h.logger.Warn("Access denied for toggle", "user_id", userID, "chat_id", chatID)
		return
	}

	h.logger.Info("Toggle setting requested", "setting", setting, "chat_id", chatID)
	val, err := h.svc.ToggleSetting(ctx, chatID, setting)
	if err != nil {
		h.logger.Error("Failed to toggle setting", "error", err)
	} else {
		h.logger.Info("Toggle setting success", "setting", setting, "chat_id", chatID, "old_value", !val, "new_value", val)
	}
	h.HandleManageGroup(ctx, chatID, userID)
	metrics.IncBotAction("toggle_setting")
}

func (h *CallbackHandler) handlePromptInput(ctx context.Context, payload string, userID int64, action string) {
	parts := strings.Split(payload, "_")
	if len(parts) < 3 {
		return
	}
	var chatID int64
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &chatID); err != nil {
		h.logger.Error("Invalid chat ID in prompt", "payload", payload)
		return
	}

	if !h.verifyAccess(ctx, userID, chatID) {
		h.logger.Warn("Access denied for prompt", "user_id", userID, "chat_id", chatID)
		return
	}

	if err := h.userStateRepo.SetState(userID, chatID, action); err != nil {
		h.logger.Error("Failed to set user state", "error", err)
	}
	label := fmt.Sprintf("%d", chatID)
	if chat, err := h.bot.Chats.GetChat(ctx, chatID); err == nil && chat.Title != "" {
		label = chat.Title
	}

	settings, err := h.svc.GetChatSettings(ctx, chatID)
	if err != nil {
		h.logger.Error("Failed to get settings for prompt", "chat_id", chatID, "error", err)
	}

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetFormat("markdown")

	switch action {
	case "add_words":
		examples := "плохое, злое, спам"
		if settings != nil && len(settings.BlockedWords) > 0 {
			examples = strings.Join(settings.BlockedWords, ", ")
		}
		msg.SetText(fmt.Sprintf(messages.MsgPromptAddWords, label, examples))
	case "import_words":
		msg.SetText(fmt.Sprintf(messages.MsgPromptImportWords, label))
	default:
		examples := "bad.com, spam.org"
		if settings != nil && len(settings.BlockedDomains) > 0 {
			examples = strings.Join(settings.BlockedDomains, ", ")
		}
		msg.SetText(fmt.Sprintf(messages.MsgPromptAddDomains, label, examples))
	}
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("manage_%d", chatID))
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send prompt message", "error", err)
	}
}

func (h *CallbackHandler) handleClearBlocked(ctx context.Context, payload string, userID int64, action string) {
	parts := strings.Split(payload, "_")
	if len(parts) < 3 {
		return
	}
	var chatID int64
	if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &chatID); err != nil {
		h.logger.Error("Invalid chat ID in clear", "payload", payload)
		return
	}

	if !h.verifyAccess(ctx, userID, chatID) {
		h.logger.Warn("Access denied for clear", "user_id", userID, "chat_id", chatID)
		return
	}

	var err error
	var msgText string

	if action == "clear_words" {
		err = h.svc.SetBlockedWords(ctx, chatID, []string{})
		msgText = messages.MsgWordsCleared
	} else {
		err = h.svc.SetBlockedDomains(ctx, chatID, []string{})
		msgText = messages.MsgDomainsCleared
	}

	if err != nil {
		h.logger.Error("Failed to clear blocked items", "error", err)
		return
	}

	h.HandleManageGroup(ctx, chatID, userID)

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(msgText)
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send clear confirmation", "error", err)
	}
}
