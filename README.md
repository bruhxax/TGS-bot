<div align="center">

# TGS-bot

### Telegram-бот и Mini App для продажи VPN-подписок через Remnawave

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![Telegram](https://img.shields.io/badge/Telegram-Mini%20App-26A5E4?logo=telegram&logoColor=white)](https://core.telegram.org/bots/webapps)
[![Remnawave](https://img.shields.io/badge/Remnawave-API-8B5CF6)](https://remna.st/)

[⚡ Установить на VPS](#-быстрый-запуск) · [Возможности](#-возможности) · [Настройка](#-после-установки) · [Webhook](#-webhook-адреса)

</div>

> Всё для VPN-сервиса в одном месте: бот, личный кабинет, выдача подписок, платежи, поддержка и админка.

## ✨ Возможности

| Раздел | Что умеет |
| --- | --- |
| 🤖 Telegram-бот | `/start`, Mini App, триал, реферальные ссылки, рассылки с предпросмотром |
| 📱 Mini App | Подписка, трафик, устройства, тарифы, промокоды, оплата, поддержка |
| 🌊 Remnawave | Создание и продление пользователей, сквады, ноды, HWID-устройства |
| 💳 Оплата | ЮKassa и CryptoBot с проверкой webhook перед выдачей подписки |
| 🛠️ Админка | Тарифы, пользователи, промокоды, тикеты, рассылки, диагностика, тема и контент |
| 🔐 Безопасность | Проверка Telegram Mini App, подписанные сессии, PostgreSQL и автоматические миграции |

## 🚀 Быстрый запуск

### Перед началом

| Нужно | Зачем |
| --- | --- |
| Linux VPS | Рекомендуется Ubuntu 22.04 / 24.04 |
| Docker Engine + Docker Compose | Запускают приложение и PostgreSQL |
| Домен | Например, `app.example.com`; его A-запись должна вести на IP VPS |
| Открытые порты `80` и `443` | Нужны для HTTPS; установщик сам проверит, кто их использует |
| Telegram Bot Token | Получается у [@BotFather](https://t.me/BotFather) |
| Telegram ID администратора | Узнать можно у [@userinfobot](https://t.me/userinfobot) |
| URL и API Token Remnawave | Для выдачи VPN-подписок |

### Одна команда

После публикации репозитория в GitHub установщик можно запустить прямо на VPS:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bruhxax/TGS-bot/main/install.sh)
```

Он скачает проект в `/opt/tgs-bot`, задаст нужные вопросы, сгенерирует секреты, поднимет Docker-контейнеры и включит интеграцию с Remnawave.

Установщик сам проверяет `80/443` и действующие сервисы. На чистой VPS он использует встроенный Caddy. Если найдёт системный **nginx** или **Caddy**, предложит безопасно подключить TGS-bot к нему на `127.0.0.1:8080` и настроит HTTPS. Apache, Traefik и reverse proxy в Docker он распознаёт, но не переписывает их конфигурацию без готового адаптера.

> Пока репозиторий приватный, прямая ссылка GitHub недоступна посторонним. Для приватной установки сначала клонируй проект по SSH, затем запусти `./install.sh` из его папки.

```bash
git clone git@github.com:bruhxax/TGS-bot.git /opt/tgs-bot
cd /opt/tgs-bot
chmod +x install.sh
./install.sh
```

### Что спросит установщик

```text
🤖 Токен Telegram-бота
🌐 Публичный HTTPS URL Mini App
👤 Telegram ID администратора
🌊 URL панели Remnawave
🔑 API Token Remnawave
```

Он сам создаёт защищённый `.env`, генерирует `APP_SECRET` и пароль PostgreSQL. Секреты не выводятся после установки, а существующий `.env` скрипт никогда не перезаписывает.

```text
Интернет → HTTPS / reverse proxy → TGS-bot → PostgreSQL
                              ├→ Telegram Bot API
                              └→ Remnawave API
```

## ✅ После установки

1. Открой в Telegram своего бота и отправь `/start`.
2. Твой ID из установщика автоматически получит права администратора.
3. Открой Mini App: `https://твой-домен/app`.
4. В разделе **Админ → Интеграции** добавь ЮKassa, CryptoBot и уведомления, если они нужны.
5. Создай или отредактируй тарифы, триал и сквады в админке.

### Работа рядом с существующим веб-сервером

При найденном системном nginx установщик создаёт отдельный сайт только для указанного домена, проверяет конфигурацию, перезагружает nginx и выпускает сертификат через Certbot. Остальные сайты не изменяются.

При системном Caddy он добавляет отдельный файл в `/etc/caddy/conf.d/` и направляет домен на `127.0.0.1:8080`; Caddy сам выдаёт HTTPS.

Для Apache, Traefik и прокси в Docker приложение запускается безопасно только на `127.0.0.1:8080`. Установщик покажет готовый upstream, а конфигурацию существующего прокси нужно добавить вручную.

## 🔗 Webhook-адреса

После установки в кабинетах сервисов используй эти адреса:

| Сервис | Адрес |
| --- | --- |
| Remnawave | `https://<домен>/api/webhooks/remnawave` |
| ЮKassa | `https://<домен>/api/webhooks/yookassa` |
| CryptoBot | `https://<домен>/api/webhooks/cryptobot` |

Если для Remnawave задан webhook secret, панель должна передавать его в заголовке `X-Webhook-Secret`.

## 🧰 Управление на VPS

Все команды выполняются из каталога проекта:

```bash
cd /opt/tgs-bot
```

### Статус и логи

```bash
# Состояние всех контейнеров
docker compose ps

# Логи бота в реальном времени
docker compose logs -f --tail=200 app

# Последние логи базы данных
docker compose logs --tail=200 db
```

### Запуск, остановка и перезапуск

```bash
# Запустить весь проект
docker compose up -d

# Остановить без удаления базы
docker compose stop

# Перезапустить только бота
docker compose restart app

# Остановить и удалить контейнеры, сохранив данные
docker compose down
```

### Обновление из GitHub

Перед обновлением закоммить или убери свои локальные правки на VPS. `--ff-only` специально останавливает обновление при конфликте и не перезаписывает чужие изменения.

```bash
cd /opt/tgs-bot
git fetch origin
git pull --ff-only origin main
docker compose up -d --build
docker compose ps
curl -fsS https://твой-домен/health
```

### Резервная копия PostgreSQL

```bash
cd /opt/tgs-bot
mkdir -p /opt/tgs-bot/backups
docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "/opt/tgs-bot/backups/tgsbot-$(date +%Y%m%d-%H%M%S).dump"
ls -lh /opt/tgs-bot/backups
```

### Восстановление PostgreSQL

Сначала укажи точное имя нужной копии. На время восстановления бот останавливается.

```bash
cd /opt/tgs-bot
docker compose stop app
docker compose exec -T db sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists' \
  < /opt/tgs-bot/backups/tgsbot-YYYYMMDD-HHMMSS.dump
docker compose up -d app
docker compose ps
```

### Очистка базы или полное удаление

> Команды ниже необратимы. Сначала сделай резервную копию.

```bash
# Полностью очистить только базу TGS-bot и создать её заново
cd /opt/tgs-bot
docker compose down
docker volume rm tgs_bot_v2_postgres
docker compose up -d --build
```

```bash
# Полностью удалить TGS-bot, его БД и данные встроенного Caddy
cd /opt/tgs-bot
docker compose down --volumes --remove-orphans
cd /opt
rm -rf /opt/tgs-bot
```

Обычные `stop`, `restart`, `up` и `down` не удаляют PostgreSQL volume. Другие проекты и контейнеры этими командами не затрагиваются.

## 🧑‍💻 Разработка

Локальный предпросмотр интерфейса доступен по адресу `/app?preview=1`. Он показывает демонстрационные данные и не создаёт серверную сессию.

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

## 📄 Лицензия

Исходный код распространяется по [закрытой лицензии](LICENSE).
