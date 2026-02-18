package repository

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UserStateRepository interface {
	SetState(userID, chatID int64, action string) error
	GetState(userID int64) (*UserState, error)
	ClearState(userID int64) error
}
type PostgresUserStateRepository struct {
	db *gorm.DB
}

func NewUserStateRepository(db *gorm.DB) UserStateRepository {
	return &PostgresUserStateRepository{db: db}
}
func (r *PostgresUserStateRepository) SetState(userID, chatID int64, action string) error {
	state := UserState{
		UserID:    userID,
		ChatID:    chatID,
		Action:    action,
		CreatedAt: time.Now(),
	}
	err := r.db.Save(&state).Error
	if err != nil {
		return fmt.Errorf("failed to set user state: %w", err)
	}
	return nil
}
func (r *PostgresUserStateRepository) GetState(userID int64) (*UserState, error) {
	var state UserState
	err := r.db.Where("user_id = ?", userID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user state: %w", err)
	}
	return &state, nil
}
func (r *PostgresUserStateRepository) ClearState(userID int64) error {
	if err := r.db.Where("user_id = ?", userID).Delete(&UserState{}).Error; err != nil {
		return fmt.Errorf("failed to clear user state: %w", err)
	}
	return nil
}
