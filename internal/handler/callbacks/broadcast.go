package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"strconv"
	"strings"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// handleBroadcastStart начинает процесс рассылки
func (h *CallbackHandler) handleBroadcastStart(ctx context.Context, userID int64) {
	if err := h.userStateRepo.SetState(userID, 0, "broadcast_wait_text"); err != nil {
		h.logger.Error("Failed to set broadcast state", "error", err)
		return
	}

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(messages.MsgBroadcastPrompt)
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send broadcast prompt", "error", err)
	}
}

// handleBroadcastCancel отменяет рассылку и возвращает в меню
func (h *CallbackHandler) handleBroadcastCancel(ctx context.Context, userID int64) {
	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Error("Failed to clear state", "error", err)
	}

	// ✅ Отправляем сообщение об отмене с кнопкой возврата в меню
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText("❌ Рассылка отменена.")
	msg.SetFormat("markdown")

	// Добавляем кнопку возврата в главное меню
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("🔙 В меню", schemes.DEFAULT, "main_menu")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send cancel message", "error", err)
	}
}

// handleBroadcastSelectChats — показывает список чатов с чекбоксами и названиями
func (h *CallbackHandler) handleBroadcastSelectChats(ctx context.Context, userID int64) {
	// ✅ Получаем чаты с названиями вместо простых ID
	chatInfos, err := h.svc.GetManagedChatsWithNames(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get managed chats with names", "error", err)
		h.sendText(ctx, userID, "❌ Ошибка при получении списка чатов.")
		return
	}

	if len(chatInfos) == 0 {
		h.sendText(ctx, userID, messages.MsgNoManagedGroups)
		return
	}

	// Получаем текущие выбранные чаты из состояния
	state, _ := h.userStateRepo.GetState(userID)
	selectedChats := make(map[int64]bool)

	if state != nil && state.Action == "broadcast_selected_chats" && state.Metadata != "" {
		for _, idStr := range strings.Split(state.Metadata, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				selectedChats[id] = true
			}
		}
	}

	// Строим клавиатуру с чекбоксами
	kb := h.bot.Messages.NewKeyboardBuilder()
	row := kb.AddRow()

	for i, chat := range chatInfos {
		mark := "⬜"
		if selectedChats[chat.ID] {
			mark = "✅"
		}
		
		// ✅ ПОКАЗЫВАЕМ НАЗВАНИЕ + ID (если название пустое — fallback на "Чат %d")
		chatLabel := chat.Name
		if chatLabel == "" {
			chatLabel = fmt.Sprintf("Чат %d", chat.ID)
		}
		
		// Обрезаем длинное название для кнопки (MAX API имеет лимит ~40 символов)
		if len(chatLabel) > 35 {
			chatLabel = chatLabel[:32] + "..."
		}

		row.AddCallback(
			fmt.Sprintf("%s %s", mark, chatLabel),
			schemes.DEFAULT,
			fmt.Sprintf("broadcast_toggle_chat_%d", chat.ID),
		)

		// По 2 кнопки в ряд
		if (i+1)%2 == 0 && i < len(chatInfos)-1 {
			row = kb.AddRow()
		}
	}

	// Кнопки управления
	kb.AddRow().
		AddCallback(messages.BtnBroadcastSendAll, schemes.POSITIVE, "broadcast_send_all").
		AddCallback(messages.BtnBroadcastClear, schemes.NEGATIVE, "broadcast_clear_selection")
	kb.AddRow().
		AddCallback(messages.BtnBroadcastConfirm, schemes.POSITIVE, "broadcast_confirm_chats").
		AddCallback("❌ Отмена", schemes.NEGATIVE, "main_menu")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(messages.MsgBroadcastSelectChats)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send chat selection", "error", err)
	}
}

// handleBroadcastToggleChat — переключает выбор чата
func (h *CallbackHandler) handleBroadcastToggleChat(ctx context.Context, userID, chatID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	selectedChats := make(map[int64]bool)

	if state != nil && state.Action == "broadcast_selected_chats" && state.Metadata != "" {
		for _, idStr := range strings.Split(state.Metadata, ",") {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				selectedChats[id] = true
			}
		}
	}

	// Переключаем чат
	if selectedChats[chatID] {
		delete(selectedChats, chatID)
	} else {
		selectedChats[chatID] = true
	}

	// Сохраняем обратно в состояние
	var idsStr string
	for id := range selectedChats {
		if idsStr != "" {
			idsStr += ","
		}
		idsStr += fmt.Sprintf("%d", id)
	}

	// ✅ ИСПРАВЛЕНО: 4 аргумента (userID, chatID, action, metadata)
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", idsStr)

	// Обновляем клавиатуру
	h.handleBroadcastSelectChats(ctx, userID)
}

// handleBroadcastConfirmChats — подтверждает выбор и переходит к вводу текста
func (h *CallbackHandler) handleBroadcastConfirmChats(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	if state == nil || state.Action != "broadcast_selected_chats" || state.Metadata == "" {
		h.sendText(ctx, userID, messages.MsgBroadcastNoChatsSelected)
		return
	}

	chatCount := len(strings.Split(state.Metadata, ","))

	// ✅ ИСПРАВЛЕНО: 4 аргумента
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_wait_text_with_chats", state.Metadata)

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf(messages.MsgBroadcastChatsSelected, chatCount))
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send broadcast prompt", "error", err)
	}
}

// handleBroadcastClearSelection — очищает выбор чатов
func (h *CallbackHandler) handleBroadcastClearSelection(ctx context.Context, userID int64) {
	// ✅ ИСПРАВЛЕНО: 4 аргумента
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", "")
	h.handleBroadcastSelectChats(ctx, userID)
}

// Вспомогательная функция для отправки текста

// ProcessBroadcastInput обрабатывает введенный текст для рассылки
func (h *CallbackHandler) ProcessBroadcastInput(ctx context.Context, userID int64, text string) {
	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Error("Failed to clear state before broadcast", "error", err)
	}

	statusMsg := maxbot.NewMessage()
	statusMsg.SetUser(userID)
	statusMsg.SetText("🚀 Запускаю рассылку по всем чатам... Это может занять некоторое время.")
	statusMsg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, statusMsg); err != nil {
		h.logger.Error("Failed to send status message", "error", err)
	}

	go func() {
		bgCtx := context.Background()
		success, failed, errs := h.svcBroadcast.BroadcastText(bgCtx, text, "markdown")

		resultText := fmt.Sprintf("✅ Рассылка завершена!\n\nУспешно: %d\nОшибка: %d", success, failed)

		if len(errs) > 0 && len(errs) <= 3 {
			resultText += "\n\nПоследние ошибки:\n"
			for _, e := range errs {
				resultText += fmt.Sprintf("- %v\n", e)
			}
		} else if len(errs) > 3 {
			resultText += fmt.Sprintf("\n... и еще %d ошибок.", len(errs)-3)
		}

		resp := maxbot.NewMessage()
		resp.SetUser(userID)
		resp.SetText(resultText)
		resp.SetFormat("markdown")

		if err := h.bot.Messages.Send(bgCtx, resp); err != nil {
			h.logger.Error("Failed to send broadcast result", "error", err)
		}
	}()
}