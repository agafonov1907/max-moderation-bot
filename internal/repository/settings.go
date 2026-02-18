package repository

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

type SettingsRepository interface {
	GetSettings(chatID int64) (*ChatSettings, error)
	InitSettings(chatID int64) error
	UpdateSettings(settings *ChatSettings) error
}

type CachedSettingsRepository struct {
	db          *gorm.DB
	cache       sync.Map
	enableCache bool
}

type cachedSettings struct {
	settings  *ChatSettings
	expiresAt time.Time
}

const cacheTTL = 5 * time.Minute

func NewSettingsRepository(db *gorm.DB, enableCache bool) SettingsRepository {
	return &CachedSettingsRepository{
		db:          db,
		enableCache: enableCache,
	}
}

func (r *CachedSettingsRepository) GetSettings(chatID int64) (*ChatSettings, error) {
	if r.enableCache {
		if val, ok := r.cache.Load(chatID); ok {
			entry := val.(*cachedSettings)
			if time.Now().Before(entry.expiresAt) {
				return entry.settings, nil
			}
			r.cache.Delete(chatID)
		}
	}
	var settings ChatSettings
	err := r.db.First(&settings, "chat_id = ?", chatID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if initErr := r.InitSettings(chatID); initErr != nil {
				return nil, fmt.Errorf("failed to init settings on miss: %w", initErr)
			}
			return &ChatSettings{
				ChatID:           chatID,
				EnableWordFilter: true,
				EnableLinkFilter: true,
				EnableMute:       true,
				EnableAutoDelete: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}
	if r.enableCache {
		r.cache.Store(chatID, &cachedSettings{
			settings:  &settings,
			expiresAt: time.Now().Add(cacheTTL),
		})
	}
	return &settings, nil
}

func (r *CachedSettingsRepository) InitSettings(chatID int64) error {

	settings := ChatSettings{
		ChatID:           chatID,
		EnableWordFilter: true,
		EnableLinkFilter: true,
		EnableMute:       true,
		EnableAutoDelete: true,
		RestrictImage:    false,
		RestrictVideo:    false,
		RestrictAudio:    false,
	}
	if err := r.db.FirstOrCreate(&settings, ChatSettings{ChatID: chatID}).Error; err != nil {
		return fmt.Errorf("failed to init settings: %w", err)
	}
	return nil
}

func (r *CachedSettingsRepository) UpdateSettings(settings *ChatSettings) error {
	if err := r.db.Save(settings).Error; err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	if r.enableCache {
		r.cache.Store(settings.ChatID, &cachedSettings{
			settings:  settings,
			expiresAt: time.Now().Add(cacheTTL),
		})
	}
	return nil
}
