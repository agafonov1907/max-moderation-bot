package broadcast

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Service struct {
	logger *slog.Logger
	bot    *maxbot.Api
	tracer trace.Tracer
}

func NewService(logger *slog.Logger, bot *maxbot.Api) *Service {
	return &Service{
		logger: logger,
		bot:    bot,
		tracer: otel.Tracer("broadcast"),
	}
}

// BroadcastText отправляет текстовое сообщение во все чаты, где есть бот.
// Возвращает количество успешных отправок, количество ошибок и список ошибок.
// Для отправки с вложениями используйте прямую логику в handler, так как
// maxbot.Message не предоставляет публичного API для копирования вложений.
func (s *Service) BroadcastText(ctx context.Context, text string, format string) (int, int, []error) {
	ctx, span := s.tracer.Start(ctx, "BroadcastText")
	defer span.End()

	var successCount, failCount int
	var errorsList []error

	// Получаем список чатов с пагинацией
	var marker int64 = 0
	limit := int64(50) // Оптимальный размер страницы

	for {
		chatList, err := s.bot.Chats.GetChats(ctx, limit, marker)
		if err != nil {
			s.logger.Error("Failed to get chat list", "error", err)
			return successCount, failCount, append(errorsList, fmt.Errorf("failed to get chats: %w", err))
		}

		for _, chat := range chatList.Chats {
			// Пропускаем диалоги (личные сообщения), если нужна рассылка только по группам
			if chat.Type == schemes.DIALOG {
				continue
			}

			// Создаем новое сообщение для каждого чата, используя только публичные методы
			newMsg := maxbot.NewMessage().
				SetChat(chat.ChatId).
				SetText(text)
			
			if format != "" {
				newMsg.SetFormat(format)
			}
			
			// Отправляем
			if err := s.bot.Messages.Send(ctx, newMsg); err != nil {
				s.logger.Warn("Failed to send broadcast to chat", 
					"chat_id", chat.ChatId, 
					"title", chat.Title, 
					"error", err)
				failCount++
				errorsList = append(errorsList, fmt.Errorf("chat %d: %w", chat.ChatId, err))
				
				// Небольшая задержка, чтобы не превысить лимиты API (Rate Limit)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			successCount++
			s.logger.Debug("Broadcast sent successfully", 
				"chat_id", chat.ChatId, 
				"title", chat.Title)
			
			// Задержка между сообщениями
			time.Sleep(100 * time.Millisecond)
		}

		if chatList.Marker == nil || *chatList.Marker == marker {
			break
		}
		marker = *chatList.Marker
	}

	return successCount, failCount, errorsList
}