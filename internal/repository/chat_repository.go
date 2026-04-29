package repository

import (
	"fmt"
	"gorm.io/gorm"
)

// ChatRepository определяет методы для работы с информацией о чатах
type ChatRepository interface {
	GetManagedChatsWithNames(userID int64) ([]ChatInfo, error)
	GetChatName(chatID int64) (string, error)
}

// PostgresChatRepository реализует ChatRepository для PostgreSQL
type PostgresChatRepository struct {
	db *gorm.DB
}

// NewChatRepository создаёт новый экземпляр репозитория
func NewChatRepository(db *gorm.DB) ChatRepository {
	return &PostgresChatRepository{db: db}
}

// GetManagedChatsWithNames возвращает список чатов пользователя с названиями
// Предполагаемая схема БД: таблица "groups" с полями id, name, admin_user_id
// И таблица "group_members" для связи пользователей с чатами
func (r *PostgresChatRepository) GetManagedChatsWithNames(userID int64) ([]ChatInfo, error) {
	var chats []ChatInfo

	// Запрос получает чаты, где пользователь является админом
	// Адаптируйте под вашу схему БД!
	err := r.db.Table("groups").
		Select("groups.id, groups.name").
		Where("groups.admin_user_id = ? OR groups.id IN (SELECT group_id FROM group_members WHERE user_id = ? AND is_admin = true)",
			userID, userID).
		Order("groups.name ASC").
		Scan(&chats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get chats with names: %w", err)
	}

	return chats, nil
}

// GetChatName возвращает название чата по его ID
func (r *PostgresChatRepository) GetChatName(chatID int64) (string, error) {
	var name string
	err := r.db.Table("groups").
		Select("name").
		Where("id = ?", chatID).
		Scan(&name).Error

	if err != nil {
		return "", fmt.Errorf("failed to get chat name: %w", err)
	}
	return name, nil
}
