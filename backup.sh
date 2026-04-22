#!/bin/bash

# === Настройки ===
# Путь к папке проекта
PROJECT_DIR="/home/vagafonov/Рабочий стол/max-moderation-bot-main"

# Путь к папке для бэкапов
BACKUP_DIR="${PROJECT_DIR}/backups"

# Имя проекта для docker compose (по умолчанию - имя папки)
COMPOSE_PROJECT_NAME="max-moderation-bot-main"

# Настройки БД (должны совпадать с .env)
DB_USER="maxbot"
DB_NAME="maxbot_db"

# Сколько дней хранить бэкапы
DAYS_TO_KEEP=7

# === Основной код ===

# Создаём папку для бэкапов, если нет
mkdir -p "${BACKUP_DIR}"

# Формируем имя файла с датой
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/backup_${DATE}.sql"

echo "[$(date)] Начало бэкапа: ${BACKUP_FILE}"

# Переходим в папку проекта
cd "${PROJECT_DIR}" || exit 1

# Делаем дамп БД через docker compose
docker compose exec -T db pg_dump -U "${DB_USER}" "${DB_NAME}" > "${BACKUP_FILE}"

# Проверяем, что бэкап создан и не пустой
if [ -s "${BACKUP_FILE}" ]; then
    SIZE=$(du -h "${BACKUP_FILE}" | cut -f1)
    echo "[$(date)] ✅ Бэкап создан: ${BACKUP_FILE} (${SIZE})"
else
    echo "[$(date)] ❌ Ошибка: бэкап пустой или не создан!"
    exit 1
fi

# Сжимаем бэкап (опционально, экономит место)
if command -v gzip &> /dev/null; then
    gzip "${BACKUP_FILE}"
    echo "[$(date)] ✅ Бэкап сжат: ${BACKUP_FILE}.gz"
fi

# Удаляем старые бэкапы
echo "[$(date)] Удаление бэкапов старше ${DAYS_TO_KEEP} дней..."
find "${BACKUP_DIR}" -name "backup_*.sql*" -type f -mtime +${DAYS_TO_KEEP} -delete

echo "[$(date)] ✅ Бэкап завершён"