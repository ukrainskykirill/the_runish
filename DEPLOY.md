# Деплой The Runish на Timeweb Cloud

Полная инструкция: от пула в GitHub до работающего сайта на VPS Timeweb.

## Архитектура

```
GitHub (push main)
  └─ GitHub Actions CI
       ├─ test (go vet, build, test)
       ├─ docker (build → push в GHCR)
       └─ deploy (SSH на VPS → docker compose pull → up -d)

Timeweb Cloud
  ├─ VPS Ubuntu 24.04 (web + worker контейнеры)
  ├─ Managed PostgreSQL 18 (облачная БД)
  └─ домен → Caddy (reverse proxy, TLS) → 127.0.0.1:8080
```

---

## 1. Репозиторий на GitHub

```bash
# Если ещё не инициализирован
git init
git add .
git commit -m "init"
git branch -M main
git remote add origin git@github.com:<ВАШ_ЛОГИН>/therunish.git
git push -u origin main
```

Пуш в `main` запустит CI автоматически.

---

## 2. GitHub Secrets

В репозитории: **Settings → Secrets and variables → Actions → New repository secret**

| Secret | Значение | Обязательный |
|--------|----------|:---:|
| `SSH_HOST` | IP-адрес VPS (например `185.46.8.x`) | ✅ |
| `SSH_USER` | Пользователь SSH (например `root` или `deploy`) | ✅ |
| `SSH_KEY` | Приватный SSH-ключ (содержимое `~/.ssh/id_ed25519`) | ✅ |
| `SSH_PORT` | Порт SSH, если нестандартный (по умолчанию `22`) | — |
| `GHCR_PAT` | PAT с `read:packages` (см. ниже — нужен только для private-образа) | — |

### Генерация SSH-ключа для CI

**На локальной машине:**

```bash
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ~/.ssh/therunish_ci
```

- **Приватный ключ** (`cat ~/.ssh/therunish_ci`) → вставить в Secret `SSH_KEY`
- **Публичный ключ** (`cat ~/.ssh/therunish_ci.pub`) → добавить на VPS в `~/.ssh/authorized_keys`

### GHCR PAT (только если образ приватный)

1. GitHub → **Settings → Developer settings → Personal access tokens → Fine-grained**
2. Name: `therunish-ghcr`, Expiration: 90 дней
3. Scope: `read:packages` для репозитория
4. Токен → Secret `GHCR_PAT`

> **Рекомендация:** сделайте пакет публичным (Settings пакета на GHCR → Change visibility → Public).
> Тогда `GHCR_PAT` не нужен, и сервер будет тянуть образ без аутентификации.

---

## 3. Настройка VPS Timeweb (один раз)

### 3.1. Подключение

```bash
ssh root@<VPS_IP>
```

### 3.2. Установка Docker

```bash
apt update && apt upgrade -y
curl -fsSL https://get.docker.com | sh
apt install -y docker-compose-plugin
systemctl enable --now docker
docker --version && docker compose version
```

### 3.3. Рабочая директория

```bash
mkdir -p ~/therunish
cd ~/therunish
```

### 3.4. Файл `.env`

Создайте `.env` на сервере (не коммитьте в репозиторий!):

```bash
cat > ~/therunish/.env << 'EOF'
# БД — облачная (скопируйте из панели Timeweb → Базы данных)
DATABASE_URL=postgres://<user>:<password>@<host>:<port>/<dbname>?sslmode=require

# HTTP
HTTP_ADDR=:8080
BASE_URL=https://therunish.ru

# Сессии
SESSION_TTL=720h

# Telegram
BOT_TOKEN=<боевой_токен>
BOT_USERNAME=therunish_bot

# T-Bank
PAYMENT_PROVIDER=tbank
TBANK_TERMINAL_KEY=<ключ>
TBANK_PASSWORD=<пароль>
TBANK_API_BASE=https://securepay.tinkoff.ru/v2

# Admin
ADMIN_TOKEN=<длинный_рандомный_токен>
ADMIN_LOGIN=admin
ADMIN_PASSWORD=<сильный_пароль>

# Логирование
LOG_LEVEL=INFO
EOF
chmod 600 ~/therunish/.env
```

> Значения для `DATABASE_URL` возьмите в панели Timeweb Cloud → Базы данных → подключение.
> По умолчанию там PostgreSQL 18, формат: `postgres://<user>:<pass>@<host>:<port>/<db>?sslmode=require`

### 3.5. `IMAGE_OWNER` для compose

Задайте логин владельца образа на GitHub (это тот же логин, что в пути к репозиторию):

```bash
# В ~/therunish/.env допишите:
IMAGE_OWNER=<ВАШ_GITHUB_ЛОГИН>
IMAGE_TAG=latest
```

`docker-compose.prod.yml` подставит их в имя образа
(`ghcr.io/<IMAGE_OWNER>/therunish:<IMAGE_TAG>`).

---

## 4. Reverse Proxy + TLS (Caddy)

Caddy автоматически выпускает Let's Encrypt сертификаты.

### Установка

```bash
apt install -y debian-keyring debian-archive-keyring apt-transport-https
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
apt update
apt install -y caddy
```

### Конфигурация

```bash
cat > /etc/caddy/Caddyfile << 'EOF'
therunish.ru {
    encode gzip zstd
    reverse_proxy 127.0.0.1:8080
}
EOF
systemctl restart caddy
systemctl enable caddy
```

> Не забудьте направить A-запись домена `therunish.ru` на IP вашего VPS в DNS-панели регистратора.

---

## 5. Первый деплой

После настройки:

1. **Запуште код** в `main` — CI соберёт образ и запушит в GHCR.
2. **CI скопирует** `docker-compose.prod.yml` на сервер и выполнит `docker compose pull && up -d`.
3. **Проверьте:**

```bash
# На VPS
cd ~/therunish
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs -f --tail=50
```

4. Откройте `https://therunish.ru` в браузере.

### Ручной деплой (без CI)

```bash
# На VPS
cd ~/therunish
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d --remove-orphans
docker image prune -f
```

---

## 6. Миграции БД

Миграции применяются автоматически при старте `web` (читает `/app/migrations/`).
Проверить схему:

```bash
# На VPS, подключившись к облачной БД
psql "postgres://<user>:<pass>@<host>:<port>/<db>?sslmode=require" -c "\dt"
```

---

## 7. Полезные команды

| Действие | Команда |
|----------|---------|
| Логи web | `docker compose -f docker-compose.prod.yml logs -f web` |
| Логи worker | `docker compose -f docker-compose.prod.yml logs -f worker` |
| Перезапуск | `docker compose -f docker-compose.prod.yml restart` |
| Остановить | `docker compose -f docker-compose.prod.yml down` |
| Статус | `docker compose -f docker-compose.prod.yml ps` |

---

## Чек-лист первого запуска

- [ ] Репозиторий запушен на GitHub
- [ ] Secrets добавлены: `SSH_HOST`, `SSH_USER`, `SSH_KEY`
- [ ] VPS: Docker установлен
- [ ] VPS: `~/therunish/.env` создан и заполнен
- [ ] DNS: A-запись домена направлена на IP VPS
- [ ] Caddy установлен и настроен
- [ ] CI отработал без ошибок (вкладка Actions)
- [ ] Сайт открывается по `https://<домен>`