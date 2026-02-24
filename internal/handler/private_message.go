package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/metrics"
	"max-moderation-bot/internal/repository"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

func (h *Handler) handlePrivateMessage(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	// === ПРОВЕРКА НА АДМИНИСТРАТОРА ===
	isAdmin := false
	for _, adminID := range h.config.AdminUserIDs {
		if adminID == upd.Message.Sender.UserId {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		h.logger.Warn("Non-admin user access denied", "user_id", upd.Message.Sender.UserId)
		return // Игнорируем всех неадминов
	}
	// ===================================

	var attachmentTypes []string
	if len(upd.Message.Body.RawAttachments) > 0 {
		for _, raw := range upd.Message.Body.RawAttachments {
			var attMap map[string]interface{}
			if err := json.Unmarshal(raw, &attMap); err == nil {
				if typeVal, ok := attMap["type"].(string); ok {
					attachmentTypes = append(attachmentTypes, typeVal)
				}
			}
		}
	}

	h.logger.Info("Received private message",
		"text", upd.Message.Body.Text,
		"sender", upd.Message.Sender.UserId,
		"attachments", attachmentTypes,
	)

	text := strings.TrimSpace(upd.Message.Body.Text)
	state, _ := h.userStateRepo.GetState(upd.Message.Sender.UserId)

	h.logger.Debug("DEBUG private_message",
	"user_id", upd.Message.Sender.UserId,
	"state_exists", state != nil,
	"state_action", func() string {
		if state != nil {
			return fmt.Sprintf("'%s' (len=%d)", state.Action, len(state.Action))
		}
		return "nil"
	}(),
	"has_attachments", len(upd.Message.Body.RawAttachments) > 0,
	"attachment_count", len(upd.Message.Body.RawAttachments),
	"text", text,
	"text_len", len(text),
	"command", upd.GetCommand(),
)

	// 1. Обработка состояний (FSM)
	if state != nil {
		switch state.Action {
		case "import_words":
			h.handleFileImport(ctx, upd.Message.Sender.UserId, state.ChatID, upd.Message.Body.RawAttachments)
			return

		case "broadcast_wait_text":
			// Обработка ввода текста для рассылки
			if strings.HasPrefix(text, "/cancel") || text == "❌ Отмена" {
				h.userStateRepo.ClearState(upd.Message.Sender.UserId)
				h.sendText(ctx, upd.Message.Sender.UserId, "Рассылка отменена.")
				return
			}
			// ✅ ПРОВЕРКА: текст ИЛИ вложения
			if text == "" && len(upd.Message.Body.RawAttachments) == 0 {
				h.sendText(ctx, upd.Message.Sender.UserId, "Сообщение не может быть пустым. Отправьте текст или файл.")
				return
			}

			// Запускаем рассылку с введенным текстом и текущими вложениями
			h.performBroadcast(ctx, upd.Message.Sender.UserId, "", text, upd.Message.Body.RawAttachments)

			// Очищаем состояние после запуска
			h.userStateRepo.ClearState(upd.Message.Sender.UserId)
			return

		default:
			// Неизвестное состояние или ввод текста для add_words/add_domains
			if text == "" {
				h.sendText(ctx, upd.Message.Sender.UserId, messages.MsgOnlyTextSupported)
				return
			}
			h.handleUserInput(ctx, text, upd.Message.Sender.UserId, state)
			return
		}
	}

	// 2. Обработка команд вне состояний
	if strings.HasPrefix(upd.GetCommand(), "/broadcast") {
		h.handleBroadcastCommand(ctx, upd)
		return
	}

	if strings.HasPrefix(text, "/start") || strings.HasPrefix(text, "/menu") {
		h.sendMainMenu(ctx, upd.Message.Sender.UserId)
		return
	}

	// Если просто текст без состояния и не команда
	h.sendText(ctx, upd.Message.Sender.UserId, "Неизвестная команда. Используйте /menu.")
}

func (h *Handler) handleBotStarted(ctx context.Context, upd *schemes.BotStartedUpdate) {
	// === ПРОВЕРКА НА АДМИНИСТРАТОРА ===
	isAdmin := false
	for _, adminID := range h.config.AdminUserIDs {
		if adminID == upd.User.UserId {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		h.logger.Warn("Non-admin user started bot", "user_id", upd.User.UserId)
		return
	}
	// ===================================

	start := time.Now()
	defer func() {
		metrics.ObserveUpdateProcessing("bot_started", time.Since(start).Seconds(), nil)
	}()

	h.sendMainMenu(ctx, upd.User.UserId)
}

// handleBroadcastCommand обрабатывает команду /broadcast [chat_ids] текст
func (h *Handler) handleBroadcastCommand(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	userID := upd.Message.Sender.UserId
	text := strings.TrimSpace(upd.Message.Body.Text)

	// Удаляем саму команду
	payload := strings.TrimPrefix(text, "/broadcast")
	payload = strings.TrimSpace(payload)

	var chatIDStr string
	var broadcastText string

	// Пытаемся разделить на ID чатов и текст
	parts := strings.SplitN(payload, " ", 2)

	if len(parts) == 2 {
		if isLikelyChatIDs(parts[0]) {
			chatIDStr = parts[0]
			broadcastText = parts[1]
		} else {
			chatIDStr = ""
			broadcastText = payload
		}
	} else if len(parts) == 1 && parts[0] != "" {
		if isLikelyChatIDs(parts[0]) {
			h.sendText(ctx, userID, "❌ Указаны ID чатов, но нет текста сообщения.")
			return
		}
		chatIDStr = ""
		broadcastText = parts[0]
	} else {
		h.sendText(ctx, userID, "❌ Укажите текст сообщения после команды /broadcast.")
		return
	}

	h.performBroadcast(ctx, userID, chatIDStr, broadcastText, upd.Message.Body.RawAttachments)
}

// performBroadcast - единая точка отправки рассылок (и из FSM, и из команды)
func (h *Handler) performBroadcast(ctx context.Context, userID int64, chatIDStr string, broadcastText string, rawAttachments []json.RawMessage) {
	if broadcastText == "" && len(rawAttachments) == 0 {
		h.sendText(ctx, userID, "❌ Текст сообщения или вложения не могут быть пустыми.")
		return
	}

	allManagedChats, err := h.svc.GetManagedChats(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get managed chats", "error", err)
		h.sendText(ctx, userID, "❌ Ошибка при получении списка чатов.")
		return
	}

	if len(allManagedChats) == 0 {
		h.sendText(ctx, userID, "У вас нет привязанных чатов.")
		return
	}

	var targetChatIDs []int64

	if chatIDStr == "" {
		// Отправка во все чаты
		targetChatIDs = allManagedChats
	} else {
		// Парсим список chat_id (через запятую)
		idList := strings.Split(chatIDStr, ",")
		for _, idStr := range idList {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			var chatID int64
			if _, err := fmt.Sscanf(idStr, "%d", &chatID); err != nil {
				h.sendText(ctx, userID, fmt.Sprintf("❌ Неверный формат ID чата: %s", idStr))
				return
			}

			hasAccess := false
			for _, id := range allManagedChats {
				if id == chatID {
					hasAccess = true
					break
				}
			}
			if !hasAccess {
				h.sendText(ctx, userID, fmt.Sprintf("❌ У вас нет доступа к чату %d", chatID))
				return
			}
			targetChatIDs = append(targetChatIDs, chatID)
		}
	}

	if len(targetChatIDs) == 0 {
		h.sendText(ctx, userID, "❌ Не выбрано чатов для рассылки.")
		return
	}

	h.sendText(ctx, userID, fmt.Sprintf("🚀 Запуск рассылки в %d чат(ов)...", len(targetChatIDs)))

	successCount := 0
	failCount := 0

	for _, chatID := range targetChatIDs {
		msg := maxbot.NewMessage()
		msg.SetChat(chatID)
		msg.SetText(broadcastText)
		msg.SetFormat("markdown")

		// Обработка вложений
		for _, raw := range rawAttachments {
			var attMap map[string]interface{}
			if err := json.Unmarshal(raw, &attMap); err != nil {
				continue
			}
			if typeVal, ok := attMap["type"].(string); ok {
				if payload, ok := attMap["payload"].(map[string]interface{}); ok {
					if token, ok := payload["token"].(string); ok && token != "" {
						switch typeVal {
						case "image":
							msg.AddPhoto(&schemes.PhotoTokens{Photos: map[string]schemes.PhotoToken{"full": {Token: token}}})
						case "file":
							msg.AddFile(&schemes.UploadedInfo{Token: token})
						case "video":
							msg.AddVideo(&schemes.UploadedInfo{Token: token})
						case "audio":
							msg.AddAudio(&schemes.UploadedInfo{Token: token})
						}
					}
				}
			}
		}

		if _, err := h.bot.Messages.SendWithResult(ctx, msg); err != nil {
			h.logger.Warn("Failed to send broadcast", "chat_id", chatID, "error", err)
			failCount++
		} else {
			successCount++
		}

		// Небольшая пауза, чтобы не спамить API
		time.Sleep(100 * time.Millisecond)
	}

	report := fmt.Sprintf("✅ Рассылка завершена!\nУспешно: %d\nОшибка: %d", successCount, failCount)
	h.sendText(ctx, userID, report)
}

// isLikelyChatIDs проверяет, похожа ли строка на список ID (цифры и запятые)
func isLikelyChatIDs(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != ',' && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func (h *Handler) handleUserInput(ctx context.Context, text string, userID int64, state *repository.UserState) {
	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Error("Failed to delete user state", "error", err)
	}

	rawItems := strings.Split(text, ",")
	var items []string
	for _, item := range rawItems {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}

	if len(items) == 0 {
		h.sendText(ctx, userID, messages.MsgNoValidItems)
		return
	}

	var err error
	var msgKey string
	switch state.Action {
	case "add_words":
		err = h.svc.AddBlockedWords(ctx, state.ChatID, items)
		msgKey = messages.MsgAddedBlockedWords
	case "add_domains":
		err = h.svc.AddBlockedDomains(ctx, state.ChatID, items)
		msgKey = messages.MsgAddedBlockedDomains
	default:
		h.sendText(ctx, userID, messages.MsgUnknownAction)
		return
	}

	if err != nil {
		h.logger.Error("Failed to update settings", "error", err)
		h.sendText(ctx, userID, messages.MsgSettingsUpdateFailed)
		return
	}

	h.sendText(ctx, userID, fmt.Sprintf(messages.MsgSettingsUpdated, msgKey, len(items)))
	h.callbackHandler.HandleManageGroup(ctx, state.ChatID, userID)
}

func (h *Handler) sendText(ctx context.Context, userID int64, text string) {
	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text message", "error", err)
	}
}

func (h *Handler) sendMainMenu(ctx context.Context, userID int64) {
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnMyGroups, schemes.DEFAULT, "my_groups")
	kb.AddRow().AddCallback(messages.BtnAddGroup, schemes.POSITIVE, "add_group")
	// Добавляем кнопку рассылки
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

func (h *Handler) handleFileImport(ctx context.Context, userID, chatID int64, rawAttachments []json.RawMessage) {
	if len(rawAttachments) == 0 {
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgImportFileRequired)
		return
	}

	var fileURL string
	var fileName string
	for _, raw := range rawAttachments {
		var attMap map[string]interface{}
		if err := json.Unmarshal(raw, &attMap); err != nil {
			continue
		}
		typ, _ := attMap["type"].(string)
		if typ == "file" || typ == "document" {
			if payload, ok := attMap["payload"].(map[string]interface{}); ok {
				if urlVal, ok := payload["url"].(string); ok {
					fileURL = urlVal
				}
				for _, key := range []string{"name", "filename", "title"} {
					if name, ok := payload[key].(string); ok && name != "" {
						fileName = name
						break
					}
				}
			}
			if fileURL == "" {
				if urlVal, ok := attMap["url"].(string); ok {
					fileURL = urlVal
				}
			}
			if fileName == "" {
				for _, key := range []string{"name", "filename", "title"} {
					if name, ok := attMap[key].(string); ok && name != "" {
						fileName = name
						break
					}
				}
			}
			if fileURL != "" {
				break
			}
		}
	}

	if fileURL == "" {
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgImportFileRequired)
		return
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" {
		if u, err := url.Parse(fileURL); err == nil {
			ext = strings.ToLower(filepath.Ext(u.Path))
		}
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		h.logger.Error("Failed to download file", "url", fileURL, "error", err)
		h.sendTextWithBack(ctx, userID, chatID, fmt.Sprintf(messages.MsgImportError, err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			h.logger.Warn("Failed to close response body", "error", err)
		}
	}()

	peekBuf := bufio.NewReader(resp.Body)
	sniff, _ := peekBuf.Peek(512)
	contentType := http.DetectContentType(sniff)
	headerType := resp.Header.Get("Content-Type")

	isTxtExtension := ext == ".txt"
	isTxtContent := strings.HasPrefix(contentType, "text/plain") || strings.HasPrefix(headerType, "text/plain")

	if !isTxtContent {
		h.logger.Warn("Import blocked: content is not text", "url", fileURL, "content_type", contentType, "header_type", headerType)
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgImportFileRequired)
		return
	}

	if !isTxtExtension {
		h.logger.Warn("Import blocked: strictly .txt required", "url", fileURL, "ext", ext)
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgImportFileRequired)
		return
	}

	scanner := bufio.NewScanner(peekBuf)
	words, skippedCount, err := parseWordsFile(scanner)
	if err != nil {
		h.logger.Error("Error scanning file", "error", err)
		h.sendTextWithBack(ctx, userID, chatID, fmt.Sprintf(messages.MsgImportError, err))
		return
	}

	if len(words) == 0 {
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgImportEmpty)
		return
	}

	if err := h.svc.AddBlockedWords(ctx, chatID, words); err != nil {
		h.logger.Error("Failed to save imported words", "error", err)
		h.sendTextWithBack(ctx, userID, chatID, messages.MsgSettingsUpdateFailed)
		return
	}

	if err := h.userStateRepo.ClearState(userID); err != nil {
		h.logger.Warn("Failed to clear state after successful import", "error", err)
	}

	var msgText string
	if skippedCount > 0 {
		msgText = fmt.Sprintf(messages.MsgImportPartialSuccess, len(words), skippedCount)
	} else {
		msgText = fmt.Sprintf(messages.MsgImportSuccess, len(words))
	}

	h.sendText(ctx, userID, msgText)
	h.callbackHandler.HandleManageGroup(ctx, chatID, userID)
}

func (h *Handler) sendTextWithBack(ctx context.Context, userID, chatID int64, text string) {
	kb := h.bot.Messages.NewKeyboardBuilder()
	kb.AddRow().AddCallback(messages.BtnBack, schemes.DEFAULT, fmt.Sprintf("manage_%d", chatID))

	msg := maxbot.NewMessage()
	msg.SetUser(userID)
	msg.SetText(text)
	msg.SetFormat("markdown")
	msg.AddKeyboard(kb)
	if err := h.bot.Messages.Send(ctx, msg); err != nil {
		h.logger.Error("Failed to send text message with back button", "error", err)
	}
}

func parseWordsFile(scanner *bufio.Scanner) ([]string, int, error) {
	var words []string
	var skippedCount int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.Contains(line, " ") {
			skippedCount++
			continue
		}
		words = append(words, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	return words, skippedCount, nil
}