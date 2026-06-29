# ---- Frontend build stage ----
FROM node:22-alpine AS frontend

WORKDIR /app/web/frontend

COPY web/frontend/package.json web/frontend/package-lock.json ./
RUN npm ci

COPY web/frontend/ ./
RUN npm run build

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Кэшируем модули.
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники.
COPY . .

# Собираем два бинарника.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/therunish-web ./cmd/web
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/therunish-worker ./cmd/worker

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Копируем бинарники.
COPY --from=builder /bin/therunish-web /app/therunish-web
COPY --from=builder /bin/therunish-worker /app/therunish-worker

# Копируем веб-статику и миграции.
COPY web/ /app/web/
COPY migrations/ /app/migrations/

# Собранный React SPA (заменяет dist из контекста сборки).
COPY --from=frontend /app/web/frontend/dist /app/web/frontend/dist

ENV TZ=Europe/Moscow

# По умолчанию запускаем web; worker запускается отдельно.
CMD ["/app/therunish-web"]
