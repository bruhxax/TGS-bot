#!/usr/bin/env bash
# TGS-bot installer for a fresh Linux VPS.
set -Eeuo pipefail

readonly ENV_FILE=".env"
readonly DEFAULT_REPOSITORY_URL="https://github.com/bruhxax/TGS-bot.git"
readonly DEFAULT_INSTALL_DIR="/opt/tgs-bot"

cyan='\033[0;36m'
green='\033[0;32m'
yellow='\033[1;33m'
red='\033[0;31m'
bold='\033[1m'
reset='\033[0m'

title() { printf "\n${cyan}${bold}╔══════════════════════════════════════╗\n║           TGS-bot installer          ║\n╚══════════════════════════════════════╝${reset}\n\n"; }
info() { printf "${cyan}›${reset} %s\n" "$*"; }
success() { printf "${green}✓${reset} %s\n" "$*"; }
warn() { printf "${yellow}!${reset} %s\n" "$*"; }
fail() { printf "${red}✗ %s${reset}\n" "$*" >&2; exit 1; }

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Не найдено: $1"
}

ask_required() {
  local label="$1" value
  while :; do
    read -r -p "$label: " value
    value="${value#${value%%[![:space:]]*}}"
    value="${value%${value##*[![:space:]]}}"
    if [[ -n "$value" ]]; then
      REPLY="$value"
      return
    fi
    warn "Это поле обязательно."
  done
}

prepare_project_directory() {
  if [[ -f compose.yaml && -f .env.example ]]; then
    return
  fi

  local repository_url="${TGS_REPOSITORY_URL:-$DEFAULT_REPOSITORY_URL}"
  local install_dir="${TGS_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
  need_command git

  if [[ -f "$install_dir/compose.yaml" && -f "$install_dir/.env.example" ]]; then
    cd "$install_dir"
    return
  fi
  [[ ! -e "$install_dir" ]] || fail "Папка $install_dir уже существует и не похожа на TGS-bot."

  info "Скачиваю исходный код проекта в $install_dir…"
  mkdir -p "$(dirname "$install_dir")"
  git clone --depth 1 "$repository_url" "$install_dir" || fail "Не удалось скачать репозиторий. Для приватного репозитория используй SSH-адрес и TGS_REPOSITORY_URL."
  cd "$install_dir"
}

title

prepare_project_directory
[[ -f compose.yaml && -f .env.example ]] || fail "Запусти скрипт из папки TGS-bot."
[[ ! -e "$ENV_FILE" ]] || fail "Файл .env уже существует. Установщик не перезаписывает работающую конфигурацию."
need_command docker
need_command openssl
docker compose version >/dev/null 2>&1 || fail "Docker Compose недоступен."
docker info >/dev/null 2>&1 || fail "Docker Engine не запущен или у текущего пользователя нет к нему доступа."

info "Введи данные. Ввод токенов виден только в этом терминале и не попадёт в историю команд."
ask_required "Токен Telegram-бота"
TELEGRAM_BOT_TOKEN="$REPLY"

ask_required "Публичный URL Mini App (например, https://dev.example.com)"
PUBLIC_BASE_URL="${REPLY%/}"
[[ "$PUBLIC_BASE_URL" =~ ^https://[^/[:space:]]+(/.*)?$ ]] || fail "Нужен HTTPS URL без пробелов."
PUBLIC_HOST="${PUBLIC_BASE_URL#https://}"
PUBLIC_HOST="${PUBLIC_HOST%%/*}"

ask_required "Telegram ID администратора (несколько — через запятую)"
ADMIN_TELEGRAM_IDS="$REPLY"
[[ "$ADMIN_TELEGRAM_IDS" =~ ^[0-9]+(,[0-9]+)*$ ]] || fail "Используй только числовые Telegram ID через запятую."

ask_required "URL панели Remnawave (например, https://panel.example.com)"
REMNAWAVE_URL="${REPLY%/}"
[[ "$REMNAWAVE_URL" =~ ^https?://[^/[:space:]]+(/.*)?$ ]] || fail "Нужен URL панели без пробелов."

ask_required "API Token Remnawave"
REMNAWAVE_TOKEN="$REPLY"

APP_SECRET="$(openssl rand -hex 32)"
POSTGRES_PASSWORD="$(openssl rand -hex 24)"

umask 077
{
  printf 'COMPOSE_PROJECT_NAME=tgs-bot\n'
  printf 'TELEGRAM_BOT_TOKEN=%s\n' "$TELEGRAM_BOT_TOKEN"
  printf 'PUBLIC_BASE_URL=%s\n' "$PUBLIC_BASE_URL"
  printf 'PUBLIC_HOST=%s\n' "$PUBLIC_HOST"
  printf 'ADMIN_TELEGRAM_IDS=%s\n' "$ADMIN_TELEGRAM_IDS"
  printf 'APP_SECRET=%s\n' "$APP_SECRET"
  printf 'POSTGRES_DB=tgsbot\n'
  printf 'POSTGRES_USER=tgsbot\n'
  printf 'POSTGRES_PASSWORD=%s\n' "$POSTGRES_PASSWORD"
  printf 'DATABASE_URL=postgresql://tgsbot:%s@db:5432/tgsbot?sslmode=disable\n' "$POSTGRES_PASSWORD"
  printf 'REMNAWAVE_URL=%s\n' "$REMNAWAVE_URL"
  printf 'REMNAWAVE_TOKEN=%s\n' "$REMNAWAVE_TOKEN"
  printf 'LOG_LEVEL=INFO\n'
} > "$ENV_FILE"
chmod 600 "$ENV_FILE"
success "Создан защищённый .env; секреты базы и приложения сгенерированы автоматически."

info "Собираю и запускаю контейнеры. Это может занять несколько минут при первом запуске."
docker compose up -d --build

info "Жду инициализацию базы данных…"
ready=false
for _ in $(seq 1 45); do
  if docker compose exec -T db psql -U tgsbot -d tgsbot -tAc "SELECT 1 FROM runtime_settings WHERE key='integrations'" 2>/dev/null | grep -qx '1'; then
    ready=true
    break
  fi
  sleep 2
done
[[ "$ready" == true ]] || {
  docker compose logs --tail=100 app db >&2 || true
  fail "База или приложение не успели запуститься. Логи показаны выше."
}

# Remnawave is a runtime setting. Activate the values supplied above now.
docker compose exec -T db \
  psql -v ON_ERROR_STOP=1 -U tgsbot -d tgsbot \
  -v "remnawave_url=$REMNAWAVE_URL" \
  -v "remnawave_token=$REMNAWAVE_TOKEN" <<'SQL'
UPDATE runtime_settings
SET value = jsonb_set(
              jsonb_set(
                jsonb_set(value, '{remnawave,enabled}', 'true'::jsonb, true),
                '{remnawave,url}', to_jsonb(:'remnawave_url'::text), true
              ),
              '{remnawave,token}', to_jsonb(:'remnawave_token'::text), true
            ),
    updated_at = NOW()
WHERE key = 'integrations';
SQL

docker compose exec -T app wget -q -O /dev/null http://127.0.0.1:8080/health || {
  docker compose logs --tail=100 app >&2 || true
  fail "Приложение не прошло локальную проверку здоровья."
}

success "TGS-bot запущен."
printf "\n${bold}Адрес Mini App:${reset} %s/app\n" "$PUBLIC_BASE_URL"
printf "${bold}Webhook Remnawave:${reset} %s/api/webhooks/remnawave\n\n" "$PUBLIC_BASE_URL"
info "Проверь, что DNS домена уже указывает на VPS и порты 80/443 доступны."
info "Открой бота в Telegram командой /start — твой Telegram ID станет администратором автоматически."
