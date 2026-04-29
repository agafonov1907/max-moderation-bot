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

// Константа для пагинации: количество чатов на одной странице
const broadcastChatsPerPage = 10

// handleBroadcastStart — показывает экран подтверждения перед рассылкой во все чаты
func (h *CallbackHandler) handleBroadcastStart(ctx context.Context, userID int64) {
	// Получаем количество чатов для информирования
	chatInfos, err := h.svc.GetManagedChatsWithNames(ctx, userID)
	chatCount := 0
	if err == nil {
		chatCount = len(chatInfos)
	}

	// ✅ Показываем экран подтверждения
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf("⚠️ **Подтверждение рассылки**\n\n"+
		"Вы собираетесь отправить сообщение во **все %d чат(ов)**.\n\n"+
		"Сейчас вы введёте текст сообщения, и оно будет отправлено во все ваши чаты.\n\n"+
		"⚠️ **Это действие нельзя отменить после отправки!**\n\n"+
		"Нажмите **✅ Подтвердить** для продолжения или **❌ Отмена**.", chatCount))
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().
		AddCallback("✅ Подтвердить", schemes.POSITIVE, "broadcast_start_final").
		AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	// Сохраняем состояние ожидания финального подтверждения
	_ = h.userStateRepo.SetState(userID, 0, "broadcast_wait_confirm_all")

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send confirm prompt", "error", err)
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

	// ✅ Добавляем кнопку возврата в главное меню
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("🔙 В меню", schemes.DEFAULT, "main_menu")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send cancel message", "error", err)
	}
}

// handleBroadcastSelectChats — показывает список чатов с чекбоксами, названиями и пагинацией
func (h *CallbackHandler) handleBroadcastSelectChats(ctx context.Context, userID int64, page int) {
	if page < 1 {
		page = 1
	}

	// Получаем все чаты с названиями
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

	// === ПАГИНАЦИЯ ===
	totalChats := len(chatInfos)
	totalPages := (totalChats + broadcastChatsPerPage - 1) / broadcastChatsPerPage
	if page > totalPages {
		page = totalPages
	}
	if page < 1 {
		page = 1
	}

	startIdx := (page - 1) * broadcastChatsPerPage
	endIdx := startIdx + broadcastChatsPerPage
	if endIdx > totalChats {
		endIdx = totalChats
	}
	pageChats := chatInfos[startIdx:endIdx]

	// Получаем текущие выбранные чаты из состояния
	// Формат Metadata: "selected:123,456;page:2"
	state, _ := h.userStateRepo.GetState(userID)
	selectedChats := make(map[int64]bool)

	if state != nil && state.Action == "broadcast_selected_chats" && state.Metadata != "" {
		parts := strings.Split(state.Metadata, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "selected:") {
				idsStr := strings.TrimPrefix(part, "selected:")
				if idsStr != "" {
					for _, idStr := range strings.Split(idsStr, ",") {
						if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
							selectedChats[id] = true
						}
					}
				}
			}
		}
	}

	// Строим клавиатуру с чекбоксами
	kb := h.bot.Messages.NewKeyboardBuilder()
	row := kb.AddRow()

	for i, chat := range pageChats {
		mark := "⬜"
		if selectedChats[chat.ID] {
			mark = "✅"
		}

		// Показываем название + ID (если название пустое — fallback)
		chatLabel := chat.Name
		if chatLabel == "" {
			chatLabel = fmt.Sprintf("Чат %d", chat.ID)
		}
		// Обрезаем длинное название для кнопки (лимит MAX API ~40 символов)
		if len(chatLabel) > 30 {
			chatLabel = chatLabel[:27] + "..."
		}

		row.AddCallback(
			fmt.Sprintf("%s %s", mark, chatLabel),
			schemes.DEFAULT,
			fmt.Sprintf("broadcast_toggle_chat_%d", chat.ID),
		)

		// По 2 кнопки в ряд
		if (i+1)%2 == 0 && i < len(pageChats)-1 {
			row = kb.AddRow()
		}
	}

	// === КНОПКИ ПАГИНАЦИИ ===
	if totalPages > 1 {
		paginationRow := kb.AddRow()
		if page > 1 {
			paginationRow.AddCallback("⬅️ Назад", schemes.DEFAULT, "broadcast_prev_page")
		}
		paginationRow.AddCallback(
			fmt.Sprintf("📄 Стр. %d/%d", page, totalPages),
			schemes.DEFAULT,
			"broadcast_page_info",
		)
		if page < totalPages {
			paginationRow.AddCallback("Вперед ➡️", schemes.DEFAULT, "broadcast_next_page")
		}
	}

	// === КНОПКИ УПРАВЛЕНИЯ ===
	kb.AddRow().
		AddCallback(messages.BtnBroadcastSendAll, schemes.POSITIVE, "broadcast_send_all").
		AddCallback(messages.BtnBroadcastClear, schemes.NEGATIVE, "broadcast_clear_selection")
	kb.AddRow().
		AddCallback(messages.BtnBroadcastConfirm, schemes.POSITIVE, "broadcast_confirm_chats").
		AddCallback("❌ Отмена", schemes.NEGATIVE, "main_menu")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)

	// Заголовок с информацией о пагинации
	header := fmt.Sprintf("📋 **Выберите чаты для рассылки** (стр. %d/%d)\n\n", page, totalPages)
	if totalPages > 1 {
		header += fmt.Sprintf("_Показаны чаты %d–%d из %d_\n\n", startIdx+1, endIdx, totalChats)
	}
	msg.SetText(header + "Нажмите на чат, чтобы отметить/снять отметку.\n\nКогда закончите, нажмите «✅ Готово».")
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)

	// Сохраняем текущую страницу и выбранные чаты в состоянии
	selectedIDs := make([]string, 0, len(selectedChats))
	for id := range selectedChats {
		selectedIDs = append(selectedIDs, fmt.Sprintf("%d", id))
	}
	metadata := fmt.Sprintf("selected:%s;page:%d", strings.Join(selectedIDs, ","), page)
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", metadata)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send chat selection", "error", err)
	}
}

// handleBroadcastToggleChat — переключает выбор чата (сохраняет текущую страницу)
func (h *CallbackHandler) handleBroadcastToggleChat(ctx context.Context, userID, chatID int64) {
	state, _ := h.userStateRepo.GetState(userID)

	// Парсим текущее состояние
	selectedChats := make(map[int64]bool)
	currentPage := 1

	if state != nil && state.Action == "broadcast_selected_chats" && state.Metadata != "" {
		parts := strings.Split(state.Metadata, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "selected:") {
				idsStr := strings.TrimPrefix(part, "selected:")
				if idsStr != "" {
					for _, idStr := range strings.Split(idsStr, ",") {
						if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
							selectedChats[id] = true
						}
					}
				}
			} else if strings.HasPrefix(part, "page:") {
				pageStr := strings.TrimPrefix(part, "page:")
				if p, err := strconv.Atoi(pageStr); err == nil {
					currentPage = p
				}
			}
		}
	}

	// Переключаем чат
	if selectedChats[chatID] {
		delete(selectedChats, chatID)
	} else {
		selectedChats[chatID] = true
	}

	// Сохраняем обратно в состояние (с текущей страницей)
	selectedIDs := make([]string, 0, len(selectedChats))
	for id := range selectedChats {
		selectedIDs = append(selectedIDs, fmt.Sprintf("%d", id))
	}
	metadata := fmt.Sprintf("selected:%s;page:%d", strings.Join(selectedIDs, ","), currentPage)
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", metadata)

	// Обновляем клавиатуру (на той же странице)
	h.handleBroadcastSelectChats(ctx, userID, currentPage)
}

// handleBroadcastConfirmChats — показывает экран подтверждения перед рассылкой
func (h *CallbackHandler) handleBroadcastConfirmChats(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	if state == nil || state.Action != "broadcast_selected_chats" || state.Metadata == "" {
		h.sendText(ctx, userID, messages.MsgBroadcastNoChatsSelected)
		return
	}

	// Парсим выбранные чаты из Metadata (формат: "selected:123,456;page:2")
	var selectedIDs []string
	parts := strings.Split(state.Metadata, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "selected:") {
			idsStr := strings.TrimPrefix(part, "selected:")
			if idsStr != "" {
				selectedIDs = strings.Split(idsStr, ",")
			}
			break
		}
	}

	if len(selectedIDs) == 0 {
		h.sendText(ctx, userID, messages.MsgBroadcastNoChatsSelected)
		return
	}

	chatCount := len(selectedIDs)

	// ✅ НОВОЕ: Показываем экран подтверждения
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(fmt.Sprintf("⚠️ **Подтверждение рассылки**\n\n"+
		"Вы выбрали **%d чат(ов)** для рассылки.\n\n"+
		"Сейчас вы введёте текст сообщения, и оно будет отправлено во все выбранные чаты.\n\n"+
		"⚠️ **Это действие нельзя отменить после отправки!**\n\n"+
		"Нажмите **✅ Подтвердить** для продолжения или **❌ Отмена**.", chatCount))
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().
		AddCallback("✅ Подтвердить", schemes.POSITIVE, "broadcast_confirm_final").
		AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	// Сохраняем состояние ожидания финального подтверждения
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_wait_confirm", state.Metadata)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send confirm prompt", "error", err)
	}
}

// handleBroadcastClearSelection — очищает выбор чатов (сохраняет текущую страницу)
func (h *CallbackHandler) handleBroadcastClearSelection(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	currentPage := 1

	// Сохраняем текущую страницу при очистке
	if state != nil && state.Metadata != "" {
		parts := strings.Split(state.Metadata, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "page:") {
				pageStr := strings.TrimPrefix(part, "page:")
				if p, err := strconv.Atoi(pageStr); err == nil {
					currentPage = p
					break
				}
			}
		}
	}

	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_selected_chats", fmt.Sprintf("selected:;page:%d", currentPage))
	h.handleBroadcastSelectChats(ctx, userID, currentPage)
}

// handleBroadcastPrevPage — переход на предыдущую страницу
func (h *CallbackHandler) handleBroadcastPrevPage(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	currentPage := 1

	if state != nil && state.Metadata != "" {
		parts := strings.Split(state.Metadata, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "page:") {
				pageStr := strings.TrimPrefix(part, "page:")
				if p, err := strconv.Atoi(pageStr); err == nil {
					currentPage = p
					break
				}
			}
		}
	}

	newPage := currentPage - 1
	if newPage < 1 {
		newPage = 1
	}

	h.handleBroadcastSelectChats(ctx, userID, newPage)
}

// handleBroadcastNextPage — переход на следующую страницу
func (h *CallbackHandler) handleBroadcastNextPage(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	currentPage := 1
	totalChats := 0

	// Получаем общее количество чатов для расчёта страниц
	chatInfos, err := h.svc.GetManagedChatsWithNames(ctx, userID)
	if err == nil {
		totalChats = len(chatInfos)
	}

	if state != nil && state.Metadata != "" {
		parts := strings.Split(state.Metadata, ";")
		for _, part := range parts {
			if strings.HasPrefix(part, "page:") {
				pageStr := strings.TrimPrefix(part, "page:")
				if p, err := strconv.Atoi(pageStr); err == nil {
					currentPage = p
					break
				}
			}
		}
	}

	totalPages := (totalChats + broadcastChatsPerPage - 1) / broadcastChatsPerPage
	newPage := currentPage + 1
	if newPage > totalPages {
		newPage = totalPages
	}

	h.handleBroadcastSelectChats(ctx, userID, newPage)
}

// handleBroadcastFinalConfirm — финальное подтверждение, переход к вводу текста (для выбранных чатов)
func (h *CallbackHandler) handleBroadcastFinalConfirm(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	if state == nil || state.Action != "broadcast_wait_confirm" || state.Metadata == "" {
		h.sendText(ctx, userID, "❌ Сессия истекла. Начните заново через /start")
		return
	}

	// Извлекаем выбранные чаты из Metadata (формат: "selected:123,456;page:2")
	var selectedIDs []string
	parts := strings.Split(state.Metadata, ";")
	for _, part := range parts {
		if strings.HasPrefix(part, "selected:") {
			idsStr := strings.TrimPrefix(part, "selected:")
			if idsStr != "" {
				selectedIDs = strings.Split(idsStr, ",")
			}
			break
		}
	}

	if len(selectedIDs) == 0 {
		h.sendText(ctx, userID, messages.MsgBroadcastNoChatsSelected)
		return
	}

	// Переходим к состоянию ожидания текста
	selectedChatsStr := strings.Join(selectedIDs, ",")
	_ = h.userStateRepo.SetStateWithMetadata(userID, 0, "broadcast_wait_text_with_chats", selectedChatsStr)

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText("✍️ **Введите текст рассылки**\n\n" +
		"Отправьте сообщение (можно с фото/файлом).\n\n" +
		"Поддерживается Markdown.\n\n" +
		"/cancel — отмена")
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text prompt", "error", err)
	}
}

// ✅ НОВАЯ ФУНКЦИЯ: Финальное подтверждение для рассылки во все чаты
func (h *CallbackHandler) handleBroadcastStartFinal(ctx context.Context, userID int64) {
	state, _ := h.userStateRepo.GetState(userID)
	if state == nil || state.Action != "broadcast_wait_confirm_all" {
		h.sendText(ctx, userID, "❌ Сессия истекла. Начните заново через /start")
		return
	}

	// Переходим к состоянию ожидания текста (для всех чатов)
	_ = h.userStateRepo.SetState(userID, 0, "broadcast_wait_text")

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText("✍️ **Введите текст рассылки**\n\n" +
		"Отправьте сообщение (можно с фото/файлом).\n\n" +
		"Поддерживается Markdown.\n\n" +
		"/cancel — отмена")
	msg.SetFormat("markdown")

	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback("❌ Отмена", schemes.NEGATIVE, "broadcast_cancel")
	msg.AddKeyboard(kb)

	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text prompt", "error", err)
	}
}

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
