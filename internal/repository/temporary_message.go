package repository

import (
	"time"

	"gorm.io/gorm"
)

type TemporaryMessage struct {
	ID        int64     `gorm:"primaryKey"`
	ChatID    int64     `gorm:"not null"`
	MessageID string    `gorm:"not null"`
	DeleteAt  time.Time `gorm:"not null;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type TemporaryMessageRepository interface {
	Add(chatID int64, messageID string, duration time.Duration) error
	GetExpired(limit int) ([]TemporaryMessage, error)
	Delete(ids []int64) error
}

type PostgresTemporaryMessageRepository struct {
	db *gorm.DB
}

func NewTemporaryMessageRepository(db *gorm.DB) *PostgresTemporaryMessageRepository {
	return &PostgresTemporaryMessageRepository{db: db}
}

func (r *PostgresTemporaryMessageRepository) Add(chatID int64, messageID string, duration time.Duration) error {
	msg := TemporaryMessage{
		ChatID:    chatID,
		MessageID: messageID,
		DeleteAt:  time.Now().Add(duration),
	}
	return r.db.Create(&msg).Error
}

func (r *PostgresTemporaryMessageRepository) GetExpired(limit int) ([]TemporaryMessage, error) {
	var messages []TemporaryMessage
	err := r.db.Where("delete_at <= ?", time.Now()).Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *PostgresTemporaryMessageRepository) Delete(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Delete(&TemporaryMessage{}, ids).Error
}
