package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// ChatMemberCache — модель кэша участников
type ChatMemberCache struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	ChatID      int64     `gorm:"not null" json:"chat_id"` // ← uniqueIndex убран! Уникальность уже есть в БД (uniq_chat_id)
	MemberCount int       `gorm:"default:0" json:"member_count"`
	LastUpdated time.Time `gorm:"default:now()" json:"last_updated"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// 🔥 КРИТИЧНО: фиксируем имя таблицы для GORM (если будет использоваться)
func (ChatMemberCache) TableName() string {
	return "chat_member_caches" // ← с "s", как в БД
}

// ChatActivity — модель активности чата по дням
type ChatActivity struct {
	ChatID        int64     `json:"chat_id"`
	Date          string    `json:"date"`
	MessagesCount int       `json:"messages_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdateMemberCache обновляет кэш количества участников
func (r *PostgresRepository) UpdateMemberCache(ctx context.Context, chatID int64, memberCount int) error {
	// ✅ ИСПРАВЛЕНО: chat_member_cache → chat_member_caches
	query := `
		INSERT INTO chat_member_caches (chat_id, member_count, last_updated, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (chat_id) 
		DO UPDATE SET 
			member_count = $2,
			last_updated = NOW(),
			updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, chatID, memberCount)
	return err
}

// GetMemberCache получает кэш количества участников
func (r *PostgresRepository) GetMemberCache(ctx context.Context, chatID int64) (*ChatMemberCache, error) {
	// ✅ ИСПРАВЛЕНО: chat_member_cache → chat_member_caches
	query := `SELECT chat_id, member_count, last_updated, created_at, updated_at 
			  FROM chat_member_caches WHERE chat_id = $1`

	var cache ChatMemberCache
	err := r.db.QueryRowContext(ctx, query, chatID).Scan(
		&cache.ChatID, &cache.MemberCount, &cache.LastUpdated, &cache.CreatedAt, &cache.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

// GetAllMemberCache получает кэш для всех чатов пользователя
func (r *PostgresRepository) GetAllMemberCache(ctx context.Context, chatIDs []int64) (map[int64]*ChatMemberCache, error) {
	result := make(map[int64]*ChatMemberCache)

	if len(chatIDs) == 0 {
		return result, nil
	}

	// ✅ ИСПРАВЛЕНО: chat_member_cache → chat_member_caches
	query := `SELECT chat_id, member_count, last_updated FROM chat_member_caches WHERE chat_id = ANY($1)`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(chatIDs))
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var cache ChatMemberCache
		if err := rows.Scan(&cache.ChatID, &cache.MemberCount, &cache.LastUpdated); err != nil {
			continue
		}
		result[cache.ChatID] = &cache
	}

	return result, rows.Err()
}

// GetLastCacheUpdate возвращает время последнего обновления кэша
func (r *PostgresRepository) GetLastCacheUpdate(ctx context.Context, chatIDs []int64) (time.Time, error) {
	if len(chatIDs) == 0 {
		return time.Time{}, nil
	}

	// ✅ ИСПРАВЛЕНО: chat_member_cache → chat_member_caches
	query := `SELECT MAX(last_updated) FROM chat_member_caches WHERE chat_id = ANY($1)`

	var lastUpdated sql.NullTime
	err := r.db.QueryRowContext(ctx, query, pq.Array(chatIDs)).Scan(&lastUpdated)
	if err != nil {
		return time.Time{}, err
	}

	if !lastUpdated.Valid {
		return time.Time{}, nil
	}

	return lastUpdated.Time, nil
}

// GetChatActivity получает активность чата за день
// ⚠️ Эта функция использует таблицу chat_activity (другая таблица, не трогаем)
func (r *PostgresRepository) GetChatActivity(ctx context.Context, chatID int64, date string) (*ChatActivity, error) {
	query := `SELECT chat_id, date, messages_count, created_at, updated_at 
			  FROM chat_activity WHERE chat_id = $1 AND date = $2`

	var activity ChatActivity
	err := r.db.QueryRowContext(ctx, query, chatID, date).Scan(
		&activity.ChatID, &activity.Date, &activity.MessagesCount,
		&activity.CreatedAt, &activity.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &activity, nil
}
