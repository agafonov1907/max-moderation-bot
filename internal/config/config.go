package config

import (
	"fmt"
	"log"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	BotToken    string `env:"BOT_TOKEN,required"`
	WebhookHost string `env:"WEBHOOK_HOST"`
	Port        string `env:"PORT" envDefault:"8080"`
	MetricsAddr string `env:"METRICS_ADDR" envDefault:":9090"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`

	DBHost      string `env:"DB_HOST,required"`
	DBPort      string `env:"DB_PORT" envDefault:"5432"`
	DBUser      string `env:"DB_USER,required"`
	DBPassword  string `env:"DB_PASSWORD,required"`
	DBName      string `env:"DB_NAME,required"`
	EnableCache bool   `env:"ENABLE_CACHE" envDefault:"false"`

	DefaultMuteDuration    string `env:"DEFAULT_MUTE_DURATION" envDefault:"30m"`
	EnableTelemetry        bool   `env:"ENABLE_TELEMETRY" envDefault:"true"`
	GroupLinkedSuccessText string `env:"GROUP_LINKED_SUCCESS_TEXT" envDefault:""`
	AdminUserIDs           []int64 `env:"ADMIN_USER_IDS" envSeparator:","`
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName)
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	log.Printf("Config loaded. Port: %s, LogLevel: %s", cfg.Port, cfg.LogLevel)
	return cfg, nil
}
