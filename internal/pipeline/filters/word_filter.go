package filters

import (
	"context"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"strings"
)

type WordFilter struct {
	repo          repository.SettingsRepository
	violationRepo repository.ViolationRepository
}

func NewWordFilter(repo repository.SettingsRepository, violationRepo repository.ViolationRepository) *WordFilter {
	return &WordFilter{
		repo:          repo,
		violationRepo: violationRepo,
	}
}
func (f *WordFilter) Name() string {
	return "word_filter"
}
func (f *WordFilter) Process(_ context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	settings, err := f.repo.GetSettings(payload.ChatID)
	if err != nil {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	if !settings.EnableWordFilter {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	lowerMsg := strings.ToLower(payload.Text)
	for _, word := range settings.BlockedWords {
		if strings.Contains(lowerMsg, word) {
			go func(chatID int64) {
				_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "word_violations")
			}(payload.ChatID)
			return &pipeline.Result{
				IsAllowed:  false,
				Reason:     messages.MsgReasonProhibitedWord,
				FilterName: f.Name(),
			}, nil
		}
	}
	return &pipeline.Result{IsAllowed: true}, nil
}
