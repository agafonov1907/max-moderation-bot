package filters

import (
	"context"
	"max-moderation-bot/internal/pipeline"
	"max-moderation-bot/internal/repository"
	"testing"
)

func TestLinkFilter_Process(t *testing.T) {
	tests := []struct {
		name        string
		settings    *repository.ChatSettings
		message     string
		wantAllowed bool
	}{
		{
			name: "Filter disabled",
			settings: &repository.ChatSettings{
				EnableLinkFilter: false,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "http://bad.com",
			wantAllowed: true,
		},
		{
			name: "No links",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "hello world",
			wantAllowed: true,
		},
		{
			name: "Allowed link",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "https://good.com",
			wantAllowed: true,
		},
		{
			name: "Blocked domain exact",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "https://bad.com",
			wantAllowed: false,
		},
		{
			name: "Blocked domain partial",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "https://sub.bad.com/page",
			wantAllowed: false,
		},
		{
			name: "Blocked domain in text",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "Check this out: https://bad.com now",
			wantAllowed: false,
		},
		{
			name: "blocked domain case insensitive without scheme",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"spam.com"},
			},
			message:     "Check out SPAM.COM now",
			wantAllowed: false,
		},
		{
			name: "Case insensitive",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "HTTPS://BAD.COM",
			wantAllowed: false,
		},
		{
			name: "Blocked domain with trailing slash in DB",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com/"},
			},
			message:     "https://example.com",
			wantAllowed: false,
		},
		{
			name: "Blocked domain with https prefix in DB",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"https://example.com"},
			},
			message:     "http://example.com",
			wantAllowed: false,
		},
		{
			name: "Blocked subpage with trailing slash",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com"},
			},
			message:     "https://example.com/news/",
			wantAllowed: false,
		},
		{
			name: "Mixed case in DB",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"Example.com"},
			},
			message:     "http://example.com",
			wantAllowed: false,
		},
		{
			name: "Link in middle of text",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com"},
			},
			message:     "Check this link example.com inside text",
			wantAllowed: false,
		},
		{
			name: "Link at start of text",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com"},
			},
			message:     "example.com is the site",
			wantAllowed: false,
		},
		{
			name: "Link at end of text",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com"},
			},
			message:     "Go to example.com",
			wantAllowed: false,
		},
		{
			name: "Link with punctuation",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"example.com"},
			},
			message:     "Is this (example.com) blocked?",
			wantAllowed: false,
		},
		{
			name: "Multiple links, one blocked",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"bad.com"},
			},
			message:     "good.com is safe but bad.com is not",
			wantAllowed: false,
		},
		{
			name: "Link with cyrillic domain",
			settings: &repository.ChatSettings{
				EnableLinkFilter: true,
				BlockedDomains:   []string{"пример.рф"},
			},
			message:     "Заходи на пример.рф сейчас",
			wantAllowed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &mockSettingsRepo{settings: tt.settings}
			mockViolation := &mockViolationRepo{}
			f := NewLinkFilter(mockRepo, mockViolation)
			res, err := f.Process(context.Background(), pipeline.Payload{ChatID: 123, Text: tt.message})
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if res.IsAllowed != tt.wantAllowed {
				t.Errorf("Process() allowed = %v, want %v", res.IsAllowed, tt.wantAllowed)
			}
			if !tt.wantAllowed && res.FilterName != "link_filter" {
				t.Errorf("Process() filter = %v, want link_filter", res.FilterName)
			}
		})
	}
}
