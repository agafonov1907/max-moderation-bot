# === Этап 1: Сборка ===
FROM golang:1.25-alpine AS builder

# Установка зависимостей
RUN apk add --no-cache git ca-certificates tzdata

# Рабочая директория
WORKDIR /app

# Копирование go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bot ./cmd/bot

# === Этап 2: Финальный образ ===
FROM alpine:3.19

# Установка часового пояса и корневых сертификатов
RUN apk --no-cache add ca-certificates tzdata

# Рабочая директория
WORKDIR /root/

# Копирование бинарника из builder
COPY --from=builder /app/bot .

# Копирование конфигурационных файлов
COPY --from=builder /app/configs/ ./configs/
#COPY --from=builder /app/.env .

# Порт для webhook (если используется)
EXPOSE 8080

# Запуск бота
CMD ["./bot"]
