package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UserStateRepository interface {
	SetState(userID, chatID int64, action string) error
	SetStateWithMetadata(userID, chatID int64, action, metadata string) error // ✅ Новый метод
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
		Metadata:  "", // Пустая метаданные
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := r.db.Save(&state).Error
	if err != nil {
		return fmt.Errorf("failed to set user state: %w", err)
	}
	return nil
}

// ✅ SetStateWithMetadata устанавливает состояние пользователя с метаданными
func (r *PostgresUserStateRepository) SetStateWithMetadata(userID, chatID int64, action, metadata string) error {
	now := time.Now()
	state := UserState{
		UserID:    userID,
		ChatID:    chatID,
		Action:    action,
		Metadata:  metadata, // ✅ Сохраняем метаданные (например, "123,456,789")
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := r.db.Save(&state).Error
	if err != nil {
		return fmt.Errorf("failed to set user state with metadata: %w", err)
	}
	return nil
}

// GetState получает текущее состояние пользователя
func (r *PostgresUserStateRepository) GetState(userID int64) (*UserState, error) {
	var state UserState
	err := r.db.Where("user_id = ?", userID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Нет состояния — это нормально
		}
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}
	return &state, nil
}

// ClearState удаляет состояние пользователя
func (r *PostgresUserStateRepository) ClearState(userID int64) error {
	if err := r.db.Where("user_id = ?", userID).Delete(&UserState{}).Error; err != nil {
		return fmt.Errorf("failed to clear user state: %w", err)
	}
	return nil
}