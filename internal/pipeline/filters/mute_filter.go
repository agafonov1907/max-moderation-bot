package filters

import (
	"context"
	"fmt"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"time"
)

type MuteFilter struct {
	muteRepo     repository.MuteRepository
	settingsRepo repository.SettingsRepository
}

func NewMuteFilter(muteRepo repository.MuteRepository, settingsRepo repository.SettingsRepository) *MuteFilter {
	return &MuteFilter{
		muteRepo:     muteRepo,
		settingsRepo: settingsRepo,
	}
}
func (f *MuteFilter) Name() string {
	return "mute_filter"
}
func (f *MuteFilter) Process(_ context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	settings, err := f.settingsRepo.GetSettings(payload.ChatID)
	if err != nil {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	if !settings.EnableMute {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	muted, expiresAt, err := f.muteRepo.IsMuted(payload.ChatID, payload.SenderID)
	if err != nil {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	if muted {
		return &pipeline.Result{
			IsAllowed:    false,
			Reason:       fmt.Sprintf(messages.MsgReasonUserMuted, expiresAt.Format(time.RFC822)),
			FilterName:   f.Name(),
			ShouldDelete: true,
		}, nil
	}
	return &pipeline.Result{IsAllowed: true}, nil
}
