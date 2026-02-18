package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type MuteRepository interface {
	MuteUser(chatID, userID int64, userName string, duration time.Duration) error
	UnmuteUser(chatID, userID int64) error
	IsMuted(chatID, userID int64) (bool, time.Time, error)
	GetActiveMutesPaginated(chatID int64, offset, limit int) ([]Mute, int64, error)
	GetMute(chatID, userID int64) (*Mute, error)
	GetActiveMutes(chatID int64) ([]Mute, error)
	CountActiveMutes() (int64, error)
}
type PostgresMuteRepository struct {
	db *gorm.DB
}

func NewMuteRepository(db *gorm.DB) MuteRepository {
	return &PostgresMuteRepository{db: db}
}
func (r *PostgresMuteRepository) MuteUser(chatID, userID int64, userName string, duration time.Duration) error {
	expiresAt := time.Now().Add(duration)
	var existing Mute
	err := r.db.Where("chat_id = ? AND user_id = ?", chatID, userID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			mute := Mute{
				ChatID:    chatID,
				UserID:    userID,
				UserName:  userName,
				ExpiresAt: expiresAt,
			}
			if err := r.db.Create(&mute).Error; err != nil {
				return fmt.Errorf("failed to create mute: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to check existing mute: %w", err)
	}

	updates := map[string]interface{}{}
	if expiresAt.After(existing.ExpiresAt) {
		updates["expires_at"] = expiresAt
	}
	if userName != "" && userName != existing.UserName {
		updates["user_name"] = userName
	}

	if len(updates) > 0 {
		if err := r.db.Model(&existing).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to update mute: %w", err)
		}
	}
	return nil
}
func (r *PostgresMuteRepository) UnmuteUser(chatID, userID int64) error {
	if err := r.db.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&Mute{}).Error; err != nil {
		return fmt.Errorf("failed to unmute user: %w", err)
	}
	return nil
}
func (r *PostgresMuteRepository) IsMuted(chatID, userID int64) (bool, time.Time, error) {
	var mute Mute
	err := r.db.Where("chat_id = ? AND user_id = ?", chatID, userID).
		Where("expires_at > ?", time.Now()).
		First(&mute).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, time.Time{}, nil
		}
		return false, time.Time{}, fmt.Errorf("failed to check mute status: %w", err)
	}
	return true, mute.ExpiresAt, nil
}
func (r *PostgresMuteRepository) GetMute(chatID, userID int64) (*Mute, error) {
	var mute Mute
	if err := r.db.Where("chat_id = ? AND user_id = ?", chatID, userID).First(&mute).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get mute: %w", err)
	}
	return &mute, nil
}

func (r *PostgresMuteRepository) GetActiveMutes(chatID int64) ([]Mute, error) {
	var mutes []Mute
	if err := r.db.Where("chat_id = ? AND expires_at > ?", chatID, time.Now()).Find(&mutes).Error; err != nil {
		return nil, fmt.Errorf("failed to get active mutes: %w", err)
	}
	return mutes, nil
}
func (r *PostgresMuteRepository) GetActiveMutesPaginated(chatID int64, offset, limit int) ([]Mute, int64, error) {
	var mutes []Mute
	var total int64
	query := r.db.Model(&Mute{}).Where("chat_id = ? AND expires_at > ?", chatID, time.Now())
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count active mutes: %w", err)
	}
	if err := query.Offset(offset).Limit(limit).Order("expires_at ASC").Find(&mutes).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get active mutes: %w", err)
	}
	return mutes, total, nil
}
func (r *PostgresMuteRepository) CountActiveMutes() (int64, error) {
	var count int64
	if err := r.db.Model(&Mute{}).Where("expires_at > ?", time.Now()).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count active mutes: %w", err)
	}
	return count, nil
}
