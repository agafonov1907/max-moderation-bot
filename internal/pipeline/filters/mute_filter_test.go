package filters

import (
	"context"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"testing"
	"time"
)

func TestMuteFilter_Process(t *testing.T) {
	tests := []struct {
		name        string
		settings    *repository.ChatSettings
		isMuted     bool
		muteExpiry  time.Time
		wantAllowed bool
	}{
		{
			name:        "Mute disabled in settings",
			settings:    &repository.ChatSettings{EnableMute: false},
			isMuted:     true,
			wantAllowed: true,
		},
		{
			name:        "User not muted",
			settings:    &repository.ChatSettings{EnableMute: true},
			isMuted:     false,
			wantAllowed: true,
		},
		{
			name:        "User muted",
			settings:    &repository.ChatSettings{EnableMute: true},
			isMuted:     true,
			muteExpiry:  time.Now().Add(1 * time.Hour),
			wantAllowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settingsRepo := &mockSettingsRepo{settings: tt.settings}
			muteRepo := &mockMuteRepo{isMuted: tt.isMuted, expiresAt: tt.muteExpiry}
			f := NewMuteFilter(muteRepo, settingsRepo)
			res, err := f.Process(context.Background(), pipeline.Payload{ChatID: 123, SenderID: 456})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if res.IsAllowed != tt.wantAllowed {
				t.Errorf("Process() allowed = %v, want %v", res.IsAllowed, tt.wantAllowed)
			}
			if !tt.wantAllowed && res.FilterName != "mute_filter" {
				t.Errorf("Process() filter = %v, want mute_filter", res.FilterName)
			}
		})
	}
}
