package repository

import (
	"fmt"

	"gorm.io/gorm"
)

type ChatAdminRepository interface {
	IsAdmin(chatID, userID int64) (bool, error)
	AddAdmin(chatID, userID int64) error
	RemoveAdmin(chatID, userID int64) error
	GetManagedChats(userID int64) ([]int64, error)
	GetManagedChatsPaginated(userID int64, offset, limit int) ([]int64, int64, error)
}
type PostgresChatAdminRepository struct {
	db *gorm.DB
}

func NewChatAdminRepository(db *gorm.DB) ChatAdminRepository {
	return &PostgresChatAdminRepository{db: db}
}
func (r *PostgresChatAdminRepository) IsAdmin(chatID, userID int64) (bool, error) {
	var count int64
	err := r.db.Model(&ChatAdmin{}).Where("chat_id = ? AND user_id = ?", chatID, userID).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}
	return count > 0, nil
}
func (r *PostgresChatAdminRepository) AddAdmin(chatID, userID int64) error {
	admin := ChatAdmin{
		ChatID: chatID,
		UserID: userID,
	}
	if err := r.db.Where(ChatAdmin{ChatID: chatID, UserID: userID}).FirstOrCreate(&admin).Error; err != nil {
		return fmt.Errorf("failed to add admin: %w", err)
	}
	return nil
}
func (r *PostgresChatAdminRepository) RemoveAdmin(chatID, userID int64) error {
	if err := r.db.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&ChatAdmin{}).Error; err != nil {
		return fmt.Errorf("failed to remove admin: %w", err)
	}
	return nil
}
func (r *PostgresChatAdminRepository) GetManagedChats(userID int64) ([]int64, error) {
	var admins []ChatAdmin
	if err := r.db.Where("user_id = ?", userID).Find(&admins).Error; err != nil {
		return nil, fmt.Errorf("failed to get managed chats: %w", err)
	}
	chatIDs := make([]int64, len(admins))
	for i, admin := range admins {
		chatIDs[i] = admin.ChatID
	}
	return chatIDs, nil
}

func (r *PostgresChatAdminRepository) GetManagedChatsPaginated(userID int64, offset, limit int) ([]int64, int64, error) {
	var admins []ChatAdmin
	var total int64

	query := r.db.Model(&ChatAdmin{}).Where("user_id = ?", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count managed chats: %w", err)
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&admins).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get paginated managed chats: %w", err)
	}

	chatIDs := make([]int64, len(admins))
	for i, admin := range admins {
		chatIDs[i] = admin.ChatID
	}
	return chatIDs, total, nil
}
