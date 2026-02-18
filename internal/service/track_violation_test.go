package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestModerationService_TrackViolation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		chatID        int64
		userID        int64
		violationType string
		setupMocks    func() *MockViolationRepository
		wantMute      bool
		wantErr       bool
	}{
		{
			name:          "Below Limit - violation added, no mute",
			chatID:        123,
			userID:        456,
			violationType: "link_filter",
			setupMocks: func() *MockViolationRepository {
				return &MockViolationRepository{
					AddViolationFunc: func(ctx context.Context, chatID, userID int64, violationType string) error {
						return nil
					},
					CountViolationsSinceFunc: func(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
						return 4, nil
					},
				}
			},
			wantMute: false,
			wantErr:  false,
		},
		{
			name:          "At Limit - violation added, trigger mute",
			chatID:        123,
			userID:        456,
			violationType: "word_filter",
			setupMocks: func() *MockViolationRepository {
				return &MockViolationRepository{
					AddViolationFunc: func(ctx context.Context, chatID, userID int64, violationType string) error {
						return nil
					},
					CountViolationsSinceFunc: func(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
						expectedSince := time.Now().Add(-24 * time.Hour)
						diff := expectedSince.Sub(since)
						if diff < -1*time.Second || diff > 1*time.Second {
							t.Errorf("CountViolationsSince called with wrong time. Got %v, want ~ %v", since, expectedSince)
						}
						return 5, nil
					},
				}
			},
			wantMute: true,
			wantErr:  false,
		},
		{
			name:          "Above Limit - violation added, trigger mute",
			chatID:        123,
			userID:        456,
			violationType: "word_filter",
			setupMocks: func() *MockViolationRepository {
				return &MockViolationRepository{
					AddViolationFunc: func(ctx context.Context, chatID, userID int64, violationType string) error {
						return nil
					},
					CountViolationsSinceFunc: func(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
						return 10, nil
					},
				}
			},
			wantMute: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violationRepo := tt.setupMocks()
			svc := NewModerationService(logger, nil, nil, nil, nil, nil, violationRepo, nil)

			mute, _, err := svc.TrackViolation(context.Background(), tt.chatID, tt.userID, tt.violationType)

			if (err != nil) != tt.wantErr {
				t.Errorf("TrackViolation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if mute != tt.wantMute {
				t.Errorf("TrackViolation() mute = %v, want %v", mute, tt.wantMute)
			}
		})
	}
}
