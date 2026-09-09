#!/usr/bin/env bash
# Interactive installer for a Linux VPS.
set -Eeuo pipefail

readonly ENV_FILE=".env"
readonly DEFAULT_REPOSITORY_URL="https://github.com/bruhxax/TGS-bot.git"
readonly DEFAULT_INSTALL_DIR="/opt/tgs-bot"

cyan='\033[0;36m'
green='\033[0;32m'
yellow='\033[1;33m'
red='\033[0;31m'
purple='\033[0;35m'
bold='\033[1m'
dim='\033[2m'
reset='\033[0m'

title() {
  printf "\n${purple}${bold}╔══════════════════════════════════════════════════╗\n"
  printf "║                    ✦  TGS-bot  ✦                  ║\n"
  printf "║            VPN service deployment wizard           ║\n"
  printf "╚══════════════════════════════════════════════════╝${reset}\n"
  printf "${dim}  Bot · Mini App · Remnawave · PostgreSQL${reset}\n\n"
}
step() { printf "\n${cyan}${bold}━━━ %s ━━━${reset}\n" "$*"; }
info() { printf "  ${cyan}›${reset} %s\n" "$*"; }
success() { printf "  ${green}✓${reset} %s\n" "$*"; }
warn() { printf "  ${yellow}!${reset} %s\n" "$*"; }
fail() { printf "  ${red}✗ %s${reset}\n" "$*" >&2; exit 1; }

need_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Не найдено: $1"
}

ask_required() {
  local label="$1" value
  while :; do
    read -r -p "  $label: " value
    value="${value#${value%%[![:space:]]*}}"
    value="${value%${value##*[![:space:]]}}"
    if [[ -n "$value" ]]; then
      REPLY="$value"
      return
    fi
    warn "Это поле обязательно."
  done
}

ask_yes_no() {
  local label="$1" default="${2:-y}" answer
  while :; do
    read -r -p "  $label " answer
    answer="${answer:-$default}"
    case "${answer,,}" in
      y|yes|д|да) REPLY=true; return ;;
      n|no|н|нет) REPLY=false; return ;;
      *) warn "Ответь y или n." ;;
    esac
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

  info "Скачиваю проект в $install_dir…"
  mkdir -p "$(dirname "$install_dir")"
  git clone --depth 1 "$repository_url" "$install_dir" || fail "Не удалось скачать репозиторий. Для приватного GitHub используй SSH-адрес и TGS_REPOSITORY_URL."
  cd "$install_dir"
}

is_system_service_active() {
  command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet "$1" 2>/dev/null
}

is_docker_proxy_running() {
  docker ps --format '{{.Names}} {{.Image}}' 2>/dev/null | grep -Eiq "$1"
}

detect_reverse_proxy() {
  DETECTED_PROXY=""
  DETECTED_DETAIL=""

  if is_system_service_active nginx || pgrep -x nginx >/dev/null 2>&1; then
    DETECTED_PROXY="nginx"
    DETECTED_DETAIL="nginx уже запущен на сервере"
  elif is_system_service_active caddy || pgrep -x caddy >/dev/null 2>&1; then
    DETECTED_PROXY="caddy"
    DETECTED_DETAIL="Caddy уже запущен на сервере"
  elif is_system_service_active apache2 || is_system_service_active httpd || pgrep -x apache2 >/dev/null 2>&1 || pgrep -x httpd >/dev/null 2>&1; then
    DETECTED_PROXY="apache"
    DETECTED_DETAIL="Apache уже запущен на сервере"
  elif is_docker_proxy_running 'traefik'; then
    DETECTED_PROXY="traefik"
    DETECTED_DETAIL="Traefik запущен в Docker"
  elif is_docker_proxy_running 'nginx'; then
    DETECTED_PROXY="nginx-docker"
    DETECTED_DETAIL="nginx запущен в Docker"
  elif is_docker_proxy_running 'caddy'; then
    DETECTED_PROXY="caddy-docker"
    DETECTED_DETAIL="Caddy запущен в Docker"
  fi

  if command -v ss >/dev/null 2>&1; then
    PORT_LISTENERS="$(ss -ltnp '( sport = :80 or sport = :443 )' 2>/dev/null | sed 1d || true)"
  else
    PORT_LISTENERS=""
  fi
  if [[ -z "$DETECTED_PROXY" && -n "$PORT_LISTENERS" ]]; then
    DETECTED_PROXY="unknown"
    DETECTED_DETAIL="порты 80/443 заняты неизвестным сервисом"
  fi
}

choose_proxy_mode() {
  step "Проверка веб-сервера"
  detect_reverse_proxy
  if [[ -z "$DETECTED_PROXY" ]]; then
    success "Порты 80/443 свободны — будет использован встроенный Caddy."
    PROXY_MODE="bundled-caddy"
    return
  fi

  warn "Найден: $DETECTED_DETAIL."
  [[ -n "$PORT_LISTENERS" ]] && printf "${dim}%s${reset}\n" "$PORT_LISTENERS"

  case "$DETECTED_PROXY" in
    nginx)
      ask_yes_no "Подключить TGS-bot к этому nginx? [Y/n]" y
      [[ "$REPLY" == true ]] || fail "Установка остановлена: не будем менять существующий веб-сервер без подтверждения."
      PROXY_MODE="nginx"
      ;;
    caddy)
      ask_yes_no "Подключить TGS-bot к этому Caddy? [Y/n]" y
      [[ "$REPLY" == true ]] || fail "Установка остановлена: не будем менять существующий веб-сервер без подтверждения."
      PROXY_MODE="caddy"
      ;;
    apache|traefik|nginx-docker|caddy-docker|unknown)
      warn "Для $DETECTED_PROXY установщик не меняет конфиг автоматически: структура таких установок различается."
      info "TGS-bot будет открыт только на 127.0.0.1:8080; готовый upstream покажем в конце."
      PROXY_MODE="manual"
      ;;
  esac
}

write_env() {
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
    printf 'POSTGRES_DB=tgsbot\nPOSTGRES_USER=tgsbot\n'
    printf 'POSTGRES_PASSWORD=%s\n' "$POSTGRES_PASSWORD"
    printf 'DATABASE_URL=postgresql://tgsbot:%s@db:5432/tgsbot?sslmode=disable\n' "$POSTGRES_PASSWORD"
    printf 'REMNAWAVE_URL=%s\nREMNAWAVE_TOKEN=%s\nLOG_LEVEL=INFO\n' "$REMNAWAVE_URL" "$REMNAWAVE_TOKEN"
  } > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
  success "Создан защищённый .env; секреты базы и приложения сгенерированы."
}

configure_compose() {
  COMPOSE_ARGS=(-f compose.yaml)
  COMPOSE_SERVICES=()
  if [[ "$PROXY_MODE" != "bundled-caddy" ]]; then
    COMPOSE_ARGS+=(-f compose.proxy.yaml)
    COMPOSE_SERVICES=(app db)
  fi
}

compose() { docker compose "${COMPOSE_ARGS[@]}" "$@"; }

wait_for_application() {
  info "Жду инициализацию базы данных…"
  local ready=false
  for _ in $(seq 1 45); do
    if compose exec -T db psql -U tgsbot -d tgsbot -tAc "SELECT 1 FROM runtime_settings WHERE key='integrations'" 2>/dev/null | grep -qx '1'; then
      ready=true
      break
    fi
    sleep 2
  done
  [[ "$ready" == true ]] || {
    compose logs --tail=100 app db >&2 || true
    fail "База или приложение не успели запуститься. Логи показаны выше."
  }
}

enable_remnawave() {
  compose exec -T db psql -v ON_ERROR_STOP=1 -U tgsbot -d tgsbot \
    -v "remnawave_url=$REMNAWAVE_URL" -v "remnawave_token=$REMNAWAVE_TOKEN" <<'SQL'
UPDATE runtime_settings
SET value = jsonb_set(
              jsonb_set(
                jsonb_set(value, '{remnawave,enabled}', 'true'::jsonb, true),
                '{remnawave,url}', to_jsonb(:'remnawave_url'::text), true
              ),
              '{remnawave,token}', to_jsonb(:'remnawave_token'::text), true
            ), updated_at = NOW()
WHERE key = 'integrations';
SQL
}

configure_nginx() {
  [[ "$EUID" -eq 0 ]] || fail "Для настройки системного nginx запусти установщик от root."
  need_command nginx
  local site="/etc/nginx/sites-available/tgs-bot"
  [[ ! -e "$site" && ! -L "$site" && ! -e /etc/nginx/sites-enabled/tgs-bot && ! -L /etc/nginx/sites-enabled/tgs-bot ]] || fail "Сайт nginx для TGS-bot уже существует. Конфиг не перезаписан."
  install -m 644 nginx.tgs-bot.http.conf.example "$site"
  sed -i "s/__PUBLIC_HOST__/$PUBLIC_HOST/g" "$site"
  ln -sfn "$site" /etc/nginx/sites-enabled/tgs-bot
  nginx -t || fail "Конфигурация nginx не прошла проверку; старый nginx не перезапускался."
  systemctl reload nginx
  success "nginx направляет $PUBLIC_HOST на 127.0.0.1:8080."

  if ! command -v certbot >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
      ask_yes_no "Certbot не найден. Установить его для HTTPS? [Y/n]" y
      [[ "$REPLY" == true ]] || { warn "Без HTTPS Telegram Mini App не откроется."; return; }
      apt-get update
      apt-get install -y certbot python3-certbot-nginx
    else
      warn "Certbot не найден. Выпусти HTTPS-сертификат вручную, затем перезапусти nginx."
      return
    fi
  fi
  ask_required "Email для уведомлений Let's Encrypt"
  certbot --nginx --non-interactive --agree-tos --email "$REPLY" --redirect -d "$PUBLIC_HOST" || fail "Certbot не смог выпустить сертификат. Проверь DNS домена и доступность порта 80."
  success "HTTPS-сертификат для $PUBLIC_HOST выпущен."
}

configure_caddy() {
  [[ "$EUID" -eq 0 ]] || fail "Для настройки системного Caddy запусти установщик от root."
  need_command caddy
  [[ -f /etc/caddy/Caddyfile ]] || fail "Не найден /etc/caddy/Caddyfile. Конфиг Caddy не изменён."
  install -d -m 755 /etc/caddy/conf.d
  [[ ! -e /etc/caddy/conf.d/tgs-bot.caddy && ! -L /etc/caddy/conf.d/tgs-bot.caddy ]] || fail "Конфиг Caddy для TGS-bot уже существует. Он не перезаписан."
  install -m 644 caddy.tgs-bot.caddy.example /etc/caddy/conf.d/tgs-bot.caddy
  sed -i "s/__PUBLIC_HOST__/$PUBLIC_HOST/g" /etc/caddy/conf.d/tgs-bot.caddy
  if ! grep -qF 'import /etc/caddy/conf.d/*.caddy' /etc/caddy/Caddyfile; then
    cp -a /etc/caddy/Caddyfile "/etc/caddy/Caddyfile.tgs-bot.backup.$(date +%Y%m%d%H%M%S)"
    printf '\nimport /etc/caddy/conf.d/*.caddy\n' >> /etc/caddy/Caddyfile
  fi
  caddy validate --config /etc/caddy/Caddyfile || fail "Конфигурация Caddy не прошла проверку; сервис не перезапускался."
  systemctl reload caddy
  success "Caddy направляет $PUBLIC_HOST на 127.0.0.1:8080 и сам выдаст HTTPS."
}

title
step "Подготовка"
prepare_project_directory
[[ -f compose.yaml && -f .env.example ]] || fail "Не найдены файлы проекта."
[[ ! -e "$ENV_FILE" ]] || fail "Файл .env уже существует. Установщик не перезаписывает работающую конфигурацию."
need_command docker
need_command openssl
docker compose version >/dev/null 2>&1 || fail "Docker Compose недоступен."
docker info >/dev/null 2>&1 || fail "Docker Engine не запущен или у текущего пользователя нет к нему доступа."
success "Docker Engine и Docker Compose доступны."

step "Данные проекта"
info "Токены видны во время ввода, но не сохраняются в истории команд."
ask_required "Токен Telegram-бота"; TELEGRAM_BOT_TOKEN="$REPLY"
ask_required "Публичный URL Mini App (например, https://app.example.com)"; PUBLIC_BASE_URL="${REPLY%/}"
[[ "$PUBLIC_BASE_URL" =~ ^https://[^/[:space:]]+(/.*)?$ ]] || fail "Нужен HTTPS URL без пробелов."
PUBLIC_HOST="${PUBLIC_BASE_URL#https://}"; PUBLIC_HOST="${PUBLIC_HOST%%/*}"
ask_required "Telegram ID администратора (несколько — через запятую)"; ADMIN_TELEGRAM_IDS="$REPLY"
[[ "$ADMIN_TELEGRAM_IDS" =~ ^[0-9]+(,[0-9]+)*$ ]] || fail "Используй только числовые Telegram ID через запятую."
ask_required "URL панели Remnawave"; REMNAWAVE_URL="${REPLY%/}"
[[ "$REMNAWAVE_URL" =~ ^https?://[^/[:space:]]+(/.*)?$ ]] || fail "Нужен URL панели без пробелов."
ask_required "API Token Remnawave"; REMNAWAVE_TOKEN="$REPLY"

choose_proxy_mode
configure_compose
write_env

step "Запуск контейнеров"
info "Собираю TGS-bot. Первый запуск может занять несколько минут."
compose up -d --build "${COMPOSE_SERVICES[@]}"
wait_for_application
enable_remnawave
compose exec -T app wget -q -O /dev/null http://127.0.0.1:8080/health || fail "Приложение не прошло локальную проверку здоровья."
success "Приложение и база данных работают."

step "Публичный доступ"
case "$PROXY_MODE" in
  bundled-caddy) success "Встроенный Caddy выпустит HTTPS для $PUBLIC_HOST." ;;
  nginx) configure_nginx ;;
  caddy) configure_caddy ;;
  manual)
    warn "Добавь в существующий reverse proxy upstream: http://127.0.0.1:8080"
    warn "Без HTTPS Telegram Mini App не будет работать."
    ;;
esac

printf "\n${green}${bold}╔══════════════════════════════════════════════════╗\n"
printf "║                  Установка готова                 ║\n"
printf "╚══════════════════════════════════════════════════╝${reset}\n"
printf "\n${bold}Mini App:${reset}  %s/app\n" "$PUBLIC_BASE_URL"
printf "${bold}Webhook:${reset}   %s/api/webhooks/remnawave\n\n" "$PUBLIC_BASE_URL"
info "Открой бота в Telegram командой /start — указанный ID станет администратором автоматически."
