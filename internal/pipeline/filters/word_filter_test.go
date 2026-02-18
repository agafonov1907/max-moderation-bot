package filters

import (
	"context"
	"max-moderation-bot/internal/messages"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"testing"
)

func TestWordFilter_Process(t *testing.T) {
	mockRepo := &mockSettingsRepo{
		settings: &repository.ChatSettings{
			BlockedWords:     []string{"bad", "spam"},
			EnableWordFilter: true,
		},
	}
	mockViolation := &mockViolationRepo{}
	f := NewWordFilter(mockRepo, mockViolation)
	tests := []struct {
		name        string
		message     string
		wantAllowed bool
		wantReason  string
	}{
		{
			name:        "Clean message",
			message:     "Hello world",
			wantAllowed: true,
		},
		{
			name:        "Exact match",
			message:     "bad",
			wantAllowed: false,
			wantReason:  "bad",
		},
		{
			name:        "blocked word case insensitive",
			message:     "Some BAD word",
			wantAllowed: false,
			wantReason:  "bad",
		},
		{
			name:        "Contains word",
			message:     "This is very bad",
			wantAllowed: false,
			wantReason:  "bad",
		},
		{
			name:        "Partial match",
			message:     "badword",
			wantAllowed: false,
			wantReason:  "bad",
		},
		{
			name:        "Safe partial",
			message:     "notbadword",
			wantAllowed: false,
		},
		{
			name:        "Safe word",
			message:     "good",
			wantAllowed: true,
		},
		{
			name:        "Long sentence with bad word",
			message:     "This is a long sentence that contains a bad word in it",
			wantAllowed: false,
			wantReason:  "bad",
		},
		{
			name:        "Long sentence without bad word",
			message:     "This is a long sentence that is completely clean and safe",
			wantAllowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := f.Process(context.Background(), pipeline.Payload{ChatID: 123, Text: tt.message})
			if err != nil {
				t.Errorf("Process() error = %v", err)
				return
			}
			if res.IsAllowed != tt.wantAllowed {
				t.Errorf("Process() allowed = %v, want %v", res.IsAllowed, tt.wantAllowed)
			}
			if !tt.wantAllowed && tt.wantReason != "" {
				if res.Reason != messages.MsgReasonProhibitedWord {
					t.Errorf("Process() reason = %q, want %q", res.Reason, messages.MsgReasonProhibitedWord)
				}
			}
		})
	}
}
