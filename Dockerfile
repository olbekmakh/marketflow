# 1️⃣ Используем официальный образ Go
FROM golang:1.22 AS builder

# 2️⃣ Устанавливаем рабочую директорию
WORKDIR /app

# 3️⃣ Копируем файлы проекта
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 4️⃣ Собираем бинарник
RUN go build -o marketflow .

# 5️⃣ Используем минимальный образ для запуска
FROM debian:bookworm-slim

WORKDIR /app

# 6️⃣ Копируем бинарник из builder-образа
COPY --from=builder /app/marketflow /app/

# 7️⃣ Указываем порт, который будет слушать приложение
EXPOSE 8080

# 8️⃣ Команда запуска
ENTRYPOINT ["./marketflow"]
