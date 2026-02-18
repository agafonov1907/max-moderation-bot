package service

import (
	"context"
	"max-moderation-bot/internal/repository"
	"time"
)

type MockSettingsRepository struct {
	GetSettingsFunc    func(chatID int64) (*repository.ChatSettings, error)
	InitSettingsFunc   func(chatID int64) error
	UpdateSettingsFunc func(settings *repository.ChatSettings) error
}

func (m *MockSettingsRepository) GetSettings(chatID int64) (*repository.ChatSettings, error) {
	if m.GetSettingsFunc != nil {
		return m.GetSettingsFunc(chatID)
	}
	return &repository.ChatSettings{}, nil
}

func (m *MockSettingsRepository) InitSettings(chatID int64) error {
	if m.InitSettingsFunc != nil {
		return m.InitSettingsFunc(chatID)
	}
	return nil
}

func (m *MockSettingsRepository) UpdateSettings(settings *repository.ChatSettings) error {
	if m.UpdateSettingsFunc != nil {
		return m.UpdateSettingsFunc(settings)
	}
	return nil
}

type MockViolationRepository struct {
	AddViolationFunc         func(ctx context.Context, chatID, userID int64, violationType string) error
	CountViolationsSinceFunc func(ctx context.Context, chatID, userID int64, since time.Time) (int, error)
	IncrementChatStatFunc    func(ctx context.Context, chatID int64, field string) error
	GetChatTotalStatsFunc    func(ctx context.Context, chatID int64) (*repository.ChatStats, error)
}

func (m *MockViolationRepository) AddViolation(ctx context.Context, chatID, userID int64, violationType string) error {
	return m.AddViolationFunc(ctx, chatID, userID, violationType)
}
func (m *MockViolationRepository) CountViolationsSince(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
	return m.CountViolationsSinceFunc(ctx, chatID, userID, since)
}
func (m *MockViolationRepository) IncrementChatStat(ctx context.Context, chatID int64, field string) error {
	if m.IncrementChatStatFunc != nil {
		return m.IncrementChatStatFunc(ctx, chatID, field)
	}
	return nil
}
func (m *MockViolationRepository) GetChatTotalStats(ctx context.Context, chatID int64) (*repository.ChatStats, error) {
	if m.GetChatTotalStatsFunc != nil {
		return m.GetChatTotalStatsFunc(ctx, chatID)
	}
	return &repository.ChatStats{ChatID: chatID}, nil
}

type MockLinkTokenRepository struct {
	CreateFunc func(userID int64, ttl time.Duration) (string, error)
	GetFunc    func(token string) (*repository.LinkToken, error)
	DeleteFunc func(token string) error
}

func (m *MockLinkTokenRepository) Create(userID int64, ttl time.Duration) (string, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(userID, ttl)
	}
	return "mock-token", nil
}

func (m *MockLinkTokenRepository) Get(token string) (*repository.LinkToken, error) {
	if m.GetFunc != nil {
		return m.GetFunc(token)
	}
	return &repository.LinkToken{Token: token, UserID: 1}, nil
}

func (m *MockLinkTokenRepository) Delete(token string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(token)
	}
	return nil
}

func (m *MockLinkTokenRepository) DeleteExpired() error {
	return nil
}

type MockChatAdminRepository struct {
	IsAdminFunc                  func(chatID, userID int64) (bool, error)
	AddAdminFunc                 func(chatID, userID int64) error
	RemoveAdminFunc              func(chatID, userID int64) error
	GetManagedChatsFunc          func(userID int64) ([]int64, error)
	GetManagedChatsPaginatedFunc func(userID int64, offset, limit int) ([]int64, int64, error)
}

func (m *MockChatAdminRepository) IsAdmin(chatID, userID int64) (bool, error) {
	if m.IsAdminFunc != nil {
		return m.IsAdminFunc(chatID, userID)
	}
	return false, nil
}

func (m *MockChatAdminRepository) AddAdmin(chatID, userID int64) error {
	if m.AddAdminFunc != nil {
		return m.AddAdminFunc(chatID, userID)
	}
	return nil
}

func (m *MockChatAdminRepository) RemoveAdmin(chatID, userID int64) error {
	if m.RemoveAdminFunc != nil {
		return m.RemoveAdminFunc(chatID, userID)
	}
	return nil
}

func (m *MockChatAdminRepository) GetManagedChats(userID int64) ([]int64, error) {
	if m.GetManagedChatsFunc != nil {
		return m.GetManagedChatsFunc(userID)
	}
	return nil, nil
}

func (m *MockChatAdminRepository) GetManagedChatsPaginated(userID int64, offset, limit int) ([]int64, int64, error) {
	if m.GetManagedChatsPaginatedFunc != nil {
		return m.GetManagedChatsPaginatedFunc(userID, offset, limit)
	}
	return nil, 0, nil
}

type MockMuteRepository struct {
	MuteUserFunc                func(chatID, userID int64, userName string, duration time.Duration) error
	UnmuteUserFunc              func(chatID, userID int64) error
	IsMutedFunc                 func(chatID, userID int64) (bool, time.Time, error)
	GetActiveMutesPaginatedFunc func(chatID int64, offset, limit int) ([]repository.Mute, int64, error)
	GetMuteFunc                 func(chatID, userID int64) (*repository.Mute, error)
	GetActiveMutesFunc          func(chatID int64) ([]repository.Mute, error)
	CountActiveMutesFunc        func() (int64, error)
}

func (m *MockMuteRepository) MuteUser(chatID, userID int64, userName string, duration time.Duration) error {
	if m.MuteUserFunc != nil {
		return m.MuteUserFunc(chatID, userID, userName, duration)
	}
	return nil
}
func (m *MockMuteRepository) UnmuteUser(chatID, userID int64) error {
	if m.UnmuteUserFunc != nil {
		return m.UnmuteUserFunc(chatID, userID)
	}
	return nil
}
func (m *MockMuteRepository) IsMuted(chatID, userID int64) (bool, time.Time, error) {
	if m.IsMutedFunc != nil {
		return m.IsMutedFunc(chatID, userID)
	}
	return false, time.Time{}, nil
}
func (m *MockMuteRepository) GetMute(chatID, userID int64) (*repository.Mute, error) {
	if m.GetMuteFunc != nil {
		return m.GetMuteFunc(chatID, userID)
	}
	return nil, nil
}
func (m *MockMuteRepository) GetActiveMutesPaginated(chatID int64, offset, limit int) ([]repository.Mute, int64, error) {
	if m.GetActiveMutesPaginatedFunc != nil {
		return m.GetActiveMutesPaginatedFunc(chatID, offset, limit)
	}
	return nil, 0, nil
}
func (m *MockMuteRepository) GetActiveMutes(chatID int64) ([]repository.Mute, error) {
	if m.GetActiveMutesFunc != nil {
		return m.GetActiveMutesFunc(chatID)
	}
	return nil, nil
}
func (m *MockMuteRepository) CountActiveMutes() (int64, error) {
	if m.CountActiveMutesFunc != nil {
		return m.CountActiveMutesFunc()
	}
	return 0, nil
}
