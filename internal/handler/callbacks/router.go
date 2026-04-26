package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel/attribute"
	//"go.opentelemetry.io/otel/trace"
)

// Handle обрабатывает callback-обновления от MAX Bot API
func (h *CallbackHandler) Handle(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	payload := upd.Callback.Payload
	chatID := upd.Callback.GetChatID()
	if chatID == 0 && upd.Message != nil {
		chatID = upd.Message.Recipient.ChatId
	}
	userID := upd.Callback.GetUserID()

	ctx, span := h.tracer.Start(ctx, "handleCallback")
	defer span.End()

	span.SetAttributes(
		attribute.String("payload", payload),
		attribute.Int64("chat_id", chatID),
		attribute.Int64("user_id", userID),
	)

	h.logger.Info("Received callback",
		"payload", payload,
		"chat_id", chatID,
		"user_id", userID,
	)

	// Удаляем сообщение с клавиатурой после нажатия (чтобы не засорять чат)
	if upd.Message != nil && upd.Message.Body.Mid != "" {
		go func() {
			bgCtx := context.Background()
			if _, err := h.bot.Messages.DeleteMessage(bgCtx, upd.Message.Body.Mid); err != nil {
				h.logger.Warn("Failed to delete callback message", "error", err)
			}
		}()
	}

	// Роутинг по payload
	switch {
	case payload == "add_group":
		h.handleAddGroup(ctx, userID)

	case strings.HasPrefix(payload, "my_groups"):
		var page int
		if _, err := fmt.Sscanf(payload, "my_groups_%d", &page); err == nil {
			h.handleMyGroups(ctx, userID, page)
		} else {
			h.handleMyGroups(ctx, userID, 1)
		}

	case payload == "main_menu":
		h.sendMainMenu(ctx, userID)

	case strings.HasPrefix(payload, "manage_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "manage_%d", &groupID); err == nil {
			h.HandleManageGroup(ctx, groupID, userID)
		} else {
			h.logger.Error("Invalid manage payload", "payload", payload)
		}

	case strings.HasPrefix(payload, "toggle_"):
		h.handleToggleSetting(ctx, payload, userID)

	case strings.HasPrefix(payload, "prompt_words_"):
		h.handlePromptInput(ctx, payload, userID, "add_words")

	case strings.HasPrefix(payload, "prompt_domains_"):
		h.handlePromptInput(ctx, payload, userID, "add_domains")

	case strings.HasPrefix(payload, "prompt_import_words_"):
		h.handlePromptInput(ctx, payload, userID, "import_words")

	case strings.HasPrefix(payload, "clear_words_"):
		h.handleClearBlocked(ctx, payload, userID, "clear_words")

	case strings.HasPrefix(payload, "clear_domains_"):
		h.handleClearBlocked(ctx, payload, userID, "clear_domains")

	case strings.HasPrefix(payload, "lm_"):
		var groupID int64
		var page int
		if _, err := fmt.Sscanf(payload, "lm_%d_%d", &groupID, &page); err == nil {
			h.handleListMutes(ctx, groupID, userID, page)
		} else if _, err := fmt.Sscanf(payload, "lm_%d", &groupID); err == nil {
			h.handleListMutes(ctx, groupID, userID, 1)
		}

	case strings.HasPrefix(payload, "um_"):
		var groupID, targetUserID int64
		var page int
		if _, err := fmt.Sscanf(payload, "um_%d_%d_%d", &groupID, &targetUserID, &page); err == nil {
			h.handleUnmute(ctx, groupID, userID, targetUserID, page)
		}

	case strings.HasPrefix(payload, "vm_"):
		var groupID, targetUserID int64
		var page int
		if _, err := fmt.Sscanf(payload, "vm_%d_%d_%d", &groupID, &targetUserID, &page); err == nil {
			h.handleViewMute(ctx, groupID, userID, targetUserID, page)
		}

	case strings.HasPrefix(payload, "stats_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "stats_%d", &groupID); err == nil {
			h.handleViewStats(ctx, groupID, userID)
		}

	// === РАССЫЛКА ===
	case payload == "broadcast_start":
		// ✅ Вызываем обновлённую функцию с экраном подтверждения
		h.handleBroadcastStart(ctx, userID)
		return

	// ✅ НОВЫЙ КЕЙС: Финальное подтверждение для рассылки во все чаты
	case payload == "broadcast_start_final":
		// Переход к вводу текста для рассылки во все чаты
		h.handleBroadcastStartFinal(ctx, userID)
		return

	case payload == "broadcast_cancel":
		// Отмена рассылки
		if err := h.userStateRepo.ClearState(userID); err != nil {
			h.logger.Error("Failed to clear broadcast state", "error", err)
		}
		msg := maxbot.NewMessage()
		msg.SetUser(userID)
		msg.SetText("Рассылка отменена.")
		if err := h.bot.Messages.Send(ctx, msg); err != nil {
			h.logger.Error("Failed to send cancel message", "error", err)
		}
		return

	// === ВЫБОР ЧАТОВ ДЛЯ РАССЫЛКИ ===
	case payload == "broadcast_select_chats":
		// Показать список чатов с чекбоксами (начинаем с первой страницы)
		h.handleBroadcastSelectChats(ctx, userID, 1)
		return

	case strings.HasPrefix(payload, "broadcast_toggle_chat_"):
		// Переключить выбор чата
		var chatID int64
		if _, err := fmt.Sscanf(payload, "broadcast_toggle_chat_%d", &chatID); err == nil {
			h.handleBroadcastToggleChat(ctx, userID, chatID)
		}
		return

	case payload == "broadcast_confirm_chats":
		// Показать экран подтверждения перед рассылкой
		h.handleBroadcastConfirmChats(ctx, userID)
		return

	case payload == "broadcast_confirm_final":
		// ✅ Финальное подтверждение → переход к вводу текста
		h.handleBroadcastFinalConfirm(ctx, userID)
		return

	case payload == "broadcast_clear_selection":
		// Очистить выбор чатов
		h.handleBroadcastClearSelection(ctx, userID)
		return

	// === ПАГИНАЦИЯ ВЫБОРА ЧАТОВ ===
	case payload == "broadcast_prev_page":
		// Перейти на предыдущую страницу
		h.handleBroadcastPrevPage(ctx, userID)
		return

	case payload == "broadcast_next_page":
		// Перейти на следующую страницу
		h.handleBroadcastNextPage(ctx, userID)
		return

	default:
		h.logger.Warn("Unknown callback payload", "payload", payload)
	}
}

// sendMainMenu отправляет главное меню администратора
func (h *CallbackHandler) sendMainMenu(ctx context.Context, userID int64) {
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnMyGroups, schemes.DEFAULT, "my_groups")
	kb.AddRow().AddCallback(messages.BtnAddGroup, schemes.POSITIVE, "add_group")

	// ✅ НОВАЯ КНОПКА: Выбор чатов для рассылки
	kb.AddRow().AddCallback(messages.BtnBroadcastSelectChats, schemes.DEFAULT, "broadcast_select_chats")

	// Кнопка общей рассылки
	kb.AddRow().AddCallback(messages.BtnBroadcast, schemes.NEGATIVE, "broadcast_start")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(messages.MsgMainMenu)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send main menu", "error", err)
	}
}
