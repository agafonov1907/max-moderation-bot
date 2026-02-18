package repository

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	ID            int64     `gorm:"primaryKey"`
	ChatID        int64     `gorm:"not null;index:idx_user_violations_chat_user_created,priority:1"`
	UserID        int64     `gorm:"not null;index:idx_user_violations_chat_user_created,priority:2"`
	ViolationType string    `gorm:"size:50"`
	CreatedAt     time.Time `gorm:"not null;default:now();index:idx_user_violations_chat_user_created,priority:3"`
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
		WordViolations: func() int64 {
			if field == "word_violations" {
				return 1
			}
			return 0
		}(),
		LinkViolations: func() int64 {
			if field == "link_violations" {
				return 1
			}
			return 0
		}(),
		ImageViolations: func() int64 {
			if field == "image_violations" {
				return 1
			}
			return 0
		}(),
		VideoViolations: func() int64 {
			if field == "video_violations" {
				return 1
			}
			return 0
		}(),
		AudioViolations: func() int64 {
			if field == "audio_violations" {
				return 1
			}
			return 0
		}(),
		FileViolations: func() int64 {
			if field == "file_violations" {
				return 1
			}
			return 0
		}(),
		MuteCount: func() int64 {
			if field == "mute_count" {
				return 1
			}
			return 0
		}(),
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
