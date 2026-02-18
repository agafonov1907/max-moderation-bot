package repository

import (
	"time"

	"github.com/lib/pq"
)

type ChatSettings struct {
	ChatID           int64          `gorm:"primaryKey;autoIncrement:false"`
	BlockedWords     pq.StringArray `gorm:"type:text[]"`
	BlockedDomains   pq.StringArray `gorm:"type:text[]"`
	RestrictImage    bool           `gorm:"default:false"`
	RestrictVideo    bool           `gorm:"default:false"`
	RestrictAudio    bool           `gorm:"default:false"`
	RestrictFile     bool           `gorm:"default:false"`
	EnableWordFilter bool           `gorm:"default:true"`
	EnableLinkFilter bool           `gorm:"default:true"`
	EnableMute       bool           `gorm:"default:false"`
	EnableAutoDelete bool           `gorm:"default:true"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type UserState struct {
	UserID    int64  `gorm:"primaryKey"`
	ChatID    int64  `gorm:"not null"`
	Action    string `gorm:"not null"`
	CreatedAt time.Time
}
type Mute struct {
	ID        uint      `gorm:"primaryKey"`
	ChatID    int64     `gorm:"index"`
	UserID    int64     `gorm:"index"`
	UserName  string    `gorm:"size:255"`
	ExpiresAt time.Time `gorm:"index"`
}
type LinkToken struct {
	Token     string    `gorm:"primaryKey"`
	UserID    int64     `gorm:"index"`
	ExpiresAt time.Time `gorm:"index"`
}
type ChatAdmin struct {
	ID        uint  `gorm:"primaryKey"`
	ChatID    int64 `gorm:"index:idx_chat_user,unique"`
	UserID    int64 `gorm:"index:idx_chat_user,unique"`
	CreatedAt time.Time
}

type ChatStats struct {
	ChatID          int64     `gorm:"primaryKey;autoIncrement:false"`
	Date            time.Time `gorm:"primaryKey;type:date"`
	WordViolations  int64     `gorm:"default:0"`
	LinkViolations  int64     `gorm:"default:0"`
	ImageViolations int64     `gorm:"default:0"`
	VideoViolations int64     `gorm:"default:0"`
	AudioViolations int64     `gorm:"default:0"`
	FileViolations  int64     `gorm:"default:0"`
	MuteCount       int64     `gorm:"default:0"`
}
