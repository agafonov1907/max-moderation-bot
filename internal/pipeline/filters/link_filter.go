package filters

import (
	"context"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"max-moderation-bot/internal/utils"
	"regexp"
	"strings"
)

type LinkFilter struct {
	repo          repository.SettingsRepository
	violationRepo repository.ViolationRepository
}

func NewLinkFilter(repo repository.SettingsRepository, violationRepo repository.ViolationRepository) *LinkFilter {
	return &LinkFilter{
		repo:          repo,
		violationRepo: violationRepo,
	}
}
func (f *LinkFilter) Name() string {
	return "link_filter"
}

var urlRegex = regexp.MustCompile(`(?i)(?:https?://)?(?:[\p{L}0-9](?:[\p{L}0-9-]{0,61}[\p{L}0-9])?\.)+[\p{L}0-9][\p{L}0-9-]{0,61}[\p{L}0-9]`)

func (f *LinkFilter) Process(_ context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	settings, err := f.repo.GetSettings(payload.ChatID)
	if err != nil {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	if !settings.EnableLinkFilter {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	urls := urlRegex.FindAllString(payload.Text, -1)
	if len(urls) == 0 {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	for _, url := range urls {
		lowerURL := strings.ToLower(url)
		for _, domain := range settings.BlockedDomains {
			cleanedDomain := utils.NormalizeDomain(domain)
			if cleanedDomain == "" {
				continue
			}
			if strings.Contains(lowerURL, cleanedDomain) {
				go func(chatID int64) {
					_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "link_violations")
				}(payload.ChatID)
				return &pipeline.Result{
					IsAllowed:  false,
					Reason:     messages.MsgReasonProhibitedDomain,
					FilterName: f.Name(),
				}, nil
			}
		}
	}
	return &pipeline.Result{IsAllowed: true}, nil
}
