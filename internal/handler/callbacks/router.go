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

	// ✅ УНИВЕРСАЛЬНОЕ УДАЛЕНИЕ СООБЩЕНИЯ С КНОПКОЙ (с улучшенным логом)
	if upd.Message != nil && upd.Message.Body.Mid != "" {
		go func() {
			bgCtx := context.Background()
			mid := upd.Message.Body.Mid
			if _, err := h.bot.Messages.DeleteMessage(bgCtx, mid); err != nil {
				h.logger.Warn("❌ Failed to delete callback message", "error", err, "mid", mid)
			} else {
				h.logger.Info("✅ Deleted callback message", "mid", mid)
			}
		}()
	}

	switch {
	// === ОБЩИЕ КЕЙСЫ ===
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
		h.SendMainMenu(ctx, userID)
		return

	case payload == "monitoring_stats":
		h.handleMonitoringStats(ctx, userID)
		return

	case payload == "monitoring_refresh_cache":
		h.handleMonitoringRefreshCache(ctx, userID)
		return

	case payload == "monitoring_export":
		h.handleMonitoringExport(ctx, userID)
		return

	case payload == "monitoring_activity":
		h.handleMonitoringActivity(ctx, userID)
		return

	case strings.HasPrefix(payload, "manage_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "manage_%d", &groupID); err == nil {
			h.HandleManageGroup(ctx, groupID, userID)
		}

	case strings.HasPrefix(payload, "toggle_"):
		h.handleToggleSetting(ctx, payload, userID)

	// === РАССЫЛКА: ТОЧНЫЕ СОВПАДЕНИЯ (ДОЛЖНЫ БЫТЬ ПЕРЕД HASPREFIX!) ===

	// ✅ broadcast_start_final — финальное подтверждение рассылки во ВСЕ чаты
	// Должно быть ВЫШЕ, чем strings.HasPrefix("broadcast_start")
	case payload == "broadcast_start_final":
		h.logger.Info("🔍 DEBUG: broadcast_start_final HIT", "user_id", userID)
		h.handleBroadcastStartFinal(ctx, userID)
		return

	// ✅ broadcast_confirm_final — финальное подтверждение рассылки в ВЫБРАННЫЕ чаты
	// Должно быть ВЫШЕ, чем strings.HasPrefix("broadcast_confirm_") (если бы был такой)
	case payload == "broadcast_confirm_final":
		h.logger.Info("🔍 DEBUG: broadcast_confirm_final HIT", "user_id", userID)
		h.handleBroadcastFinalConfirm(ctx, userID)
		return

	// ✅ broadcast_cancel — отмена рассылки
	case payload == "broadcast_cancel":
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

	// ✅ broadcast_select_chats — выбор чатов для рассылки
	case payload == "broadcast_select_chats":
		h.handleBroadcastSelectChats(ctx, userID, 1)
		return

	// ✅ broadcast_confirm_chats — подтверждение выбранных чатов
	case payload == "broadcast_confirm_chats":
		h.logger.Info("🔍 DEBUG: broadcast_confirm_chats HIT", "user_id", userID)
		h.handleBroadcastConfirmChats(ctx, userID)
		return

	// ✅ broadcast_send_all — альтернатива: сразу во все чаты
	case payload == "broadcast_send_all":
		_ = h.userStateRepo.SetState(userID, 0, "broadcast_wait_text")
		h.sendText(ctx, userID, "✍️ Введите текст рассылки во все чаты...")
		return

	// ✅ broadcast_clear_selection — очистка выбора чатов
	case payload == "broadcast_clear_selection":
		_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", "selected:;page:1")
		h.handleBroadcastSelectChats(ctx, userID, 1)
		return

	// ✅ broadcast_page_info — информационная кнопка (заглушка)
	case payload == "broadcast_page_info":
		return

	// ✅ broadcast_prev_page / broadcast_next_page — пагинация
	case payload == "broadcast_prev_page":
		h.handleBroadcastPrevPage(ctx, userID)
		return

	case payload == "broadcast_next_page":
		h.handleBroadcastNextPage(ctx, userID)
		return

	// === РАССЫЛКА: HASPREFIX (ДОЛЖНЫ БЫТЬ ПОСЛЕ ТОЧНЫХ СОВПАДЕНИЙ!) ===

	// ⚠️ Этот кейс теперь НЕ перехватит "broadcast_start_final",
	// потому что точный кейс выше уже обработал его и сделал `return`
	case strings.HasPrefix(payload, "broadcast_start"):
		h.handleBroadcastStart(ctx, userID)
		return

	// ⚠️ Аналогично: этот кейс не перехватит точные совпадения выше
	case strings.HasPrefix(payload, "broadcast_toggle_chat_"):
		var chatID int64
		if _, err := fmt.Sscanf(payload, "broadcast_toggle_chat_%d", &chatID); err == nil {
			h.handleBroadcastToggleChat(ctx, userID, chatID)
		}
		return

	// === НЕИЗВЕСТНЫЕ PAYLOAD ===
	default:
		h.logger.Warn("⚠️ Unknown callback payload", "payload", payload)
	}
}

func (h *CallbackHandler) SendMainMenu(ctx context.Context, userID int64) {
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnMyGroups, schemes.DEFAULT, "my_groups")
	kb.AddRow().AddCallback(messages.BtnAddGroup, schemes.POSITIVE, "add_group")

	// ✅ Кнопка "Мониторинг" с правильной константой
	kb.AddRow().AddCallback(messages.BtnMonitoring, schemes.DEFAULT, "monitoring_stats")

	kb.AddRow().AddCallback(messages.BtnBroadcastSelectChats, schemes.DEFAULT, "broadcast_select_chats")
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
