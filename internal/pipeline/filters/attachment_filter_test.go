package filters

import (
	"context"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"testing"
)

func TestAttachmentFilter_Process(t *testing.T) {
	tests := []struct {
		name        string
		settings    *repository.ChatSettings
		attTypes    []string
		wantAllowed bool
		wantFilter  string
	}{
		{
			name:        "No attachments",
			settings:    &repository.ChatSettings{},
			attTypes:    nil,
			wantAllowed: true,
		},
		{
			name:        "Allowed image",
			settings:    &repository.ChatSettings{RestrictImage: false},
			attTypes:    []string{"image"},
			wantAllowed: true,
		},
		{
			name:        "Restricted image",
			settings:    &repository.ChatSettings{RestrictImage: true},
			attTypes:    []string{"image"},
			wantAllowed: false,
			wantFilter:  "image_filter",
		},
		{
			name:        "Allowed video",
			settings:    &repository.ChatSettings{RestrictVideo: false},
			attTypes:    []string{"video"},
			wantAllowed: true,
		},
		{
			name:        "Restricted video",
			settings:    &repository.ChatSettings{RestrictVideo: true},
			attTypes:    []string{"video"},
			wantAllowed: false,
			wantFilter:  "video_filter",
		},
		{
			name:        "Allowed audio",
			settings:    &repository.ChatSettings{RestrictAudio: false},
			attTypes:    []string{"audio"},
			wantAllowed: true,
		},
		{
			name:        "Restricted audio",
			settings:    &repository.ChatSettings{RestrictAudio: true},
			attTypes:    []string{"audio"},
			wantAllowed: false,
			wantFilter:  "audio_filter",
		},
		{
			name:        "Mixed allowed",
			settings:    &repository.ChatSettings{RestrictImage: false, RestrictVideo: false},
			attTypes:    []string{"image", "video"},
			wantAllowed: true,
		},
		{
			name:        "Mixed restricted (image)",
			settings:    &repository.ChatSettings{RestrictImage: true, RestrictVideo: false},
			attTypes:    []string{"video", "image"},
			wantAllowed: false,
			wantFilter:  "image_filter",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockSettingsRepo{settings: tt.settings}
			mockViolation := &mockViolationRepo{}
			f := NewAttachmentFilter(mockRepo, mockViolation)
			res, err := f.Process(context.Background(), pipeline.Payload{ChatID: 123, AttachmentTypes: tt.attTypes})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if res.IsAllowed != tt.wantAllowed {
				t.Errorf("Process() allowed = %v, want %v", res.IsAllowed, tt.wantAllowed)
			}
			if !tt.wantAllowed && res.FilterName != tt.wantFilter {
				t.Errorf("Process() filter = %v, want %v", res.FilterName, tt.wantFilter)
			}
		})
	}
}
