package repository

import (
	"time"

	"github.com/lib/pq"
)

type ChatSettings struct {
	ChatID           int64          `gorm:"primaryKey" json:"chat_id"`
	BlockedWords     pq.StringArray `gorm:"type:text[]" json:"blocked_words"`
	BlockedDomains   pq.StringArray `gorm:"type:text[]" json:"blocked_domains"`
	RestrictImage    bool           `gorm:"default:false" json:"restrict_image"`
	RestrictVideo    bool           `gorm:"default:false" json:"restrict_video"`
	RestrictAudio    bool           `gorm:"default:false" json:"restrict_audio"`
	RestrictFile     bool           `gorm:"default:false" json:"restrict_file"`
	EnableWordFilter bool           `gorm:"default:true" json:"enable_word_filter"`
	EnableLinkFilter bool           `gorm:"default:true" json:"enable_link_filter"`
	EnableMute       bool           `gorm:"default:true" json:"enable_mute"`
	EnableAutoDelete bool           `gorm:"default:true" json:"enable_auto_delete"`
	CreatedAt        time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

type UserState struct {
	UserID    int64     `gorm:"primaryKey" json:"user_id"`
	ChatID    int64     `gorm:"not null" json:"chat_id"`
	Action    string    `gorm:"not null" json:"action"`
	Metadata  string    `gorm:"type:text" json:"metadata"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// 🔥 КРИТИЧНО: Горм по умолчанию делает "user_states". Фиксируем "user_state":
func (UserState) TableName() string {
	return "user_state"
}

type Mute struct {
	ChatID    int64     `gorm:"primaryKey" json:"chat_id"`
	UserID    int64     `gorm:"primaryKey" json:"user_id"`
	UserName  string    `gorm:"size:255" json:"user_name"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type LinkToken struct {
	Token     string    `gorm:"primaryKey" json:"token"`
	UserID    int64     `gorm:"index" json:"user_id"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type ChatAdmin struct {
	ChatID    int64     `gorm:"primaryKey" json:"chat_id"`
	UserID    int64     `gorm:"primaryKey" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type ChatStats struct {
	ChatID          int64     `gorm:"primaryKey" json:"chat_id"`
	Date            time.Time `gorm:"primaryKey;type:date" json:"date"`
	WordViolations  int64     `gorm:"default:0" json:"word_violations"`
	LinkViolations  int64     `gorm:"default:0" json:"link_violations"`
	ImageViolations int64     `gorm:"default:0" json:"image_violations"`
	VideoViolations int64     `gorm:"default:0" json:"video_violations"`
	AudioViolations int64     `gorm:"default:0" json:"audio_violations"`
	FileViolations  int64     `gorm:"default:0" json:"file_violations"`
	MuteCount       int64     `gorm:"default:0" json:"mute_count"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type ChatInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
