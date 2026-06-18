# образ скачиваем
FROM golang:1.26-alpine AS builder

# показываем где будем работать
WORKDIR /app

# зависимости скачиваем
COPY go.mod go.sum ./
RUN go mod download

# все остальное скачиваем
COPY . .

# собираем в дирректории /app/server
RUN go build -o /app/server .   

# Готовимся к запуску
FROM alpine:latest

WORKDIR /app

# берем бинарник
COPY --from=builder /app/server /app/server

# порт указываем
EXPOSE 8080

# что запустит при старте контейнера
CMD [ "/app/server" ]