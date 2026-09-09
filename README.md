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
| Docker Engine + Docker Compose | Запускают приложение, PostgreSQL и Caddy |
| Домен | Например, `app.example.com`; его A-запись должна вести на IP VPS |
| Открытые порты `80` и `443` | Caddy выпустит HTTPS-сертификат автоматически |
| Telegram Bot Token | Получается у [@BotFather](https://t.me/BotFather) |
| Telegram ID администратора | Узнать можно у [@userinfobot](https://t.me/userinfobot) |
| URL и API Token Remnawave | Для выдачи VPN-подписок |

### Одна команда

После публикации репозитория в GitHub установщик можно запустить прямо на VPS:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/bruhxax/TGS-bot/main/install.sh)
```

Он скачает проект в `/opt/tgs-bot`, задаст нужные вопросы, сгенерирует секреты, поднимет Docker-контейнеры и включит интеграцию с Remnawave.

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
Интернет → HTTPS / Caddy → TGS-bot → PostgreSQL
                         ├→ Telegram Bot API
                         └→ Remnawave API
```

## ✅ После установки

1. Открой в Telegram своего бота и отправь `/start`.
2. Твой ID из установщика автоматически получит права администратора.
3. Открой Mini App: `https://твой-домен/app`.
4. В разделе **Админ → Интеграции** добавь ЮKassa, CryptoBot и уведомления, если они нужны.
5. Создай или отредактируй тарифы, триал и сквады в админке.

## 🔗 Webhook-адреса

После установки в кабинетах сервисов используй эти адреса:

| Сервис | Адрес |
| --- | --- |
| Remnawave | `https://<домен>/api/webhooks/remnawave` |
| ЮKassa | `https://<домен>/api/webhooks/yookassa` |
| CryptoBot | `https://<домен>/api/webhooks/cryptobot` |

Если для Remnawave задан webhook secret, панель должна передавать его в заголовке `X-Webhook-Secret`.

## 🧰 Управление на VPS

```bash
cd /opt/tgs-bot

# Посмотреть состояние контейнеров
docker compose ps

# Смотреть логи приложения
docker compose logs -f app

# Обновить проект после git pull
git pull
docker compose up -d --build

# Остановить проект
docker compose down
```

Данные PostgreSQL хранятся в Docker volume и не удаляются обычной командой `docker compose down`.

## 🧑‍💻 Разработка

Локальный предпросмотр интерфейса доступен по адресу `/app?preview=1`. Он показывает демонстрационные данные и не создаёт серверную сессию.

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

## 📄 Лицензия

Исходный код распространяется по [закрытой лицензии](LICENSE).
