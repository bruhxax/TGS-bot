package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct{ DB *sql.DB }

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
 id BIGSERIAL PRIMARY KEY, telegram_id BIGINT UNIQUE NOT NULL, username VARCHAR(64) NOT NULL DEFAULT '',
 first_name VARCHAR(128) NOT NULL DEFAULT 'Пользователь', photo_url TEXT NOT NULL DEFAULT '', language VARCHAR(8) NOT NULL DEFAULT 'ru',
 is_admin BOOLEAN NOT NULL DEFAULT FALSE, is_blocked BOOLEAN NOT NULL DEFAULT FALSE, trial_used BOOLEAN NOT NULL DEFAULT FALSE,
 referral_rewarded BOOLEAN NOT NULL DEFAULT FALSE, referral_code VARCHAR(24) UNIQUE NOT NULL, referred_by_id BIGINT REFERENCES users(id),
 remnawave_user_id BIGINT, remnawave_user_uuid VARCHAR(64) NOT NULL DEFAULT '', remnawave_username VARCHAR(64) NOT NULL DEFAULT '', subscription_url TEXT NOT NULL DEFAULT '',
 subscription_status VARCHAR(20) NOT NULL DEFAULT 'INACTIVE', expires_at TIMESTAMPTZ, traffic_limit_bytes BIGINT NOT NULL DEFAULT 0,
 traffic_used_bytes BIGINT NOT NULL DEFAULT 0, device_limit INT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS users_telegram_id_idx ON users(telegram_id);
CREATE INDEX IF NOT EXISTS users_referral_code_idx ON users(referral_code);
ALTER TABLE users ADD COLUMN IF NOT EXISTS remnawave_username VARCHAR(64) NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS runtime_settings (
 key VARCHAR(64) PRIMARY KEY, value JSONB NOT NULL DEFAULT '{}'::jsonb, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS tariffs (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name VARCHAR(96) NOT NULL, description VARCHAR(240) NOT NULL DEFAULT '',
 price_kopecks BIGINT NOT NULL, days INT NOT NULL, traffic_gb INT NOT NULL DEFAULT 0, device_limit INT NOT NULL DEFAULT 1,
 internal_squads JSONB NOT NULL DEFAULT '[]'::jsonb, external_squad_uuid VARCHAR(64) NOT NULL DEFAULT '',
 active BOOLEAN NOT NULL DEFAULT TRUE, pinned BOOLEAN NOT NULL DEFAULT FALSE, position INT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS payments (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id BIGINT NOT NULL REFERENCES users(id), tariff_id UUID REFERENCES tariffs(id),
 provider VARCHAR(32) NOT NULL, provider_payment_id VARCHAR(128) NOT NULL DEFAULT '', status VARCHAR(24) NOT NULL DEFAULT 'pending',
 amount_kopecks BIGINT NOT NULL, promo_code VARCHAR(64) NOT NULL DEFAULT '', snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), paid_at TIMESTAMPTZ
);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS merchant_order_id BIGSERIAL;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS entitlement JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS processing_started_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS payments_user_idx ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS payments_provider_idx ON payments(provider_payment_id);
CREATE UNIQUE INDEX IF NOT EXISTS payments_merchant_order_idx ON payments(merchant_order_id);
CREATE TABLE IF NOT EXISTS promo_codes (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), code VARCHAR(64) UNIQUE NOT NULL, discount_percent INT NOT NULL DEFAULT 0,
 max_uses INT NOT NULL DEFAULT 0, uses INT NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT TRUE,
 expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS tickets (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), user_id BIGINT NOT NULL REFERENCES users(id), subject VARCHAR(140) NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'open', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS tickets_user_idx ON tickets(user_id, updated_at DESC);
CREATE TABLE IF NOT EXISTS ticket_messages (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
 sender_user_id BIGINT REFERENCES users(id), is_admin BOOLEAN NOT NULL DEFAULT FALSE, text TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS diagnostics (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), source VARCHAR(32) NOT NULL, level VARCHAR(16) NOT NULL DEFAULT 'error',
 message TEXT NOT NULL, details JSONB NOT NULL DEFAULT '{}'::jsonb, resolved BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS admin_audit (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), admin_user_id BIGINT NOT NULL REFERENCES users(id),
 target_user_id BIGINT REFERENCES users(id), action VARCHAR(64) NOT NULL, reason TEXT NOT NULL DEFAULT '',
 details JSONB NOT NULL DEFAULT '{}'::jsonb, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS subscription_grace_delivery (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, source_expire_at TIMESTAMPTZ NOT NULL,
 grace_expire_at TIMESTAMPTZ, applied_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(user_id,source_expire_at)
);
CREATE INDEX IF NOT EXISTS subscription_grace_expire_idx ON subscription_grace_delivery(user_id,grace_expire_at) WHERE grace_expire_at IS NOT NULL;
CREATE TABLE IF NOT EXISTS broadcasts (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), admin_user_id BIGINT NOT NULL REFERENCES users(id), text TEXT NOT NULL,
 buttons JSONB NOT NULL DEFAULT '[]'::jsonb, status VARCHAR(20) NOT NULL DEFAULT 'draft', sent_count INT NOT NULL DEFAULT 0,
 failed_count INT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS source_chat_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS source_message_id INT NOT NULL DEFAULT 0;
ALTER TABLE broadcasts ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
CREATE TABLE IF NOT EXISTS broadcast_deliveries (
 broadcast_id UUID NOT NULL REFERENCES broadcasts(id) ON DELETE CASCADE, chat_id BIGINT NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'pending', attempts INT NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(broadcast_id,chat_id)
);`
	_, err := s.DB.ExecContext(ctx, schema)
	return err
}

var DefaultSettings = map[string]map[string]any{
	"content": {
		"brand": "TGS VPN", "logo_url": "", "connect_button": "Подключиться", "renew_button": "Продлить", "support_welcome": "Опишите вопрос — поддержка ответит в этом чате.",
		"no_subscription_title": "Подписки нету", "subscription_expired_title": "Подписка закончилась", "buy_subscription_button": "Купить подписку", "traffic_used_label": "Использовано",
		"support_create_button": "Создать тикет", "support_tickets_title": "Ваши тикеты", "support_admin_title": "Все тикеты", "support_admin_hint": "Новые сообщения появляются автоматически", "support_empty": "Открытых тикетов нет",
		"connect_title": "Установка", "connect_hint": "Выберите устройство и приложение — мы проведём вас через три коротких шага.", "connect_recommended": "Рекомендуем", "connect_open_button": "Добавить подписку", "connect_install_button": "Скачать", "connect_copy_button": "Скопировать ссылку", "connect_empty": "Для этого устройства клиенты пока не добавлены",
		"connect_install_title": "Установите приложение", "connect_install_hint": "Скачайте клиент для своего устройства.", "connect_add_title": "Добавьте подписку", "connect_add_hint": "Браузер предложит открыть выбранное приложение.", "connect_use_title": "Подключитесь", "connect_use_hint": "Выберите сервер в приложении и включите подключение.", "connect_qr_overline": "Другой способ", "connect_qr_title": "Перенести на другое устройство", "connect_qr_hint": "Отсканируйте QR-код камерой или сканером внутри VPN-клиента.",
		"start_title": "Добро пожаловать в TGS VPN", "start_text": "Управляйте подпиской, подключением и поддержкой в одном приложении.", "start_message": "",
		"trial_button": "Бесплатный период", "trial_button_style": "success", "trial_button_emoji_id": "",
		"cabinet_button": "Личный кабинет", "cabinet_button_style": "primary", "cabinet_button_emoji_id": "",
		"support_button": "Поддержка", "support_button_style": "", "support_button_emoji_id": "",
		"emergency_message":             "Сервис временно недоступен. Мы уже работаем над восстановлением.",
		"myid_message":                  "Ваш Telegram ID: <code>{id}</code>",
		"payment_success_message":       "<b>Оплата прошла</b>\nПодписка обновлена.",
		"support_reply_message":         "Поддержка ответила в тикете «<b>{subject}</b>». Откройте Mini App.",
		"full_block_message":            "Доступ к кабинету и VPN временно заблокирован администратором.",
		"admin_only_message":            "Команда доступна только администратору.",
		"broadcast_prompt_message":      "Отправьте следующим сообщением материал для рассылки. Фото, форматирование и премиум-эмодзи сохранятся.",
		"broadcast_draft_error_message": "Не удалось создать черновик.", "broadcast_save_error_message": "Не удалось сохранить сообщение. Попробуйте ещё раз.",
		"broadcast_preview_error_message":  "Сообщение сохранено, но предпросмотр не создался. Нажмите «Изменить» в Mini App и повторите.",
		"broadcast_edit_message":           "Отправьте новый вариант одним сообщением.",
		"broadcast_confirmed_message":      "Готово. Теперь добавьте кнопки, выполните тест и запустите рассылку в Mini App.",
		"broadcast_start_error_message":    "Рассылка не запущена: не удалось получить пользователей.",
		"broadcast_complete_message":       "Рассылка завершена. Доставлено: <b>{sent}</b>, ошибок: <b>{failed}</b>.",
		"trial_admin_message":              "<b>Активирован пробный период</b>\n{name} · <code>{id}</code>",
		"new_ticket_admin_message":         "<b>Новый тикет</b>\n{subject}\n{name} · <code>{id}</code>",
		"ticket_message_admin_message":     "Новое сообщение в тикете «<b>{subject}</b>».",
		"payment_admin_message":            "<b>Новая оплата</b>\n{name} · {amount} ₽ · {provider}",
		"grace_access_message":             "<b>Временный доступ активирован</b>\n\nОсновная подписка закончилась. Доступ сохранён ещё на <b>{days}</b> дн.\n\nПродлите подписку, чтобы вернуть полный доступ.",
		"subscription_rebound_old_message": "Подписка перенесена на другой Telegram-аккаунт администратором.",
		"subscription_rebound_new_message": "<b>Подписка привязана</b>\nТеперь она доступна в вашем Telegram-аккаунте.",
	},
	"features": {"trial": true, "server_status": true, "devices": true, "payments": true, "referrals": true, "promo_codes": true, "support": true},
	"trial":    {"enabled": true, "days": float64(3), "traffic_gb": float64(10), "device_limit": float64(1), "internal_squads": []any{}, "external_squad_uuid": ""},
	"integrations": {
		"remnawave":     map[string]any{"enabled": false, "url": "", "token": "", "webhook_secret": ""},
		"yookassa":      map[string]any{"enabled": false, "shop_id": "", "secret_key": "", "email": ""},
		"cryptobot":     map[string]any{"enabled": false, "token": "", "testnet": false},
		"lava":          map[string]any{"enabled": false, "shop_id": "", "secret_key": "", "additional_key": ""},
		"wata":          map[string]any{"enabled": false, "access_token": "", "api_url": "https://api.wata.pro/api/h2h"},
		"platega":       map[string]any{"enabled": false, "merchant_id": "", "secret_key": "", "api_url": "https://app.platega.io"},
		"freekassa":     map[string]any{"enabled": false, "shop_id": "", "secret_word": "", "secret_word2": ""},
		"heleket":       map[string]any{"enabled": false, "merchant_id": "", "api_key": "", "api_url": "https://api.heleket.com"},
		"pally":         map[string]any{"enabled": false, "shop_id": "", "api_token": "", "api_url": "https://pal24.pro"},
		"notifications": map[string]any{"enabled": false, "bot_token": "", "chat_id": ""},
	},
	"theme":      {"template": "telegram", "accent": "#2aabee", "background": "#111315", "surface": "#1c1f22", "surface_alt": "#24282d", "text": "#ffffff", "muted": "#8f969e"},
	"system":     {"referral_days": float64(7), "referral_traffic_gb": float64(10), "reward_after_payment": true},
	"emergency":  {"enabled": false, "message": "Сервис временно недоступен. Мы уже работаем над восстановлением."},
	"language":   {"default": "ru"},
	"more_order": {"items": []any{"servers", "devices", "payments", "referral"}},
	"grace":      {"enabled": false, "days": float64(2), "internal_squads": []any{}},
	"subpage":    {"include_builtins": true, "clients": []any{}},
}

var ThemeTemplates = map[string]map[string]any{
	"telegram": {"accent": "#2aabee", "background": "#111315", "surface": "#1c1f22", "surface_alt": "#24282d"},
	"graphite": {"accent": "#ffffff", "background": "#0c0c0d", "surface": "#19191b", "surface_alt": "#232326"},
	"emerald":  {"accent": "#2fbf8f", "background": "#101413", "surface": "#1a211f", "surface_alt": "#222c29"},
	"sand":     {"accent": "#e7b55e", "background": "#14120f", "surface": "#211e19", "surface_alt": "#2b271f"},
	"ocean":    {"accent": "#5aa9e6", "background": "#0d1217", "surface": "#182028", "surface_alt": "#222d36"},
	"violet":   {"accent": "#9b8cff", "background": "#111016", "surface": "#1d1a25", "surface_alt": "#282333"},
	"ruby":     {"accent": "#e06b78", "background": "#151012", "surface": "#22191c", "surface_alt": "#2e2226"},
	"steel":    {"accent": "#8ea2b5", "background": "#101214", "surface": "#1a1e22", "surface_alt": "#242a30"},
}

func (s *Store) seed(ctx context.Context) error {
	for key, value := range DefaultSettings {
		raw, _ := json.Marshal(value)
		if _, err := s.DB.ExecContext(ctx, `INSERT INTO runtime_settings(key,value) VALUES($1,$2) ON CONFLICT(key) DO NOTHING`, key, raw); err != nil {
			return err
		}
	}
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM tariffs`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		_, err := s.DB.ExecContext(ctx, `INSERT INTO tariffs(name,description,price_kopecks,days,traffic_gb,device_limit,pinned,position) VALUES
 ('Старт','Для одного устройства',19900,30,100,1,FALSE,1),
 ('Оптимальный','Три месяца без забот',49900,90,300,3,TRUE,2),
 ('Годовой','Максимальная выгода',149000,365,0,5,FALSE,3)`)
		return err
	}
	return nil
}

func merge(base, update map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range update {
		if old, ok := out[k].(map[string]any); ok {
			if next, ok := v.(map[string]any); ok {
				out[k] = merge(old, next)
				continue
			}
		}
		if text, ok := v.(string); ok && text == "••••••••" {
			continue
		}
		out[k] = v
	}
	return out
}

func (s *Store) Setting(ctx context.Context, key string) (map[string]any, error) {
	var raw []byte
	if err := s.DB.QueryRowContext(ctx, `SELECT value FROM runtime_settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return nil, err
	}
	current := map[string]any{}
	if err := json.Unmarshal(raw, &current); err != nil {
		return nil, err
	}
	if defaults, ok := DefaultSettings[key]; ok {
		current = merge(defaults, current)
	}
	return current, nil
}

func (s *Store) SaveSetting(ctx context.Context, key string, update map[string]any) (map[string]any, error) {
	current, err := s.Setting(ctx, key)
	if err != nil {
		return nil, err
	}
	current = merge(current, update)
	raw, _ := json.Marshal(current)
	_, err = s.DB.ExecContext(ctx, `UPDATE runtime_settings SET value=$2,updated_at=NOW() WHERE key=$1`, key, raw)
	return current, err
}

func scanUser(scanner interface{ Scan(...any) error }) (User, error) {
	var u User
	err := scanner.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.PhotoURL, &u.Language, &u.IsAdmin, &u.IsBlocked, &u.TrialUsed, &u.ReferralRewarded, &u.ReferralCode, &u.ReferredByID, &u.RemnawaveUserID, &u.RemnawaveUserUUID, &u.RemnawaveUsername, &u.SubscriptionURL, &u.SubscriptionStatus, &u.ExpiresAt, &u.TrafficLimitBytes, &u.TrafficUsedBytes, &u.DeviceLimit, &u.CreatedAt, &u.LastSeenAt)
	return u, err
}

const userColumns = `id,telegram_id,username,first_name,photo_url,language,is_admin,is_blocked,trial_used,referral_rewarded,referral_code,referred_by_id,remnawave_user_id,remnawave_user_uuid,remnawave_username,subscription_url,subscription_status,expires_at,traffic_limit_bytes,traffic_used_bytes,device_limit,created_at,last_seen_at`

func (s *Store) UserByTelegram(ctx context.Context, id int64) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE telegram_id=$1`, id))
}
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.DB.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id))
}

func (s *Store) UpsertTelegramUser(ctx context.Context, t TelegramUser, admins map[int64]bool, referral string) (User, error) {
	var referredBy *int64
	if referral != "" {
		var id int64
		if err := s.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE referral_code=$1 AND telegram_id<>$2`, referral, t.ID).Scan(&id); err == nil {
			referredBy = &id
		}
	}
	lang := t.LanguageCode
	if lang == "" {
		lang = "ru"
	}
	if t.FirstName == "" {
		t.FirstName = "Пользователь"
	}
	refCode := fmt.Sprintf("tgs%x", t.ID)
	isAdmin := admins[t.ID]
	q := `INSERT INTO users(telegram_id,username,first_name,photo_url,language,is_admin,referral_code,referred_by_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT(telegram_id) DO UPDATE SET username=EXCLUDED.username,first_name=EXCLUDED.first_name,
 photo_url=CASE WHEN EXCLUDED.photo_url<>'' THEN EXCLUDED.photo_url ELSE users.photo_url END,
 language=EXCLUDED.language,is_admin=users.is_admin OR EXCLUDED.is_admin,last_seen_at=NOW()
 RETURNING ` + userColumns
	return scanUser(s.DB.QueryRowContext(ctx, q, t.ID, t.Username, t.FirstName, t.PhotoURL, lang, isAdmin, refCode, referredBy))
}

func (s *Store) UpdateSubscription(ctx context.Context, u User) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET remnawave_user_id=$2,remnawave_user_uuid=$3,remnawave_username=$4,subscription_url=$5,subscription_status=$6,expires_at=$7,traffic_limit_bytes=$8,traffic_used_bytes=$9,device_limit=$10 WHERE id=$1`, u.ID, u.RemnawaveUserID, u.RemnawaveUserUUID, u.RemnawaveUsername, u.SubscriptionURL, u.SubscriptionStatus, u.ExpiresAt, u.TrafficLimitBytes, u.TrafficUsedBytes, u.DeviceLimit)
	return err
}

func (s *Store) SetTrialUsed(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET trial_used=TRUE WHERE id=$1`, id)
	return err
}
func (s *Store) ReserveTrial(ctx context.Context, id int64) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE users SET trial_used=TRUE WHERE id=$1 AND trial_used=FALSE`)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
func (s *Store) ReleaseTrial(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET trial_used=FALSE WHERE id=$1 AND trial_used=TRUE`)
	return err
}
func (s *Store) SetReferralRewarded(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET referral_rewarded=TRUE WHERE id=$1`, id)
	return err
}

func scanTariff(scanner interface{ Scan(...any) error }) (Tariff, error) {
	var t Tariff
	var raw []byte
	err := scanner.Scan(&t.ID, &t.Name, &t.Description, &t.PriceKopecks, &t.Days, &t.TrafficGB, &t.DeviceLimit, &raw, &t.ExternalSquadUUID, &t.Active, &t.Pinned, &t.Position, &t.CreatedAt)
	if err == nil {
		_ = json.Unmarshal(raw, &t.InternalSquads)
	}
	return t, err
}

const tariffColumns = `id::text,name,description,price_kopecks,days,traffic_gb,device_limit,internal_squads,external_squad_uuid,active,pinned,position,created_at`

func (s *Store) Tariffs(ctx context.Context, all bool) ([]Tariff, error) {
	q := `SELECT ` + tariffColumns + ` FROM tariffs`
	if !all {
		q += ` WHERE active=TRUE`
	}
	q += ` ORDER BY pinned DESC,position,created_at`
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tariff{}
	for rows.Next() {
		t, e := scanTariff(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) Tariff(ctx context.Context, id string) (Tariff, error) {
	return scanTariff(s.DB.QueryRowContext(ctx, `SELECT `+tariffColumns+` FROM tariffs WHERE id=$1`, id))
}
func (s *Store) SaveTariff(ctx context.Context, t Tariff) (Tariff, error) {
	raw, _ := json.Marshal(t.InternalSquads)
	if t.ID == "" {
		return scanTariff(s.DB.QueryRowContext(ctx, `INSERT INTO tariffs(name,description,price_kopecks,days,traffic_gb,device_limit,internal_squads,external_squad_uuid,active,pinned,position) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+tariffColumns, t.Name, t.Description, t.PriceKopecks, t.Days, t.TrafficGB, t.DeviceLimit, raw, t.ExternalSquadUUID, t.Active, t.Pinned, t.Position))
	}
	return scanTariff(s.DB.QueryRowContext(ctx, `UPDATE tariffs SET name=$2,description=$3,price_kopecks=$4,days=$5,traffic_gb=$6,device_limit=$7,internal_squads=$8,external_squad_uuid=$9,active=$10,pinned=$11,position=$12 WHERE id=$1 RETURNING `+tariffColumns, t.ID, t.Name, t.Description, t.PriceKopecks, t.Days, t.TrafficGB, t.DeviceLimit, raw, t.ExternalSquadUUID, t.Active, t.Pinned, t.Position))
}
func (s *Store) DeleteTariff(ctx context.Context, id string) error {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments WHERE tariff_id=$1`, id).Scan(&n)
	if n > 0 {
		_, e := s.DB.ExecContext(ctx, `UPDATE tariffs SET active=FALSE WHERE id=$1`, id)
		return e
	}
	_, e := s.DB.ExecContext(ctx, `DELETE FROM tariffs WHERE id=$1`, id)
	return e
}

func scanPayment(scanner interface{ Scan(...any) error }) (Payment, error) {
	var p Payment
	var snapshotRaw, entitlementRaw []byte
	err := scanner.Scan(&p.ID, &p.MerchantOrderID, &p.UserID, &p.TariffID, &p.Provider, &p.ProviderPaymentID, &p.Status, &p.AmountKopecks, &p.PromoCode, &snapshotRaw, &entitlementRaw, &p.CreatedAt, &p.PaidAt)
	if err == nil {
		_ = json.Unmarshal(snapshotRaw, &p.Snapshot)
		_ = json.Unmarshal(entitlementRaw, &p.Entitlement)
	}
	return p, err
}

const paymentColumns = `id::text,merchant_order_id,user_id,COALESCE(tariff_id::text,''),provider,provider_payment_id,status,amount_kopecks,promo_code,snapshot,entitlement,created_at,paid_at`

func (s *Store) CreatePayment(ctx context.Context, p Payment) (Payment, error) {
	raw, _ := json.Marshal(p.Snapshot)
	return scanPayment(s.DB.QueryRowContext(ctx, `INSERT INTO payments(user_id,tariff_id,provider,amount_kopecks,promo_code,snapshot) VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6) RETURNING `+paymentColumns, p.UserID, p.TariffID, p.Provider, p.AmountKopecks, p.PromoCode, raw))
}
func (s *Store) SetPaymentProviderID(ctx context.Context, id, providerID string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE payments SET provider_payment_id=$2 WHERE id=$1`, id, providerID)
	return e
}
func (s *Store) Payment(ctx context.Context, id string) (Payment, error) {
	return scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1`, id))
}
func (s *Store) PaymentByProvider(ctx context.Context, id string) (Payment, error) {
	return scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE provider_payment_id=$1`, id))
}
func (s *Store) PaymentByMerchantOrder(ctx context.Context, id int64) (Payment, error) {
	return scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE merchant_order_id=$1`, id))
}
func (s *Store) PaymentsByUser(ctx context.Context, userID int64) ([]Payment, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		p, x := scanPayment(rows)
		if x != nil {
			return nil, x
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) FailPayment(ctx context.Context, id string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE payments SET status='failed' WHERE id=$1 AND status='pending'`, id)
	return e
}
func (s *Store) MarkPaymentPaid(ctx context.Context, id string) (bool, error) {
	r, e := s.DB.ExecContext(ctx, `UPDATE payments SET status='succeeded',paid_at=NOW() WHERE id=$1 AND status<>'succeeded'`, id)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
func (s *Store) ClaimPayment(ctx context.Context, id string) (bool, error) {
	r, e := s.DB.ExecContext(ctx, `UPDATE payments SET status='processing',processing_started_at=NOW() WHERE id=$1 AND (status='pending' OR (status='processing' AND (processing_started_at IS NULL OR processing_started_at < NOW()-INTERVAL '10 minutes')))`, id)
	if e != nil {
		return false, e
	}
	n, _ := r.RowsAffected()
	return n == 1, nil
}
func (s *Store) CompleteClaimedPayment(ctx context.Context, id string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE payments SET status='succeeded',paid_at=NOW(),processing_started_at=NULL WHERE id=$1 AND status='processing'`, id)
	return e
}
func (s *Store) ReleasePaymentClaim(ctx context.Context, id string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE payments SET status='pending',processing_started_at=NULL WHERE id=$1 AND status='processing'`, id)
	return e
}
func (s *Store) SavePaymentEntitlement(ctx context.Context, id string, value map[string]any) (Payment, error) {
	raw, _ := json.Marshal(value)
	_, err := s.DB.ExecContext(ctx, `UPDATE payments SET entitlement=$2 WHERE id=$1 AND entitlement='{}'::jsonb`, id, raw)
	if err != nil {
		return Payment{}, err
	}
	return s.Payment(ctx, id)
}
func (s *Store) StaleProcessingPayments(ctx context.Context) ([]Payment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE (status='processing' AND (processing_started_at IS NULL OR processing_started_at < NOW()-INTERVAL '10 minutes')) OR (status='pending' AND entitlement<>'{}'::jsonb) ORDER BY created_at LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		payment, scanErr := scanPayment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, payment)
	}
	return out, rows.Err()
}

func (s *Store) WithUserLock(ctx context.Context, userID int64, fn func() error) error {
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, userID); err != nil {
		return err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, userID) }()
	return fn()
}

func (s *Store) ExpiredSubscriptionUsers(ctx context.Context) ([]User, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+userColumns+` FROM users u
		WHERE u.is_blocked=FALSE AND u.expires_at IS NOT NULL AND u.expires_at<=NOW()
		  AND u.subscription_url<>'' AND u.subscription_status IN ('ACTIVE','EXPIRED')
		  AND NOT EXISTS (SELECT 1 FROM subscription_grace_delivery g WHERE g.user_id=u.id AND g.grace_expire_at IS NULL)
		ORDER BY u.expires_at LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, scanErr := scanUser(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) ClaimSubscriptionGrace(ctx context.Context, userID int64, sourceExpireAt time.Time) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `INSERT INTO subscription_grace_delivery(user_id,source_expire_at)
		SELECT $1,$2 WHERE NOT EXISTS (
			SELECT 1 FROM subscription_grace_delivery
			WHERE user_id=$1 AND (grace_expire_at IS NULL OR grace_expire_at BETWEEN $2::timestamptz-INTERVAL '5 seconds' AND $2::timestamptz+INTERVAL '5 seconds')
		) ON CONFLICT(user_id,source_expire_at) DO NOTHING`, userID, sourceExpireAt)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (s *Store) CompleteSubscriptionGrace(ctx context.Context, user User, sourceExpireAt, graceExpireAt time.Time) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE subscription_grace_delivery SET grace_expire_at=$3,applied_at=NOW()
		WHERE user_id=$1 AND source_expire_at=$2 AND grace_expire_at IS NULL`, user.ID, sourceExpireAt, graceExpireAt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return fmt.Errorf("заявка временного доступа не найдена")
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET remnawave_user_id=$2,remnawave_user_uuid=$3,remnawave_username=$4,
		subscription_url=$5,subscription_status=$6,expires_at=$7,traffic_limit_bytes=$8,traffic_used_bytes=$9,device_limit=$10 WHERE id=$1`,
		user.ID, user.RemnawaveUserID, user.RemnawaveUserUUID, user.RemnawaveUsername, user.SubscriptionURL, user.SubscriptionStatus,
		graceExpireAt, user.TrafficLimitBytes, user.TrafficUsedBytes, user.DeviceLimit)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ReleaseSubscriptionGrace(ctx context.Context, userID int64, sourceExpireAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM subscription_grace_delivery WHERE user_id=$1 AND source_expire_at=$2 AND grace_expire_at IS NULL`, userID, sourceExpireAt)
	return err
}

var (
	ErrSubscriptionTransferSame   = errors.New("нельзя перенести подписку на тот же Telegram ID")
	ErrSubscriptionTransferSource = errors.New("у исходного пользователя нет подписки")
	ErrSubscriptionTransferTarget = errors.New("у нового Telegram ID уже есть подписка")
)

func (s *Store) TransferSubscription(ctx context.Context, adminID, sourceTelegramID, targetTelegramID int64, reason string) (User, User, error) {
	if sourceTelegramID == targetTelegramID {
		return User{}, User{}, ErrSubscriptionTransferSame
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return User{}, User{}, err
	}
	defer tx.Rollback()
	source, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE telegram_id=$1 FOR UPDATE`, sourceTelegramID))
	if err != nil {
		return User{}, User{}, err
	}
	target, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE telegram_id=$1 FOR UPDATE`, targetTelegramID))
	if err != nil {
		return User{}, User{}, err
	}
	if source.RemnawaveUserID == nil && source.RemnawaveUserUUID == "" && source.SubscriptionURL == "" {
		return User{}, User{}, ErrSubscriptionTransferSource
	}
	if target.RemnawaveUserID != nil || target.RemnawaveUserUUID != "" || target.SubscriptionURL != "" {
		return User{}, User{}, ErrSubscriptionTransferTarget
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET remnawave_user_id=$2,remnawave_user_uuid=$3,remnawave_username=$4,
		subscription_url=$5,subscription_status=$6,expires_at=$7,traffic_limit_bytes=$8,traffic_used_bytes=$9,device_limit=$10 WHERE id=$1`,
		target.ID, source.RemnawaveUserID, source.RemnawaveUserUUID, source.RemnawaveUsername, source.SubscriptionURL,
		source.SubscriptionStatus, source.ExpiresAt, source.TrafficLimitBytes, source.TrafficUsedBytes, source.DeviceLimit)
	if err != nil {
		return User{}, User{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE users SET remnawave_user_id=NULL,remnawave_user_uuid='',remnawave_username='',subscription_url='',
		subscription_status='INACTIVE',expires_at=NULL,traffic_limit_bytes=0,traffic_used_bytes=0,device_limit=1 WHERE id=$1`, source.ID)
	if err != nil {
		return User{}, User{}, err
	}
	details, _ := json.Marshal(map[string]any{"old_telegram_id": sourceTelegramID, "new_telegram_id": targetTelegramID})
	_, err = tx.ExecContext(ctx, `INSERT INTO admin_audit(admin_user_id,target_user_id,action,reason,details) VALUES($1,$2,'transfer_subscription',$3,$4)`, adminID, target.ID, reason, details)
	if err != nil {
		return User{}, User{}, err
	}
	if err = tx.Commit(); err != nil {
		return User{}, User{}, err
	}
	source, err = s.UserByID(ctx, source.ID)
	if err != nil {
		return User{}, User{}, err
	}
	target, err = s.UserByID(ctx, target.ID)
	return source, target, err
}

func (s *Store) PromoByCode(ctx context.Context, code string) (Promo, error) {
	var p Promo
	err := s.DB.QueryRowContext(ctx, `SELECT id::text,code,discount_percent,max_uses,uses,active,expires_at,created_at FROM promo_codes WHERE code=$1`, strings.ToUpper(strings.TrimSpace(code))).Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.MaxUses, &p.Uses, &p.Active, &p.ExpiresAt, &p.CreatedAt)
	return p, err
}
func (s *Store) Promos(ctx context.Context) ([]Promo, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id::text,code,discount_percent,max_uses,uses,active,expires_at,created_at FROM promo_codes ORDER BY created_at DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Promo{}
	for rows.Next() {
		var p Promo
		if e = rows.Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.MaxUses, &p.Uses, &p.Active, &p.ExpiresAt, &p.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) CreatePromo(ctx context.Context, p Promo) (Promo, error) {
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	e := s.DB.QueryRowContext(ctx, `INSERT INTO promo_codes(code,discount_percent,max_uses,active,expires_at)VALUES($1,$2,$3,$4,$5) RETURNING id::text,created_at`, p.Code, p.DiscountPercent, p.MaxUses, p.Active, p.ExpiresAt).Scan(&p.ID, &p.CreatedAt)
	return p, e
}
func (s *Store) DeletePromo(ctx context.Context, id string) error {
	_, e := s.DB.ExecContext(ctx, `DELETE FROM promo_codes WHERE id=$1`, id)
	return e
}
func (s *Store) IncrementPromo(ctx context.Context, code string) error {
	if code == "" {
		return nil
	}
	_, e := s.DB.ExecContext(ctx, `UPDATE promo_codes SET uses=uses+1 WHERE code=$1`, code)
	return e
}

func (s *Store) CreateTicket(ctx context.Context, userID int64, subject, text string) (Ticket, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Ticket{}, e
	}
	defer tx.Rollback()
	var t Ticket
	e = tx.QueryRowContext(ctx, `INSERT INTO tickets(user_id,subject)VALUES($1,$2) RETURNING id::text,user_id,subject,status,created_at,updated_at`, userID, subject).Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if e != nil {
		return t, e
	}
	var m TicketMessage
	e = tx.QueryRowContext(ctx, `INSERT INTO ticket_messages(ticket_id,sender_user_id,is_admin,text)VALUES($1,$2,FALSE,$3) RETURNING id::text,ticket_id::text,sender_user_id,is_admin,text,created_at`, t.ID, userID, text).Scan(&m.ID, &m.TicketID, &m.SenderUserID, &m.IsAdmin, &m.Text, &m.CreatedAt)
	if e != nil {
		return t, e
	}
	t.Messages = []TicketMessage{m}
	e = tx.Commit()
	return t, e
}
func (s *Store) Tickets(ctx context.Context, userID *int64) ([]Ticket, error) {
	q := `SELECT id::text,user_id,subject,status,created_at,updated_at FROM tickets`
	args := []any{}
	if userID != nil {
		q += ` WHERE user_id=$1`
		args = append(args, *userID)
	}
	q += ` ORDER BY updated_at DESC LIMIT 200`
	rows, e := s.DB.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		var t Ticket
		if e = rows.Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt); e != nil {
			return nil, e
		}
		msgs, x := s.TicketMessages(ctx, t.ID)
		if x != nil {
			return nil, x
		}
		t.Messages = msgs
		if userID == nil {
			u, x := s.UserByID(ctx, t.UserID)
			if x == nil {
				t.User = &u
			}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Store) Ticket(ctx context.Context, id string) (Ticket, error) {
	var t Ticket
	e := s.DB.QueryRowContext(ctx, `SELECT id::text,user_id,subject,status,created_at,updated_at FROM tickets WHERE id=$1`, id).Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if e != nil {
		return t, e
	}
	t.Messages, e = s.TicketMessages(ctx, id)
	return t, e
}
func (s *Store) TicketMessages(ctx context.Context, id string) ([]TicketMessage, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id::text,ticket_id::text,sender_user_id,is_admin,text,created_at FROM ticket_messages WHERE ticket_id=$1 ORDER BY created_at`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []TicketMessage{}
	for rows.Next() {
		var m TicketMessage
		if e = rows.Scan(&m.ID, &m.TicketID, &m.SenderUserID, &m.IsAdmin, &m.Text, &m.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) AddTicketMessage(ctx context.Context, id string, userID int64, isAdmin bool, text string) (Ticket, error) {
	status := "open"
	if isAdmin {
		status = "answered"
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO ticket_messages(ticket_id,sender_user_id,is_admin,text)VALUES($1,$2,$3,$4)`, id, userID, isAdmin, text)
	if e != nil {
		return Ticket{}, e
	}
	_, e = s.DB.ExecContext(ctx, `UPDATE tickets SET status=$2,updated_at=NOW() WHERE id=$1`, id, status)
	if e != nil {
		return Ticket{}, e
	}
	return s.Ticket(ctx, id)
}
func (s *Store) SetTicketStatus(ctx context.Context, id, status string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE tickets SET status=$2,updated_at=NOW() WHERE id=$1`, id, status)
	return e
}

func (s *Store) Diagnostic(ctx context.Context, source, message string, details map[string]any) error {
	raw, _ := json.Marshal(details)
	_, e := s.DB.ExecContext(ctx, `INSERT INTO diagnostics(source,message,details)VALUES($1,$2,$3)`, source, message, raw)
	return e
}
func (s *Store) Diagnostics(ctx context.Context) ([]Diagnostic, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id::text,source,level,message,details,resolved,created_at FROM diagnostics ORDER BY created_at DESC LIMIT 250`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Diagnostic{}
	for rows.Next() {
		var d Diagnostic
		var raw []byte
		if e = rows.Scan(&d.ID, &d.Source, &d.Level, &d.Message, &raw, &d.Resolved, &d.CreatedAt); e != nil {
			return nil, e
		}
		_ = json.Unmarshal(raw, &d.Details)
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *Store) ResolveDiagnostic(ctx context.Context, id string) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE diagnostics SET resolved=TRUE WHERE id=$1`, id)
	return e
}

func (s *Store) AdminAudit(ctx context.Context, adminID, targetID int64, action, reason string, details map[string]any) error {
	raw, _ := json.Marshal(details)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO admin_audit(admin_user_id,target_user_id,action,reason,details) VALUES($1,$2,$3,$4,$5)`, adminID, targetID, action, reason, raw)
	return err
}

func (s *Store) Users(ctx context.Context, q string) ([]User, error) {
	sqlq := `SELECT ` + userColumns + ` FROM users`
	args := []any{}
	query := strings.TrimSpace(q)
	if query != "" {
		if telegramID, err := strconv.ParseInt(query, 10, 64); err == nil {
			sqlq += ` WHERE first_name ILIKE $1 OR username ILIKE $1 OR telegram_id=$2`
			args = append(args, "%"+query+"%", telegramID)
		} else {
			sqlq += ` WHERE first_name ILIKE $1 OR username ILIKE $1`
			args = append(args, "%"+query+"%")
		}
	}
	sqlq += ` ORDER BY created_at DESC LIMIT 200`
	rows, e := s.DB.QueryContext(ctx, sqlq, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, x := scanUser(rows)
		if x != nil {
			return nil, x
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Store) UpdateUserAdmin(ctx context.Context, id int64, blocked, admin *bool, status string) error {
	if blocked != nil {
		if _, e := s.DB.ExecContext(ctx, `UPDATE users SET is_blocked=$2 WHERE id=$1`, id, *blocked); e != nil {
			return e
		}
	}
	if admin != nil {
		if _, e := s.DB.ExecContext(ctx, `UPDATE users SET is_admin=$2 WHERE id=$1`, id, *admin); e != nil {
			return e
		}
	}
	if status != "" {
		if _, e := s.DB.ExecContext(ctx, `UPDATE users SET subscription_status=$2 WHERE id=$1`, id, strings.ToUpper(status)); e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) ReferralCount(ctx context.Context, id int64) (int, error) {
	var n int
	e := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE referred_by_id=$1`, id).Scan(&n)
	return n, e
}
func (s *Store) Overview(ctx context.Context) (map[string]any, error) {
	var users, active, tickets, diagnostics int
	var revenue int64
	e := s.DB.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM users),(SELECT COUNT(*) FROM users WHERE subscription_status='ACTIVE'),(SELECT COUNT(*) FROM tickets WHERE status<>'closed'),(SELECT COALESCE(SUM(amount_kopecks),0) FROM payments WHERE status='succeeded'),(SELECT COUNT(*) FROM diagnostics WHERE resolved=FALSE)`).Scan(&users, &active, &tickets, &revenue, &diagnostics)
	return map[string]any{"users": users, "active_subscriptions": active, "open_tickets": tickets, "revenue_kopecks": revenue, "diagnostics": diagnostics}, e
}
func (s *Store) AllTelegramIDs(ctx context.Context) ([]int64, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT telegram_id FROM users WHERE is_blocked=FALSE`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
func (s *Store) CreateBroadcast(ctx context.Context, adminID int64, message string, buttons []map[string]string) (Broadcast, error) {
	raw, _ := json.Marshal(buttons)
	return scanBroadcast(s.DB.QueryRowContext(ctx, `INSERT INTO broadcasts(admin_user_id,text,buttons,status)VALUES($1,$2,$3,'draft') RETURNING id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at`, adminID, message, raw))
}

func scanBroadcast(row interface{ Scan(...any) error }) (Broadcast, error) {
	var b Broadcast
	var raw []byte
	err := row.Scan(&b.ID, &b.AdminUserID, &b.Text, &b.SourceChatID, &b.SourceMessageID, &raw, &b.Status, &b.SentCount, &b.FailedCount, &b.CreatedAt, &b.UpdatedAt)
	if err == nil {
		_ = json.Unmarshal(raw, &b.Buttons)
	}
	return b, err
}

func (s *Store) BeginBroadcastDraft(ctx context.Context, adminID int64) (Broadcast, error) {
	if current, err := s.LatestBroadcast(ctx, adminID); err == nil && current.Status != "completed" && current.Status != "sending" && current.Status != "cancelled" {
		return scanBroadcast(s.DB.QueryRowContext(ctx, `UPDATE broadcasts SET text='',source_chat_id=0,source_message_id=0,buttons='[]'::jsonb,status='awaiting',sent_count=0,failed_count=0,updated_at=NOW() WHERE id=$1 RETURNING id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at`, current.ID))
	}
	return scanBroadcast(s.DB.QueryRowContext(ctx, `INSERT INTO broadcasts(admin_user_id,text,status)VALUES($1,'','awaiting') RETURNING id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at`, adminID))
}

func (s *Store) AwaitingBroadcast(ctx context.Context, adminID int64) (Broadcast, error) {
	return scanBroadcast(s.DB.QueryRowContext(ctx, `SELECT id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at FROM broadcasts WHERE admin_user_id=$1 AND status='awaiting' ORDER BY updated_at DESC LIMIT 1`, adminID))
}

func (s *Store) LatestBroadcast(ctx context.Context, adminID int64) (Broadcast, error) {
	return scanBroadcast(s.DB.QueryRowContext(ctx, `SELECT id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at FROM broadcasts WHERE admin_user_id=$1 ORDER BY updated_at DESC LIMIT 1`, adminID))
}

func (s *Store) CaptureBroadcast(ctx context.Context, id string, chatID int64, messageID int, summary string) (Broadcast, error) {
	return scanBroadcast(s.DB.QueryRowContext(ctx, `UPDATE broadcasts SET text=$2,source_chat_id=$3,source_message_id=$4,status='preview',updated_at=NOW() WHERE id=$1 AND status='awaiting' RETURNING id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at`, id, summary, chatID, messageID))

}

func (s *Store) SetBroadcastStatus(ctx context.Context, id, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE broadcasts SET status=$2,updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (s *Store) SaveBroadcastButtons(ctx context.Context, id string, buttons []map[string]string) (Broadcast, error) {
	raw, _ := json.Marshal(buttons)
	return scanBroadcast(s.DB.QueryRowContext(ctx, `UPDATE broadcasts SET buttons=$2,updated_at=NOW() WHERE id=$1 AND status IN ('preview','confirmed') RETURNING id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at`, id, raw))
}

func (s *Store) ClaimBroadcast(ctx context.Context, id string) (bool, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE broadcasts SET status='sending',updated_at=NOW() WHERE id=$1 AND status='confirmed'`, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
func (s *Store) SendingBroadcasts(ctx context.Context) ([]Broadcast, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at FROM broadcasts WHERE status='sending' ORDER BY updated_at LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Broadcast{}
	for rows.Next() {
		broadcast, scanErr := scanBroadcast(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, broadcast)
	}
	return out, rows.Err()
}
func (s *Store) PrepareBroadcastDeliveries(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO broadcast_deliveries(broadcast_id,chat_id) SELECT $1,telegram_id FROM users WHERE is_blocked=FALSE ON CONFLICT(broadcast_id,chat_id) DO NOTHING`, id)
	return err
}
func (s *Store) ClaimBroadcastDeliveries(ctx context.Context, id string) ([]BroadcastDelivery, error) {
	rows, err := s.DB.QueryContext(ctx, `WITH picked AS (
 SELECT broadcast_id,chat_id FROM broadcast_deliveries
 WHERE broadcast_id=$1 AND status<>'sent' AND attempts<3 AND (status<>'processing' OR updated_at<NOW()-INTERVAL '5 minutes')
 ORDER BY chat_id LIMIT 100 FOR UPDATE SKIP LOCKED
) UPDATE broadcast_deliveries d SET status='processing',attempts=d.attempts+1,updated_at=NOW()
FROM picked p WHERE d.broadcast_id=p.broadcast_id AND d.chat_id=p.chat_id RETURNING d.chat_id,d.attempts`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BroadcastDelivery{}
	for rows.Next() {
		var delivery BroadcastDelivery
		if err = rows.Scan(&delivery.ChatID, &delivery.Attempts); err != nil {
			return nil, err
		}
		out = append(out, delivery)
	}
	return out, rows.Err()
}
func (s *Store) FinishBroadcastDelivery(ctx context.Context, id string, chatID int64, delivered bool, lastError string) error {
	status := "failed"
	if delivered {
		status = "sent"
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE broadcast_deliveries SET status=$3,last_error=$4,updated_at=NOW() WHERE broadcast_id=$1 AND chat_id=$2`, id, chatID, status, lastError)
	return err
}
func (s *Store) BroadcastDeliveryCounts(ctx context.Context, id string) (int, int, int, error) {
	var sent, failed, remaining int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE status='sent'),COUNT(*) FILTER(WHERE status='failed' AND attempts>=3),COUNT(*) FILTER(WHERE status='processing' OR (status<>'sent' AND attempts<3)) FROM broadcast_deliveries WHERE broadcast_id=$1`, id).Scan(&sent, &failed, &remaining)
	return sent, failed, remaining, err
}
func (s *Store) Broadcast(ctx context.Context, id string) (Broadcast, error) {
	return scanBroadcast(s.DB.QueryRowContext(ctx, `SELECT id::text,admin_user_id,text,source_chat_id,source_message_id,buttons,status,sent_count,failed_count,created_at,updated_at FROM broadcasts WHERE id=$1`, id))
}
func (s *Store) FinishBroadcast(ctx context.Context, id, status string, sent, failed int) error {
	_, e := s.DB.ExecContext(ctx, `UPDATE broadcasts SET status=$2,sent_count=$3,failed_count=$4,updated_at=NOW() WHERE id=$1`, id, status, sent, failed)
	return e
}

func IsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
