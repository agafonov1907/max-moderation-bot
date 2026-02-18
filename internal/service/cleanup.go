package service

import (
	"context"
	"max-moderation-bot/internal/metrics"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
)

func (s *ModerationService) StartCleanupTask(ctx context.Context, bot *maxbot.Api) {
	ticker := time.NewTicker(2 * time.Second)

	cleanup := func() {
		expired, err := s.tempMessageRepo.GetExpired(50)

		if err != nil {
			s.logger.Error("Failed to get expired messages", "error", err)
			return
		}

		if len(expired) == 0 {
			return
		}

		s.logger.Debug("Found expired messages to delete", "count", len(expired))

		var toDeleteIDs []int64
		for _, msg := range expired {
			if _, err := bot.Messages.DeleteMessage(ctx, msg.MessageID); err != nil {

				s.logger.Warn("Failed to delete expired message from chat (will delete from DB)",
					"msg_id", msg.MessageID, "chat_id", msg.ChatID, "error", err)
			} else {
				metrics.IncDeletedMessages("temp_expired")
			}
			toDeleteIDs = append(toDeleteIDs, msg.ID)
		}

		if len(toDeleteIDs) > 0 {
			if err := s.tempMessageRepo.Delete(toDeleteIDs); err != nil {
				s.logger.Error("Failed to delete messages from DB", "error", err)
			}
		}
	}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}

func (s *ModerationService) ScheduleDeletion(_ context.Context, chatID int64, messageID string, duration time.Duration) error {
	return s.tempMessageRepo.Add(chatID, messageID, duration)
}
