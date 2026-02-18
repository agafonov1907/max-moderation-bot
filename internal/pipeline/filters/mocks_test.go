package filters

import (
	"context"
	"max-moderation-bot/internal/repository"
	"time"
)

type mockSettingsRepo struct {
	settings         *repository.ChatSettings
	err              error
	InitSettingsFunc func(_ int64) error
	GetSettingsFunc  func(_ int64) (*repository.ChatSettings, error)
}

func (m *mockSettingsRepo) InitSettings(chatID int64) error {
	if m.InitSettingsFunc != nil {
		return m.InitSettingsFunc(chatID)
	}
	return m.err
}
func (m *mockSettingsRepo) UpdateSettings(settings *repository.ChatSettings) error {
	m.settings = settings
	return m.err
}
func (m *mockSettingsRepo) GetSettings(chatID int64) (*repository.ChatSettings, error) {
	if m.GetSettingsFunc != nil {
		return m.GetSettingsFunc(chatID)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.settings, nil
}

type mockMuteRepo struct {
	isMuted                     bool
	expiresAt                   time.Time
	err                         error
	muteErr                     error
	activeMutes                 int64
	IsMutedFunc                 func(_, _ int64) (bool, time.Time, error)
	MuteUserFunc                func(_, _ int64, _ string, _ time.Duration) error
	UnmuteUserFunc              func(_, _ int64) error
	GetActiveMutesFunc          func(_ int64) ([]repository.Mute, error)
	GetActiveMutesPaginatedFunc func(_ int64, _, _ int) ([]repository.Mute, int64, error)
	GetMuteFunc                 func(_, _ int64) (*repository.Mute, error)
}

func (m *mockMuteRepo) IsMuted(chatID, userID int64) (bool, time.Time, error) {
	if m.IsMutedFunc != nil {
		return m.IsMutedFunc(chatID, userID)
	}
	return m.isMuted, m.expiresAt, m.err
}
func (m *mockMuteRepo) MuteUser(chatID, userID int64, userName string, duration time.Duration) error {
	if m.MuteUserFunc != nil {
		return m.MuteUserFunc(chatID, userID, userName, duration)
	}
	return m.muteErr
}
func (m *mockMuteRepo) UnmuteUser(chatID, userID int64) error {
	if m.UnmuteUserFunc != nil {
		return m.UnmuteUserFunc(chatID, userID)
	}
	return nil
}
func (m *mockMuteRepo) GetMute(chatID, userID int64) (*repository.Mute, error) {
	if m.GetMuteFunc != nil {
		return m.GetMuteFunc(chatID, userID)
	}
	return nil, nil
}
func (m *mockMuteRepo) GetActiveMutes(chatID int64) ([]repository.Mute, error) {
	if m.GetActiveMutesFunc != nil {
		return m.GetActiveMutesFunc(chatID)
	}
	return nil, nil
}
func (m *mockMuteRepo) GetActiveMutesPaginated(chatID int64, offset, limit int) ([]repository.Mute, int64, error) {
	if m.GetActiveMutesPaginatedFunc != nil {
		return m.GetActiveMutesPaginatedFunc(chatID, offset, limit)
	}
	return nil, 0, nil
}
func (m *mockMuteRepo) CountActiveMutes() (int64, error) {
	return m.activeMutes, m.err
}

type mockViolationRepo struct {
	IncrementChatStatFunc func(ctx context.Context, chatID int64, field string) error
	GetChatTotalStatsFunc func(ctx context.Context, chatID int64) (*repository.ChatStats, error)
	AddViolationFunc      func(ctx context.Context, chatID, userID int64, violationType string) error
	CountViolationsFunc   func(ctx context.Context, chatID, userID int64, since time.Time) (int, error)
}

func (m *mockViolationRepo) IncrementChatStat(ctx context.Context, chatID int64, field string) error {
	if m.IncrementChatStatFunc != nil {
		return m.IncrementChatStatFunc(ctx, chatID, field)
	}
	return nil
}

func (m *mockViolationRepo) GetChatTotalStats(ctx context.Context, chatID int64) (*repository.ChatStats, error) {
	if m.GetChatTotalStatsFunc != nil {
		return m.GetChatTotalStatsFunc(ctx, chatID)
	}
	return &repository.ChatStats{ChatID: chatID}, nil
}

func (m *mockViolationRepo) AddViolation(ctx context.Context, chatID, userID int64, violationType string) error {
	if m.AddViolationFunc != nil {
		return m.AddViolationFunc(ctx, chatID, userID, violationType)
	}
	return nil
}

func (m *mockViolationRepo) CountViolationsSince(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
	if m.CountViolationsFunc != nil {
		return m.CountViolationsFunc(ctx, chatID, userID, since)
	}
	return 0, nil
}
