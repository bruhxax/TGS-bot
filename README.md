<div align="center">

# TGS-bot

Telegram-бот и Mini App для продажи VPN-подписок через Remnawave

[Открыть бота](https://t.me/rwTGS_bot) · [Mini App](https://tgs-bot.mooo.com/app) · [Remnawave](https://remna.st/)

</div>

---

## 🧩 Что такое TGS-bot?

TGS-bot объединяет личный кабинет, тарифы, оплату, поддержку и управление VPN-сервисом в одном Telegram Mini App.

Всё основное настраивается через встроенную админку — без ручного редактирования конфигурации.

## ✨ Возможности

| Для пользователей | Для администратора |
| --- | --- |
| Личный кабинет и собственная страница подключения | Управление пользователями и перенос подписок |
| Тарифы, промокоды и пробный период | Тарифы, сквады, триал и доступ после окончания |
| 8 платёжных систем | Интеграции и способы оплаты |
| Управление устройствами | Клиенты подключения для разных платформ |
| Поддержка с историей обращений | Тикеты, диагностика и режим аварии |
| Серверы, платежи и реферальная система | Редактор текстов, оформления и рассылок |

## 📋 Требования

| Компонент | Требование |
| --- | --- |
| Сервер | Ubuntu 22.04/24.04 или Debian 12 |
| Docker | Docker Engine и Docker Compose |
| Домен | `A`-запись на IP сервера |
| Порты | Открытые `80` и `443` |
| Telegram | Бот от [@BotFather](https://t.me/BotFather) |
| Remnawave | URL панели и API-токен |

## 🚀 Установка

### 1. Подготовьте VPS

```bash
apt update && apt install -y git curl
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker
```

### 2. Запустите установщик

```bash
git clone git@github.com:bruhxax/TGS-bot.git /opt/tgs-bot
cd /opt/tgs-bot
chmod +x install.sh
./install.sh
```

Установщик запросит:

- токен Telegram-бота;
- HTTPS-адрес Mini App;
- Telegram ID администратора;
- URL и API-токен Remnawave.

После этого он создаст защищённый `.env`, сгенерирует пароли, запустит PostgreSQL и подключит HTTPS. Если на сервере уже есть nginx или Caddy, установщик предложит использовать его.

> Платёжные системы, тарифы, триал, сквады, тексты и оформление настраиваются внутри Mini App: **Админ → нужный раздел**.

## ✅ Первый запуск

1. Отправьте боту `/start`.
2. Откройте Mini App с аккаунта администратора.
3. Проверьте Remnawave в разделе **Админ → Интеграции**.
4. Настройте тарифы, триал и способы оплаты.

## 🔗 Webhook-адреса

| Сервис | Адрес |
| --- | --- |
| Remnawave | `https://ваш-домен/api/webhooks/remnawave` |
| ЮKassa | `https://ваш-домен/api/webhooks/yookassa` |
| CryptoBot | `https://ваш-домен/api/webhooks/cryptobot` |
| LAVA, WATA, Platega, FreeKassa, Heleket, Pally | `https://ваш-домен/api/webhooks/payments/название` |

Проверка работы: `https://ваш-домен/health`

## 🧰 Команды

Все команды выполняются из каталога проекта:

```bash
cd /opt/tgs-bot
```

| Действие | Команда |
| --- | --- |
| Статус | `docker compose ps` |
| Логи бота | `docker compose logs -f --tail=200 app` |
| Перезапуск бота | `docker compose restart app` |
| Остановить | `docker compose stop` |
| Запустить | `docker compose start` |

### Обновление

```bash
git pull --ff-only origin main
docker compose up -d --build
```

Настройки и база данных при обновлении сохраняются.

<details>
<summary><strong>Резервная копия и восстановление базы</strong></summary>

Создать резервную копию:

```bash
mkdir -p /opt/tgs-bot/backups
docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "/opt/tgs-bot/backups/tgsbot-$(date +%Y%m%d-%H%M%S).dump"
```

Восстановить выбранную копию:

```bash
docker compose stop app
docker compose exec -T db sh -c 'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists' \
  < /opt/tgs-bot/backups/tgsbot-YYYYMMDD-HHMMSS.dump
docker compose up -d app
```

</details>

<details>
<summary><strong>Остановка и удаление</strong></summary>

Удалить контейнеры, сохранив базу:

```bash
docker compose down
```

Полностью удалить контейнеры и данные TGS-bot:

```bash
docker compose down --volumes --remove-orphans
```

> ⚠️ Команда с `--volumes` необратимо удаляет базу данных. Сначала создайте резервную копию.

</details>

## 🧑‍💻 Разработка

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

Предпросмотр Mini App: `/app?preview=1`

## 📄 Лицензия

Проект распространяется по лицензии [GNU AGPL v3](LICENSE): его можно устанавливать, изучать и изменять. При распространении изменённой версии или предоставлении её пользователям через сеть необходимо сохранить эту лицензию и предоставить соответствующий исходный код.
