package filters

import (
	"context"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
)

type AttachmentFilter struct {
	repo          repository.SettingsRepository
	violationRepo repository.ViolationRepository
}

func NewAttachmentFilter(repo repository.SettingsRepository, violationRepo repository.ViolationRepository) *AttachmentFilter {
	return &AttachmentFilter{
		repo:          repo,
		violationRepo: violationRepo,
	}
}
func (f *AttachmentFilter) Name() string {
	return "attachment_filter"
}
func (f *AttachmentFilter) Process(_ context.Context, payload pipeline.Payload) (*pipeline.Result, error) {
	if len(payload.AttachmentTypes) == 0 {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	settings, err := f.repo.GetSettings(payload.ChatID)
	if err != nil {
		return &pipeline.Result{IsAllowed: true}, nil
	}
	for _, attType := range payload.AttachmentTypes {
		if attType == "image" && settings.RestrictImage {
			go func(chatID int64) {
				_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "image_violations")
			}(payload.ChatID)
			return &pipeline.Result{IsAllowed: false, Reason: messages.MsgReasonImageRestricted, FilterName: "image_filter"}, nil
		}
		if attType == "video" && settings.RestrictVideo {
			go func(chatID int64) {
				_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "video_violations")
			}(payload.ChatID)
			return &pipeline.Result{IsAllowed: false, Reason: messages.MsgReasonVideoRestricted, FilterName: "video_filter"}, nil
		}
		if attType == "audio" && settings.RestrictAudio {
			go func(chatID int64) {
				_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "audio_violations")
			}(payload.ChatID)
			return &pipeline.Result{IsAllowed: false, Reason: messages.MsgReasonAudioRestricted, FilterName: "audio_filter"}, nil
		}
		if (attType == "file" || attType == "document") && settings.RestrictFile {
			go func(chatID int64) {
				_ = f.violationRepo.IncrementChatStat(context.Background(), chatID, "file_violations")
			}(payload.ChatID)
			return &pipeline.Result{IsAllowed: false, Reason: messages.MsgReasonFileRestricted, FilterName: "file_filter"}, nil
		}
	}
	return &pipeline.Result{IsAllowed: true}, nil
}
