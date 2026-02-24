package callbacks

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// handleBroadcastStart начинает процесс рассылки
func (h *CallbackHandler) handleBroadcastStart(ctx context.Context, userID int64) {
	// Устанавливаем состояние ожидания текста рассылки
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

// handleBroadcastCancel отменяет рассылку
func (h *CallbackHandler) handleBroadcastCancel(ctx context.Context, userID int64) {
	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Error("Failed to clear state", "error", err)
	}

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText("Рассылка отменена.")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send cancel message", "error", err)
	}
}

// ProcessBroadcastInput обрабатывает введенный текст для рассылки
// Вызывается из handlePrivateMessage в private_message.go при состоянии broadcast_wait_text
func (h *CallbackHandler) ProcessBroadcastInput(ctx context.Context, userID int64, text string) {
	// Очищаем состояние перед запуском долгой операции
	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Error("Failed to clear state before broadcast", "error", err)
	}

	// Сообщаем пользователю о начале процесса
	statusMsg := maxbot.NewMessage()
	statusMsg.SetUser(userID)
	statusMsg.SetText("🚀 Запускаю рассылку по всем чатам... Это может занять некоторое время.")
	statusMsg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, statusMsg); err != nil {
		h.logger.Error("Failed to send status message", "error", err)
	}

	// ✅ ЗАПУСКАЕМ РАССЫЛКУ АСИНХРОННО
	// Важно: BroadcastText принимает (text string, format string), а не *maxbot.Message
	go func() {
		bgCtx := context.Background()
		
		// ✅ ИСПРАВЛЕНО: Вызываем BroadcastText с правильными параметрами
		success, failed, errs := h.svcBroadcast.BroadcastText(bgCtx, text, "markdown")

		// Формируем отчет о результатах
		resultText := fmt.Sprintf("✅ Рассылка завершена!\n\nУспешно: %d\nОшибка: %d", success, failed)
		
		if len(errs) > 0 && len(errs) <= 3 {
			resultText += "\n\nПоследние ошибки:\n"
			for _, e := range errs {
				resultText += fmt.Sprintf("- %v\n", e)
			}
		} else if len(errs) > 3 {
			resultText += fmt.Sprintf("\n... и еще %d ошибок.", len(errs)-3)
		}

		// Отправляем отчет пользователю
		resp := maxbot.NewMessage()
		resp.SetUser(userID)
		resp.SetText(resultText)
		resp.SetFormat("markdown")

		if err := h.bot.Messages.Send(bgCtx, resp); err != nil {
			h.logger.Error("Failed to send broadcast result", "error", err)
		}
	}()
}