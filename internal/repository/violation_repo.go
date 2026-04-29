package repository

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log/slog"
	"time"
)

type ViolationRepository interface {
	AddViolation(ctx context.Context, chatID, userID int64, violationType string) error
	CountViolationsSince(ctx context.Context, chatID, userID int64, since time.Time) (int, error)
	IncrementChatStat(ctx context.Context, chatID int64, field string) error
	GetChatTotalStats(ctx context.Context, chatID int64) (*ChatStats, error)
}

type PostgresViolationRepository struct {
	db *gorm.DB
}

type UserViolation struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	ChatID        int64     `gorm:"not null;index:idx_user_violations_chat_user_created,priority:1" json:"chat_id"`
	UserID        int64     `gorm:"not null;index:idx_user_violations_chat_user_created,priority:2" json:"user_id"`
	ViolationType string    `gorm:"size:50" json:"violation_type"`
	CreatedAt     time.Time `gorm:"not null;default:now();index:idx_user_violations_chat_user_created,priority:3" json:"created_at"`
}

func NewViolationRepository(db *gorm.DB) ViolationRepository {
	return &PostgresViolationRepository{db: db}
}

func (r *PostgresViolationRepository) AddViolation(ctx context.Context, chatID, userID int64, violationType string) error {
	violation := UserViolation{
		ChatID:        chatID,
		UserID:        userID,
		ViolationType: violationType,
		CreatedAt:     time.Now(),
	}
	return r.db.WithContext(ctx).Create(&violation).Error
}

func (r *PostgresViolationRepository) CountViolationsSince(ctx context.Context, chatID, userID int64, since time.Time) (int, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserViolation{}).
		Where("chat_id = ? AND user_id = ? AND created_at >= ?", chatID, userID, since).
		Count(&count).Error
	return int(count), err
}

func (r *PostgresViolationRepository) IncrementChatStat(ctx context.Context, chatID int64, field string) error {
	slog.Debug("Incrementing chat stat", "chat_id", chatID, "field", field)
	now := time.Now().Truncate(24 * time.Hour)

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chat_id"}, {Name: "date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			field: clause.Expr{SQL: "chat_stats." + field + " + 1"},
		}),
	}).Create(&ChatStats{
		ChatID: chatID,
		Date:   now,
	}).Error
}

func (r *PostgresViolationRepository) GetChatTotalStats(ctx context.Context, chatID int64) (*ChatStats, error) {
	var stats ChatStats
	err := r.db.WithContext(ctx).Model(&ChatStats{}).
		Select("chat_id, SUM(word_violations) as word_violations, SUM(link_violations) as link_violations, SUM(image_violations) as image_violations, SUM(video_violations) as video_violations, SUM(audio_violations) as audio_violations, SUM(file_violations) as file_violations, SUM(mute_count) as mute_count").
		Where("chat_id = ?", chatID).
		Group("chat_id").
		First(&stats).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &ChatStats{ChatID: chatID}, nil
		}
		return nil, err
	}
	return &stats, nil
}
