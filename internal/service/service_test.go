package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"max-moderation-bot/internal/repository"
)

func TestModerationService_ToggleSetting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		chatID        int64
		setting       string
		setupMock     func() *MockSettingsRepository
		wantNewValue  bool
		wantErr       bool
		wantErrString string
	}{
		{
			name:    "Success - toggle words",
			chatID:  123,
			setting: "words",
			setupMock: func() *MockSettingsRepository {
				settings := &repository.ChatSettings{ChatID: 123, EnableWordFilter: false}
				return &MockSettingsRepository{
					GetSettingsFunc: func(chatID int64) (*repository.ChatSettings, error) {
						return settings, nil
					},
					UpdateSettingsFunc: func(s *repository.ChatSettings) error {
						if s.EnableWordFilter != true {
							t.Errorf("expected EnableWordFilter to be true")
						}
						return nil
					},
				}
			},
			wantNewValue: true,
			wantErr:      false,
		},
		{
			name:    "Unknown setting",
			chatID:  123,
			setting: "unknown_setting",
			setupMock: func() *MockSettingsRepository {
				return &MockSettingsRepository{
					GetSettingsFunc: func(chatID int64) (*repository.ChatSettings, error) {
						return &repository.ChatSettings{}, nil
					},
				}
			},
			wantNewValue:  false,
			wantErr:       true,
			wantErrString: "unknown setting: unknown_setting",
		},
		{
			name:    "Repo Error on Get",
			chatID:  123,
			setting: "words",
			setupMock: func() *MockSettingsRepository {
				return &MockSettingsRepository{
					GetSettingsFunc: func(chatID int64) (*repository.ChatSettings, error) {
						return nil, errors.New("db error")
					},
				}
			},
			wantNewValue: false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSettings := tt.setupMock()
			svc := NewModerationService(logger, mockSettings, nil, nil, nil, nil, &MockViolationRepository{}, nil)

			got, err := svc.ToggleSetting(context.Background(), tt.chatID, tt.setting)

			if (err != nil) != tt.wantErr {
				t.Errorf("ToggleSetting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrString != "" && err.Error() != tt.wantErrString {
				t.Errorf("ToggleSetting() error = %v, wantErrString %v", err, tt.wantErrString)
			}
			if got != tt.wantNewValue {
				t.Errorf("ToggleSetting() = %v, want %v", got, tt.wantNewValue)
			}
		})
	}
}

func TestModerationService_LinkGroup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		token         string
		chatID        int64
		userID        int64
		setupMocks    func() (*MockLinkTokenRepository, *MockChatAdminRepository)
		wantErr       bool
		wantErrString string
	}{
		{
			name:   "Success",
			token:  "valid-token",
			chatID: 100,
			userID: 1,
			setupMocks: func() (*MockLinkTokenRepository, *MockChatAdminRepository) {
				return &MockLinkTokenRepository{
						GetFunc: func(token string) (*repository.LinkToken, error) {
							return &repository.LinkToken{Token: token, UserID: 1}, nil
						},
						DeleteFunc: func(token string) error {
							return nil
						},
					}, &MockChatAdminRepository{
						AddAdminFunc: func(chatID, userID int64) error {
							return nil
						},
					}
			},
			wantErr: false,
		},
		{
			name:   "Invalid Token",
			token:  "invalid-token",
			chatID: 100,
			userID: 1,
			setupMocks: func() (*MockLinkTokenRepository, *MockChatAdminRepository) {
				return &MockLinkTokenRepository{
					GetFunc: func(token string) (*repository.LinkToken, error) {
						return nil, errors.New("not found")
					},
				}, &MockChatAdminRepository{}
			},
			wantErr:       true,
			wantErrString: "invalid or expired token: not found",
		},
		{
			name:   "Wrong User",
			token:  "valid-token",
			chatID: 100,
			userID: 2,
			setupMocks: func() (*MockLinkTokenRepository, *MockChatAdminRepository) {
				return &MockLinkTokenRepository{
					GetFunc: func(token string) (*repository.LinkToken, error) {
						return &repository.LinkToken{Token: token, UserID: 1}, nil
					},
				}, &MockChatAdminRepository{}
			},
			wantErr:       true,
			wantErrString: "token does not belong to user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linkRepo, adminRepo := tt.setupMocks()
			svc := NewModerationService(logger, nil, adminRepo, linkRepo, nil, nil, &MockViolationRepository{}, nil)

			err := svc.LinkGroup(context.Background(), tt.token, tt.chatID, tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("LinkGroup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrString != "" && err.Error() != tt.wantErrString {
				t.Errorf("LinkGroup() error = %v, wantErrString %v", err, tt.wantErrString)
			}
		})
	}
}

func TestModerationService_AddBlockedWords(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	tests := []struct {
		name      string
		chatID    int64
		newWords  []string
		setupMock func() *MockSettingsRepository
		wantErr   bool
	}{
		{
			name:     "Add unique words",
			chatID:   123,
			newWords: []string{"bad", "evil"},
			setupMock: func() *MockSettingsRepository {
				settings := &repository.ChatSettings{
					ChatID:       123,
					BlockedWords: []string{"existing"},
				}
				return &MockSettingsRepository{
					GetSettingsFunc: func(chatID int64) (*repository.ChatSettings, error) {
						return settings, nil
					},
					UpdateSettingsFunc: func(s *repository.ChatSettings) error {
						if len(s.BlockedWords) != 3 {
							t.Errorf("expected 3 words, got %d", len(s.BlockedWords))
						}
						return nil
					},
				}
			},
			wantErr: false,
		},
		{
			name:     "Ignore duplicates",
			chatID:   123,
			newWords: []string{"existing", "new"},
			setupMock: func() *MockSettingsRepository {
				settings := &repository.ChatSettings{
					ChatID:       123,
					BlockedWords: []string{"existing"},
				}
				return &MockSettingsRepository{
					GetSettingsFunc: func(chatID int64) (*repository.ChatSettings, error) {
						return settings, nil
					},
					UpdateSettingsFunc: func(s *repository.ChatSettings) error {
						if len(s.BlockedWords) != 2 {
							t.Errorf("expected 2 words, got %d", len(s.BlockedWords))
						}
						return nil
					},
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSettings := tt.setupMock()
			svc := NewModerationService(logger, mockSettings, nil, nil, nil, nil, &MockViolationRepository{}, nil)

			err := svc.AddBlockedWords(context.Background(), tt.chatID, tt.newWords)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddBlockedWords() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestModerationService_UnmuteUser(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name       string
		chatID     int64
		adminID    int64
		userID     int64
		setupMocks func() (*MockChatAdminRepository, *MockMuteRepository)
		wantErr    bool
	}{
		{
			name:    "Success",
			chatID:  100,
			adminID: 1,
			userID:  456,
			setupMocks: func() (*MockChatAdminRepository, *MockMuteRepository) {
				return &MockChatAdminRepository{
						IsAdminFunc: func(chatID, userID int64) (bool, error) {
							return true, nil
						},
					}, &MockMuteRepository{
						UnmuteUserFunc: func(chatID, userID int64) error {
							return nil
						},
					}
			},
			wantErr: false,
		},
		{
			name:    "Not Admin",
			chatID:  100,
			adminID: 2,
			userID:  456,
			setupMocks: func() (*MockChatAdminRepository, *MockMuteRepository) {
				return &MockChatAdminRepository{
					IsAdminFunc: func(chatID, userID int64) (bool, error) {
						return false, nil
					},
				}, &MockMuteRepository{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adminRepo, muteRepo := tt.setupMocks()
			svc := NewModerationService(logger, nil, adminRepo, nil, muteRepo, nil, &MockViolationRepository{}, nil)

			err := svc.UnmuteUser(context.Background(), tt.chatID, tt.adminID, tt.userID)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmuteUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModerationService_GetChatStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	chatID := int64(123)

	mockViolation := &MockViolationRepository{
		GetChatTotalStatsFunc: func(ctx context.Context, cID int64) (*repository.ChatStats, error) {
			if cID != chatID {
				t.Errorf("expected chatID %d, got %d", chatID, cID)
			}
			return &repository.ChatStats{
				ChatID:         chatID,
				WordViolations: 10,
				MuteCount:      5,
			}, nil
		},
	}

	svc := NewModerationService(logger, nil, nil, nil, nil, nil, mockViolation, nil)
	stats, err := svc.GetChatStats(context.Background(), chatID)

	if err != nil {
		t.Fatalf("GetChatStats() error = %v", err)
	}
	if stats.WordViolations != 10 {
		t.Errorf("expected 10 word violations, got %d", stats.WordViolations)
	}
	if stats.MuteCount != 5 {
		t.Errorf("expected 5 mutes, got %d", stats.MuteCount)
	}
}
