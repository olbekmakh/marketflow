# 1️⃣ Этап сборки бинарника
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем бинарник
RUN go build -o marketflow .

# 2️⃣ Этап финального образа
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из первого этапа
COPY --from=builder /app/marketflow /app/marketflow

# Копируем конфигурацию
COPY config.yaml /app/config.yaml

# Устанавливаем временную зону и зависимости (опционально)
RUN apk add --no-cache tzdata

# Открываем порт
EXPOSE 8080

# Команда запуска
ENTRYPOINT ["/app/marketflow"]
