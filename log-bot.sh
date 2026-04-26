#!/bin/bash

# Папка для логов
LOG_DIR="/home/vagafonov/Рабочий\ стол/max-moderation-bot-main"
mkdir -p "${LOG_DIR}"

# Дата для имени файла
DATE=$(date +%Y%m%d_%H%M%S)
LOG_FILE="${LOG_DIR}/bot_${DATE}.log"

echo "Начало записи логов: ${LOG_FILE}"
echo "Нажмите Ctrl+C для остановки"

docker compose logs -f bot > "${LOG_FILE}" 2>&1