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
	if state != nil {
		if state.Action == "import_words" {
			h.handleFileImport(ctx, upd.Message.Sender.UserId, state.ChatID, upd.Message.Body.RawAttachments)
			return
		}
		if text == "" {
			h.sendText(ctx, upd.Message.Sender.UserId, messages.MsgOnlyTextSupported)
			return
		}
		h.handleUserInput(ctx, text, upd.Message.Sender.UserId, state)
		return
	}

	// Обработка команды /broadcast
	if strings.HasPrefix(upd.GetCommand(), "/broadcast") {
	h.handleBroadcast(ctx, upd)
	return
	}

	if strings.HasPrefix(text, "/start") || strings.HasPrefix(text, "/menu") {
		h.sendMainMenu(ctx, upd.Message.Sender.UserId)
	}
}

func (h *Handler) handleBotStarted(ctx context.Context, upd *schemes.BotStartedUpdate) {
	start := time.Now()
	defer func() {
		metrics.ObserveUpdateProcessing("bot_started", time.Since(start).Seconds(), nil)
	}()

	h.sendMainMenu(ctx, upd.User.UserId)
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
	var msg string
	switch state.Action {
	case "add_words":
		err = h.svc.AddBlockedWords(ctx, state.ChatID, items)
		msg = messages.MsgAddedBlockedWords
	case "add_domains":
		err = h.svc.AddBlockedDomains(ctx, state.ChatID, items)
		msg = messages.MsgAddedBlockedDomains
	default:
		h.sendText(ctx, userID, messages.MsgUnknownAction)
		return
	}
	if err != nil {
		h.logger.Error("Failed to update settings", "error", err)
		h.sendText(ctx, userID, messages.MsgSettingsUpdateFailed)
		return
	}
	h.sendText(ctx, userID, fmt.Sprintf(messages.MsgSettingsUpdated, msg, len(items)))
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
func (h *Handler) handleBroadcast(ctx context.Context, upd *schemes.MessageCreatedUpdate) {
	userID := upd.Message.Sender.UserId

	// Получаем исходный текст
	text := strings.TrimSpace(upd.Message.Body.Text)

	// Убираем "/broadcast"
	var payload string
	if strings.HasPrefix(text, "/broadcast ") {
		payload = strings.TrimPrefix(text, "/broadcast ")
	} else {
		payload = ""
	}

	// Разделяем на части: первая — список chat_id, остальное — текст
	parts := strings.SplitN(payload, " ", 2)
	var chatIDStr string
	var broadcastText string

	if len(parts) == 2 {
		chatIDStr = parts[0]
		broadcastText = parts[1]
	} else if len(parts) == 1 && parts[0] != "" {
		// Только chat_id без текста
		chatIDStr = parts[0]
		broadcastText = ""
	} else {
		// Нет chat_id — отправляем во все чаты
		chatIDStr = ""
		broadcastText = payload
	}

	// Получаем список всех чатов пользователя
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

	// Определяем целевые чаты
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

			// Проверяем, есть ли доступ к этому чату
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

	// Отправка сообщения
	for _, chatID := range targetChatIDs {
		msg := maxbot.NewMessage()
		msg.SetChat(chatID)
		msg.SetText(broadcastText)
		msg.SetFormat("markdown")

		// Обработка вложений
		for _, raw := range upd.Message.Body.RawAttachments {
			var attMap map[string]interface{}
			if err := json.Unmarshal(raw, &attMap); err != nil {
				continue
			}

			if typeVal, ok := attMap["type"].(string); ok {
				switch typeVal {
				case "image":
					if payload, ok := attMap["payload"].(map[string]interface{}); ok {
						if token, ok := payload["token"].(string); ok && token != "" {
							msg.AddPhoto(&schemes.PhotoTokens{
								Photos: map[string]schemes.PhotoToken{
									"full": {Token: token},
								},
							})
						}
					}
				case "file":
					if payload, ok := attMap["payload"].(map[string]interface{}); ok {
						if token, ok := payload["token"].(string); ok && token != "" {
							msg.AddFile(&schemes.UploadedInfo{Token: token})
						}
					}
				case "video":
					if payload, ok := attMap["payload"].(map[string]interface{}); ok {
						if token, ok := payload["token"].(string); ok && token != "" {
							msg.AddVideo(&schemes.UploadedInfo{Token: token})
						}
					}
				case "audio":
					if payload, ok := attMap["payload"].(map[string]interface{}); ok {
						if token, ok := payload["token"].(string); ok && token != "" {
							msg.AddAudio(&schemes.UploadedInfo{Token: token})
						}
					}
				}
			}
		}

		if _, err := h.bot.Messages.SendWithResult(ctx, msg); err != nil {
			h.logger.Warn("Failed to send broadcast to chat", "chat_id", chatID, "error", err)
		}
	}

	if len(targetChatIDs) == 1 {
		h.sendText(ctx, userID, fmt.Sprintf("✅ Рассылка отправлена в чат %d", targetChatIDs[0]))
	} else {
		h.sendText(ctx, userID, fmt.Sprintf("✅ Рассылка отправлена в %d чат(ов)", len(targetChatIDs)))
	}
}