package repository

import (
	"context"
	"fmt"
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

// GetChatTotalStats возвращает суммарную статистику по чату (все дни)
func (r *PostgresViolationRepository) GetChatTotalStats(ctx context.Context, chatID int64) (*ChatStats, error) {
	// ✅ Анонимная структура ТОЛЬКО для агрегированных полей
	// Это предотвращает добавление лишних колонок (date, created_at) в SELECT
	var result struct {
		WordViolations  int64 `gorm:"column:word_violations"`
		LinkViolations  int64 `gorm:"column:link_violations"`
		ImageViolations int64 `gorm:"column:image_violations"`
		VideoViolations int64 `gorm:"column:video_violations"`
		AudioViolations int64 `gorm:"column:audio_violations"`
		FileViolations  int64 `gorm:"column:file_violations"`
		MuteCount       int64 `gorm:"column:mute_count"`
	}

	err := r.db.WithContext(ctx).
		Table("chat_stats").
		Select(`
			COALESCE(SUM(word_violations), 0) as word_violations,
			COALESCE(SUM(link_violations), 0) as link_violations,
			COALESCE(SUM(image_violations), 0) as image_violations,
			COALESCE(SUM(video_violations), 0) as video_violations,
			COALESCE(SUM(audio_violations), 0) as audio_violations,
			COALESCE(SUM(file_violations), 0) as file_violations,
			COALESCE(SUM(mute_count), 0) as mute_count
		`).
		Where("chat_id = ?", chatID).
		// ✅ Нет GROUP BY: SUM() без GROUP BY агрегирует все строки с chat_id=? в одну
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get chat total stats: %w", err)
	}

	// ✅ Собираем итоговый ChatStats вручную
	return &ChatStats{
		ChatID:          chatID,
		WordViolations:  result.WordViolations,
		LinkViolations:  result.LinkViolations,
		ImageViolations: result.ImageViolations,
		VideoViolations: result.VideoViolations,
		AudioViolations: result.AudioViolations,
		FileViolations:  result.FileViolations,
		MuteCount:       result.MuteCount,
		// Date, CreatedAt, UpdatedAt остаются нулевыми — это нормально для "итоговой" статистики
	}, nil
}
