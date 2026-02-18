package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LinkTokenRepository interface {
	Create(userID int64, ttl time.Duration) (string, error)
	Get(token string) (*LinkToken, error)
	Delete(token string) error
	DeleteExpired() error
}
type PostgresLinkTokenRepository struct {
	db *gorm.DB
}

func NewLinkTokenRepository(db *gorm.DB) LinkTokenRepository {
	return &PostgresLinkTokenRepository{db: db}
}
func (r *PostgresLinkTokenRepository) Create(userID int64, ttl time.Duration) (string, error) {
	token := uuid.New().String()
	linkToken := LinkToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(ttl),
	}
	if err := r.db.Create(&linkToken).Error; err != nil {
		return "", fmt.Errorf("failed to create link token: %w", err)
	}
	return token, nil
}
func (r *PostgresLinkTokenRepository) Get(token string) (*LinkToken, error) {
	var linkToken LinkToken
	if err := r.db.First(&linkToken, "token = ?", token).Error; err != nil {
		return nil, fmt.Errorf("failed to get link token: %w", err)
	}
	if time.Now().After(linkToken.ExpiresAt) {
		if err := r.Delete(token); err != nil {
			return nil, errors.Join(gorm.ErrRecordNotFound, err)
		}
		return nil, gorm.ErrRecordNotFound
	}
	return &linkToken, nil
}
func (r *PostgresLinkTokenRepository) Delete(token string) error {
	if err := r.db.Delete(&LinkToken{}, "token = ?", token).Error; err != nil {
		return fmt.Errorf("failed to delete link token: %w", err)
	}
	return nil
}
func (r *PostgresLinkTokenRepository) DeleteExpired() error {
	if err := r.db.Where("expires_at < ?", time.Now()).Delete(&LinkToken{}).Error; err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}
	return nil
}
