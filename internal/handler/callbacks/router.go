package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"os"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel/attribute"
)

func (h *CallbackHandler) Handle(ctx context.Context, upd *schemes.MessageCallbackUpdate) {
	// 🔥 ЯДЕРНЫЙ ЛОГ: сработает, если функция вообще вызывается
	fmt.Printf("### CALLBACK_HANDLE_CALLED: payload=%s user_id=%d\n",
		upd.Callback.Payload, upd.Callback.GetUserID())
	payload := upd.Callback.Payload
	chatID := upd.Callback.GetChatID()
	if chatID == 0 && upd.Message != nil {
		chatID = upd.Message.Recipient.ChatId
	}
	userID := upd.Callback.GetUserID()

	// 🔍 ОТЛАДОЧНЫЙ ЛОГ #1: Видим ВСЕ входящие колбэки
	// Добавлен для диагностики: показываем payload и user_id до любой логики
	h.logger.Info("# CALLBACK_DEBUG",
		"payload", payload,
		"user_id", userID,
		"chat_id", chatID,
	)

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
		h.logger.Info("🔍 MATCH: add_group", "user_id", userID)
		h.handleAddGroup(ctx, userID)
		return

	case strings.HasPrefix(payload, "my_groups"):
		var page int
		if _, err := fmt.Sscanf(payload, "my_groups_%d", &page); err == nil {
			h.logger.Info("🔍 MATCH: my_groups", "user_id", userID, "page", page)
			h.handleMyGroups(ctx, userID, page)
		} else {
			h.logger.Info("🔍 MATCH: my_groups (default page=1)", "user_id", userID)
			h.handleMyGroups(ctx, userID, 1)
		}
		return

	case payload == "main_menu":
		h.logger.Info("🔍 MATCH: main_menu", "user_id", userID)
		h.SendMainMenu(ctx, userID)
		return

	case payload == "monitoring_stats":
		h.logger.Info("🔍 MATCH: monitoring_stats", "user_id", userID)
		h.handleMonitoringStats(ctx, userID)
		return

	case payload == "monitoring_refresh_cache":
		h.logger.Info("🔍 MATCH: monitoring_refresh_cache", "user_id", userID)
		h.handleMonitoringRefreshCache(ctx, userID)
		return

	case payload == "monitoring_export":
		h.logger.Info("🔍 MATCH: monitoring_export", "user_id", userID)
		h.handleMonitoringExport(ctx, userID)
		return

	case payload == "monitoring_activity":
		h.logger.Info("🔍 MATCH: monitoring_activity", "user_id", userID)
		h.handleMonitoringActivity(ctx, userID)
		return

	case strings.HasPrefix(payload, "manage_"):
		var groupID int64
		if _, err := fmt.Sscanf(payload, "manage_%d", &groupID); err == nil {
			h.logger.Info("🔍 MATCH: manage_", "user_id", userID, "chat_id", groupID)
			h.HandleManageGroup(ctx, groupID, userID)
		}
		return

	case strings.HasPrefix(payload, "toggle_"):
		h.logger.Info("🔍 MATCH: toggle_", "user_id", userID, "payload", payload)
		h.handleToggleSetting(ctx, payload, userID)
		return

	// === НАСТРОЙКИ ЧАТА: ПРОМПТЫ И ОЧИСТКА (с логами) ===

	// ✅ prompt_words_<chat_id> — показать промпт для добавления слов
	case strings.HasPrefix(payload, "prompt_words_"):
		fmt.Printf("### MATCH_PROMPT_WORDS: payload=%s user=%d\n", payload, userID)
		os.Stdout.Sync()

		var chatID int64
		if _, err := fmt.Sscanf(payload, "prompt_words_%d", &chatID); err == nil {
			fmt.Printf("### CALLING_handlePromptInput: chatID=%d action=add_words\n", chatID)
			os.Stdout.Sync()
			h.handlePromptInput(ctx, payload, userID, "add_words")
		} else {
			fmt.Printf("### SSCANF_FAILED: payload=%s err=%v\n", payload, err)
			os.Stdout.Sync()
		}
		return

	// ✅ prompt_import_words_<chat_id> — показать промпт для импорта слов из TXT
	case strings.HasPrefix(payload, "prompt_import_words_"):
		h.logger.Info("🔍 MATCH: prompt_import_words", "payload", payload, "user_id", userID)
		var chatID int64
		if _, err := fmt.Sscanf(payload, "prompt_import_words_%d", &chatID); err == nil {
			h.logger.Info("✅ Calling handlePromptInput", "action", "import_words", "chat_id", chatID)
			h.handlePromptInput(ctx, payload, userID, "import_words")
		} else {
			h.logger.Warn("❌ Failed to parse chatID from prompt_import_words payload", "payload", payload)
		}
		return

	// ✅ prompt_domains_<chat_id> — показать промпт для добавления доменов
	case strings.HasPrefix(payload, "prompt_domains_"):
		h.logger.Info("🔍 MATCH: prompt_domains", "payload", payload, "user_id", userID)
		var chatID int64
		if _, err := fmt.Sscanf(payload, "prompt_domains_%d", &chatID); err == nil {
			h.logger.Info("✅ Calling handlePromptInput", "action", "add_domains", "chat_id", chatID)
			h.handlePromptInput(ctx, payload, userID, "add_domains")
		} else {
			h.logger.Warn("❌ Failed to parse chatID from prompt_domains payload", "payload", payload)
		}
		return

	// ✅ clear_words_<chat_id> — очистить список слов
	case strings.HasPrefix(payload, "clear_words_"):
		h.logger.Info("🔍 MATCH: clear_words", "payload", payload, "user_id", userID)
		var chatID int64
		if _, err := fmt.Sscanf(payload, "clear_words_%d", &chatID); err == nil {
			h.logger.Info("✅ Calling handleClearBlocked", "action", "clear_words", "chat_id", chatID)
			h.handleClearBlocked(ctx, payload, userID, "clear_words")
		} else {
			h.logger.Warn("❌ Failed to parse chatID from clear_words payload", "payload", payload)
		}
		return

	// ✅ clear_domains_<chat_id> — очистить список доменов
	case strings.HasPrefix(payload, "clear_domains_"):
		h.logger.Info("🔍 MATCH: clear_domains", "payload", payload, "user_id", userID)
		var chatID int64
		if _, err := fmt.Sscanf(payload, "clear_domains_%d", &chatID); err == nil {
			h.logger.Info("✅ Calling handleClearBlocked", "action", "clear_domains", "chat_id", chatID)
			h.handleClearBlocked(ctx, payload, userID, "clear_domains")
		} else {
			h.logger.Warn("❌ Failed to parse chatID from clear_domains payload", "payload", payload)
		}
		return

	// === СТАТИСТИКА И МУТЫ: ИСПРАВЛЕННЫЕ КЕЙСЫ (добавлено) ===

	// ✅ stats_<chat_id> — показать статистику по чату
	// Вызов: handleViewStats(ctx, chatID, userID) — порядок: chatID ПЕРЕД userID!
	case strings.HasPrefix(payload, "stats_"):
		fmt.Printf("### MATCH_STATS: payload=%s user=%d\n", payload, userID)
		os.Stdout.Sync()
		var chatID int64
		if _, err := fmt.Sscanf(payload, "stats_%d", &chatID); err == nil {
			fmt.Printf("### CALLING_handleViewStats: chatID=%d userID=%d\n", chatID, userID)
			os.Stdout.Sync()
			h.handleViewStats(ctx, chatID, userID) // ✅ Правильный порядок: chatID, userID
		} else {
			fmt.Printf("### SSCANF_FAILED_STATS: payload=%s err=%v\n", payload, err)
			os.Stdout.Sync()
			h.logger.Warn("Failed to parse chatID from stats payload", "payload", payload)
		}
		return

	// ✅ lm_<chat_id>[_<page>] — показать управление мутами (list mutes) с пагинацией
	// Поддерживает: "lm_123" (page=1) и "lm_123_2" (page=2)
	// Вызов: handleListMutes(ctx, chatID, userID, page) — порядок: chatID, userID, page!
	case strings.HasPrefix(payload, "lm_"):
		fmt.Printf("### MATCH_LM: payload=%s user=%d\n", payload, userID)
		os.Stdout.Sync()
		var chatID, page int64
		// Пробуем распарсить с пагинацией: "lm_123_2"
		if n, _ := fmt.Sscanf(payload, "lm_%d_%d", &chatID, &page); n == 2 {
			fmt.Printf("### CALLING_handleListMutes: chatID=%d userID=%d page=%d\n", chatID, userID, page)
			os.Stdout.Sync()
			h.handleListMutes(ctx, chatID, userID, int(page))
		} else if _, err := fmt.Sscanf(payload, "lm_%d", &chatID); err == nil {
			// Без пагинации: "lm_123" → page=1 по умолчанию
			fmt.Printf("### CALLING_handleListMutes: chatID=%d userID=%d page=1\n", chatID, userID)
			os.Stdout.Sync()
			h.handleListMutes(ctx, chatID, userID, 1)
		} else {
			fmt.Printf("### SSCANF_FAILED_LM: payload=%s err=%v\n", payload, err)
			os.Stdout.Sync()
			h.logger.Warn("Failed to parse chatID from lm payload", "payload", payload)
		}
		return

	// ✅ vm_<chat_id>_<user_id>_<page> — просмотр деталей мута
	// Вызов: handleViewMute(ctx, chatID, userID, targetUserID, page)
	case strings.HasPrefix(payload, "vm_"):
		fmt.Printf("### MATCH_VM: payload=%s user=%d\n", payload, userID)
		os.Stdout.Sync()
		var chatID, targetUserID, page int64
		if _, err := fmt.Sscanf(payload, "vm_%d_%d_%d", &chatID, &targetUserID, &page); err == nil {
			fmt.Printf("### CALLING_handleViewMute: chatID=%d targetUserID=%d page=%d\n", chatID, targetUserID, page)
			os.Stdout.Sync()
			h.handleViewMute(ctx, chatID, userID, targetUserID, int(page))
		} else {
			fmt.Printf("### SSCANF_FAILED_VM: payload=%s err=%v\n", payload, err)
			os.Stdout.Sync()
			h.logger.Warn("Failed to parse payload from vm_ callback", "payload", payload)
		}
		return

	// ✅ um_<chat_id>_<user_id>_<page> — размутить пользователя
	// Вызов: handleUnmute(ctx, chatID, adminID, targetUserID, page)
	case strings.HasPrefix(payload, "um_"):
		fmt.Printf("### MATCH_UM: payload=%s user=%d\n", payload, userID)
		os.Stdout.Sync()
		var chatID, targetUserID, page int64
		if _, err := fmt.Sscanf(payload, "um_%d_%d_%d", &chatID, &targetUserID, &page); err == nil {
			fmt.Printf("### CALLING_handleUnmute: chatID=%d adminID=%d targetUserID=%d page=%d\n", chatID, userID, targetUserID, page)
			os.Stdout.Sync()
			h.handleUnmute(ctx, chatID, userID, targetUserID, int(page))
		} else {
			fmt.Printf("### SSCANF_FAILED_UM: payload=%s err=%v\n", payload, err)
			os.Stdout.Sync()
			h.logger.Warn("Failed to parse payload from um_ callback", "payload", payload)
		}
		return

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
		h.logger.Info("🔍 MATCH: broadcast_cancel", "user_id", userID)
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
		h.logger.Info("🔍 MATCH: broadcast_select_chats", "user_id", userID)
		h.handleBroadcastSelectChats(ctx, userID, 1)
		return

	// ✅ broadcast_confirm_chats — подтверждение выбранных чатов
	case payload == "broadcast_confirm_chats":
		h.logger.Info("🔍 DEBUG: broadcast_confirm_chats HIT", "user_id", userID)
		h.handleBroadcastConfirmChats(ctx, userID)
		return

	// ✅ broadcast_send_all — альтернатива: сразу во все чаты
	case payload == "broadcast_send_all":
		h.logger.Info("🔍 MATCH: broadcast_send_all", "user_id", userID)
		_ = h.userStateRepo.SetState(userID, 0, "broadcast_wait_text")
		h.sendText(ctx, userID, "✍️ Введите текст рассылки во все чаты...")
		return

	// ✅ broadcast_clear_selection — очистка выбора чатов
	case payload == "broadcast_clear_selection":
		h.logger.Info("🔍 MATCH: broadcast_clear_selection", "user_id", userID)
		_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", "selected:;page:1")
		h.handleBroadcastSelectChats(ctx, userID, 1)
		return

	// ✅ broadcast_page_info — информационная кнопка (заглушка)
	case payload == "broadcast_page_info":
		h.logger.Info("🔍 MATCH: broadcast_page_info (no-op)", "user_id", userID)
		return

	// ✅ broadcast_prev_page / broadcast_next_page — пагинация
	case payload == "broadcast_prev_page":
		h.logger.Info("🔍 MATCH: broadcast_prev_page", "user_id", userID)
		h.handleBroadcastPrevPage(ctx, userID)
		return

	case payload == "broadcast_next_page":
		h.logger.Info("🔍 MATCH: broadcast_next_page", "user_id", userID)
		h.handleBroadcastNextPage(ctx, userID)
		return

	// === РАССЫЛКА: HASPREFIX (ДОЛЖНЫ БЫТЬ ПОСЛЕ ТОЧНЫХ СОВПАДЕНИЙ!) ===

	// ⚠️ Этот кейс теперь НЕ перехватит "broadcast_start_final",
	// потому что точный кейс выше уже обработал его и сделал `return`
	case strings.HasPrefix(payload, "broadcast_start"):
		h.logger.Info("🔍 MATCH: broadcast_start (HasPrefix)", "payload", payload, "user_id", userID)
		h.handleBroadcastStart(ctx, userID)
		return

	// ⚠️ Аналогично: этот кейс не перехватит точные совпадения выше
	case strings.HasPrefix(payload, "broadcast_toggle_chat_"):
		h.logger.Info("🔍 MATCH: broadcast_toggle_chat_ (HasPrefix)", "payload", payload, "user_id", userID)
		var chatID int64
		if _, err := fmt.Sscanf(payload, "broadcast_toggle_chat_%d", &chatID); err == nil {
			h.handleBroadcastToggleChat(ctx, userID, chatID)
		}
		return

	// === НЕИЗВЕСТНЫЕ PAYLOAD ===
	default:
		h.logger.Warn("⚠️ Unknown callback payload", "payload", payload, "user_id", userID)
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
