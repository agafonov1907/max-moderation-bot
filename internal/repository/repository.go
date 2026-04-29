package repository

import (
	"database/sql"
)

// PostgresRepository — универсальный репозиторий для общих запросов к БД
// Используется для мониторинга, кэша и других операций, не требующих специфичной логики
type PostgresRepository struct {
	db *sql.DB
}

// NewPostgresRepository создаёт новый экземпляр универсального репозитория
func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// DB возвращает прямую ссылку на *sql.DB (если нужно выполнить сырой запрос)
func (r *PostgresRepository) DB() *sql.DB {
	return r.db
}
