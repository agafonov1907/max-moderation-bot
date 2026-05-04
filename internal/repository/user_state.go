package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UserStateRepository interface {
	SetState(userID, chatID int64, action string) error
	SetStateWithMetadata(userID, chatID int64, action, metadata string) error
	GetState(userID int64) (*UserState, error)
	ClearState(userID int64) error
}

type PostgresUserStateRepository struct {
	db *gorm.DB
}

func NewUserStateRepository(db *gorm.DB) UserStateRepository {
	return &PostgresUserStateRepository{db: db}
}

// SetState устанавливает состояние пользователя (без метаданных)
func (r *PostgresUserStateRepository) SetState(userID, chatID int64, action string) error {
	now := time.Now()
	state := UserState{
		UserID:    userID,
		ChatID:    chatID,
		Action:    action,
		Metadata:  "",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// ✅ Save делает INSERT если записи нет, или UPDATE если есть (upsert)
	// Явно указываем таблицу, чтобы избежать проблем с plural/singular
	err := r.db.Table("user_state").Save(&state).Error
	if err != nil {
		return fmt.Errorf("failed to set user state: %w", err)
	}
	return nil
}

// SetStateWithMetadata устанавливает состояние пользователя с метаданными
func (r *PostgresUserStateRepository) SetStateWithMetadata(userID, chatID int64, action, metadata string) error {
	now := time.Now()
	state := UserState{
		UserID:    userID,
		ChatID:    chatID,
		Action:    action,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := r.db.Table("user_state").Save(&state).Error
	if err != nil {
		return fmt.Errorf("failed to set user state with metadata: %w", err)
	}
	return nil
}

// GetState получает текущее состояние пользователя
func (r *PostgresUserStateRepository) GetState(userID int64) (*UserState, error) {
	var state UserState
	err := r.db.Table("user_state").Where("user_id = ?", userID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}
	return &state, nil
}

// ClearState удаляет состояние пользователя
func (r *PostgresUserStateRepository) ClearState(userID int64) error {
	err := r.db.Table("user_state").Where("user_id = ?", userID).Delete(&UserState{}).Error
	if err != nil {
		return fmt.Errorf("failed to clear user state: %w", err)
	}
	return nil
}
