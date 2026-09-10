package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tgs-bot/internal/payments"
	"tgs-bot/internal/store"
)

type contextKey string

const userKey contextKey = "user"

func (s *Server) Handler(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	webRoot, _ := fs.Sub(static, "web")
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/auth/telegram", s.authTelegram)
	mux.HandleFunc("POST /api/webhooks/yookassa", s.webhookYooKassa)
	mux.HandleFunc("POST /api/webhooks/cryptobot", s.webhookCryptoBot)
	mux.HandleFunc("POST /api/webhooks/payments/{provider}", s.webhookAlternativePayment)
	mux.HandleFunc("POST /api/webhooks/remnawave", s.webhookRemnawave)

	mux.Handle("GET /api/bootstrap", s.auth(http.HandlerFunc(s.bootstrap)))
	mux.Handle("GET /api/events", s.auth(http.HandlerFunc(s.events)))
	mux.Handle("POST /api/trial", s.auth(http.HandlerFunc(s.trial)))
	mux.Handle("GET /api/tickets", s.auth(http.HandlerFunc(s.tickets)))
	mux.Handle("POST /api/tickets", s.auth(http.HandlerFunc(s.createTicket)))
	mux.Handle("POST /api/tickets/{id}/messages", s.auth(http.HandlerFunc(s.ticketMessage)))
	mux.Handle("POST /api/payments/checkout", s.auth(http.HandlerFunc(s.checkout)))
	mux.Handle("GET /api/payments/{id}", s.auth(http.HandlerFunc(s.paymentStatus)))
	mux.Handle("GET /api/nodes", s.auth(http.HandlerFunc(s.nodes)))
	mux.Handle("GET /api/devices", s.auth(http.HandlerFunc(s.devices)))
	mux.Handle("DELETE /api/devices/{hwid}", s.auth(http.HandlerFunc(s.deleteDevice)))

	s.registerAdmin(mux)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(webRoot)))
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(static, "web/index.html")
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/app", http.StatusFound)
	})
	return s.recover(s.headers(mux))
}

func (s *Server) headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.Logger.Error("panic", "value", v, "path", r.URL.Path)
				writeError(w, 500, "Внутренняя ошибка")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, 401, "Откройте приложение из Telegram")
			return
		}
		tgID, err := parseSession(s.Config.AppSecret, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(w, 401, "Сессия недействительна")
			return
		}
		u, err := s.Store.UserByTelegram(r.Context(), tgID)
		if err != nil {
			writeError(w, 401, "Пользователь не найден")
			return
		}
		if u.IsBlocked {
			writeError(w, 403, "Доступ ограничен")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, &u)))
	})
}
func (s *Server) admin(next http.Handler) http.Handler {
	return s.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := current(r)
		if !u.IsAdmin {
			writeError(w, 403, "Требуются права администратора")
			return
		}
		next.ServeHTTP(w, r)
	}))
}
func current(r *http.Request) *store.User { return r.Context().Value(userKey).(*store.User) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"detail": message})
}
func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		writeError(w, 400, "Некорректные данные")
		return false
	}
	return true
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.Store.DB.PingContext(ctx); err != nil {
		writeError(w, 503, "database unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "service": "tgs-bot"})
}

func (s *Server) authTelegram(w http.ResponseWriter, r *http.Request) {
	var body struct {
		InitData     string `json:"init_data"`
		ReferralCode string `json:"referral_code"`
	}
	if !decode(w, r, &body) {
		return
	}
	tg, err := verifyInitData(body.InitData, s.Config.TelegramToken, 24*time.Hour)
	if err != nil {
		writeError(w, 401, err.Error())
		return
	}
	u, err := s.Store.UpsertTelegramUser(r.Context(), tg, s.Config.AdminTelegramIDs, body.ReferralCode)
	if err != nil {
		s.Logger.Error("upsert telegram user", "error", err)
		writeError(w, 500, "Не удалось создать профиль")
		return
	}
	emergency, _ := s.Store.Setting(r.Context(), "emergency")
	writeJSON(w, 200, map[string]any{"token": issueSession(s.Config.AppSecret, u.TelegramID), "user": publicUser(u), "emergency": emergency})
}

func publicUser(u store.User) map[string]any {
	return map[string]any{"id": u.ID, "telegram_id": u.TelegramID, "username": u.Username, "first_name": u.FirstName, "photo_url": u.PhotoURL, "remnawave_username": u.RemnawaveUsername, "is_admin": u.IsAdmin, "trial_used": u.TrialUsed, "referral_code": u.ReferralCode, "subscription": map[string]any{"status": u.SubscriptionStatus, "expires_at": u.ExpiresAt, "traffic_limit_bytes": u.TrafficLimitBytes, "traffic_used_bytes": u.TrafficUsedBytes, "device_limit": u.DeviceLimit, "subscription_url": u.SubscriptionURL}}
}
func adminUserDTO(u store.User) map[string]any {
	out := publicUser(u)
	out["is_blocked"] = u.IsBlocked
	out["referred_by_id"] = u.ReferredByID
	out["remnawave_user_id"] = u.RemnawaveUserID
	out["remnawave_user_uuid"] = u.RemnawaveUserUUID
	out["created_at"] = u.CreatedAt
	out["last_seen_at"] = u.LastSeenAt
	return out
}
func tariffDTO(t store.Tariff) map[string]any {
	return map[string]any{"id": t.ID, "name": t.Name, "description": t.Description, "price_rub": float64(t.PriceKopecks) / 100, "days": t.Days, "traffic_gb": t.TrafficGB, "device_limit": t.DeviceLimit, "internal_squads": t.InternalSquads, "external_squad_uuid": t.ExternalSquadUUID, "active": t.Active, "pinned": t.Pinned, "position": t.Position}
}
func paymentDTO(p store.Payment) map[string]any {
	return map[string]any{"id": p.ID, "provider": p.Provider, "status": p.Status, "amount_rub": float64(p.AmountKopecks) / 100, "promo_code": p.PromoCode, "snapshot": p.Snapshot, "created_at": p.CreatedAt, "paid_at": p.PaidAt}
}

func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	s.refresh(r.Context(), u)
	content, _ := s.Store.Setting(r.Context(), "content")
	features, _ := s.Store.Setting(r.Context(), "features")
	trial, _ := s.Store.Setting(r.Context(), "trial")
	theme, _ := s.Store.Setting(r.Context(), "theme")
	language, _ := s.Store.Setting(r.Context(), "language")
	emergency, _ := s.Store.Setting(r.Context(), "emergency")
	more, _ := s.Store.Setting(r.Context(), "more_order")
	integrations, _ := s.Store.Setting(r.Context(), "integrations")
	subpage, _ := s.Store.Setting(r.Context(), "subpage")
	tariffs, _ := s.Store.Tariffs(r.Context(), false)
	tickets, _ := s.Store.Tickets(r.Context(), &u.ID)
	history, _ := s.Store.PaymentsByUser(r.Context(), u.ID)
	refs, _ := s.Store.ReferralCount(r.Context(), u.ID)
	tds := make([]any, 0, len(tariffs))
	for _, t := range tariffs {
		tds = append(tds, tariffDTO(t))
	}
	pds := make([]any, 0, len(history))
	for _, p := range history {
		pds = append(pds, paymentDTO(p))
	}
	sys, _ := s.Store.Setting(r.Context(), "system")
	sys["count"] = refs
	sys["link"] = "https://t.me/" + s.BotUsername + "?start=" + u.ReferralCode
	userDTO := publicUser(*u)
	object(userDTO["subscription"])["connected_devices"] = s.connectedDeviceCount(r.Context(), *u)
	definitions := []struct{ id, name, description string }{
		{"yookassa", "ЮKassa", "Банковская карта или СБП"}, {"cryptobot", "CryptoBot", "Криптовалюта через Telegram"},
		{"lava", "LAVA", "Платёжная форма LAVA Business"}, {"wata", "WATA", "Карты и СБП через WATA"},
		{"platega", "Platega", "Платёжная форма Platega"}, {"freekassa", "FreeKassa", "Платёжная форма FreeKassa"},
		{"heleket", "Heleket", "Криптовалютная платёжная форма"}, {"pally", "Pally", "Карты и СБП через Pally"},
	}
	methods := make([]map[string]any, 0, len(definitions))
	for _, item := range definitions {
		methods = append(methods, map[string]any{"id": item.id, "name": item.name, "description": item.description, "enabled": boolean(object(integrations[item.id])["enabled"])})
	}
	writeJSON(w, 200, map[string]any{"user": userDTO, "content": content, "features": features, "trial": trial, "theme": theme, "language": language, "emergency": emergency, "more_order": more["items"], "tariffs": tds, "tickets": tickets, "payments": pds, "referral": sys, "payment_methods": methods, "subpage": subpage})
}

func (s *Server) trial(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	features, _ := s.Store.Setting(r.Context(), "features")
	cfg, _ := s.Store.Setting(r.Context(), "trial")
	if !boolean(features["trial"]) || !boolean(cfg["enabled"]) {
		writeError(w, 400, "Пробный период выключен")
		return
	}
	if u.TrialUsed {
		writeError(w, 409, "Пробный период уже был использован")
		return
	}
	reserved, err := s.Store.ReserveTrial(r.Context(), u.ID)
	if err != nil {
		writeError(w, 500, "Не удалось зарезервировать пробный период")
		return
	}
	if !reserved {
		writeError(w, 409, "Пробный период уже был использован")
		return
	}
	limit := number(cfg["device_limit"], 1)
	err = s.applyEntitlement(r.Context(), u, entitlement{Days: number(cfg["days"], 3), TrafficGB: number(cfg["traffic_gb"], 10), TrafficMode: trafficPlan, DeviceLimit: &limit, InternalSquads: stringsList(cfg["internal_squads"]), ExternalSquadUUID: text(cfg["external_squad_uuid"]), UpdateSquads: true, Activate: true})
	if err != nil {
		if releaseErr := s.Store.ReleaseTrial(context.Background(), u.ID); releaseErr != nil {
			s.record(context.Background(), "trial", "Не удалось освободить резерв триала", map[string]any{"error": releaseErr.Error(), "user_id": u.ID})
		}
		writeError(w, 502, "Remnawave не выдал подписку. Проверьте диагностику.")
		return
	}
	u.TrialUsed = true
	s.notifyAdminsContent(r.Context(), "trial_admin_message", "<b>Активирован пробный период</b>\n{name} · <code>{id}</code>", map[string]string{"name": u.FirstName, "id": strconv.FormatInt(u.TelegramID, 10)})
	s.publishAccount(u.ID)
	writeJSON(w, 200, map[string]any{"ok": true, "user": publicUser(*u)})
}

func (s *Server) tickets(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	rows, err := s.Store.Tickets(r.Context(), &u.ID)
	if err != nil {
		writeError(w, 500, "Не удалось загрузить тикеты")
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) createTicket(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	if u.IsAdmin {
		writeError(w, http.StatusForbidden, "Администратор отвечает на обращения пользователей")
		return
	}
	var body struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Subject = strings.TrimSpace(body.Subject)
	body.Description = strings.TrimSpace(body.Description)
	if len([]rune(body.Subject)) < 3 || len([]rune(body.Subject)) > 140 || len([]rune(body.Description)) < 3 || len([]rune(body.Description)) > 5000 {
		writeError(w, 400, "Заполните тему и описание")
		return
	}
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["support"]) {
		writeError(w, 400, "Поддержка временно отключена")
		return
	}
	ticket, err := s.Store.CreateTicket(r.Context(), u.ID, body.Subject, body.Description)
	if err != nil {
		writeError(w, 500, "Не удалось создать тикет")
		return
	}
	s.notifyAdminsContent(r.Context(), "new_ticket_admin_message", "<b>Новый тикет</b>\n{subject}\n{name} · <code>{id}</code>", map[string]string{"subject": ticket.Subject, "name": u.FirstName, "id": strconv.FormatInt(u.TelegramID, 10)})
	s.publishSupport(ticket.UserID, ticket.ID)
	writeJSON(w, 201, ticket)
}
func (s *Server) ticketMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Text = strings.TrimSpace(body.Text)
	if body.Text == "" || len([]rune(body.Text)) > 5000 {
		writeError(w, 400, "Введите сообщение")
		return
	}
	u := current(r)
	if !u.IsAdmin {
		features, _ := s.Store.Setting(r.Context(), "features")
		if !boolean(features["support"]) {
			writeError(w, 404, "Поддержка временно отключена")
			return
		}
	}
	ticket, err := s.Store.Ticket(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, 404, "Тикет не найден")
		return
	}
	if ticket.UserID != u.ID && !u.IsAdmin {
		writeError(w, 404, "Тикет не найден")
		return
	}
	if ticket.Status == "closed" && !u.IsAdmin {
		writeError(w, 409, "Тикет закрыт")
		return
	}
	ticket, err = s.Store.AddTicketMessage(r.Context(), ticket.ID, u.ID, u.IsAdmin, body.Text)
	if err != nil {
		writeError(w, 500, "Не удалось отправить сообщение")
		return
	}
	if u.IsAdmin {
		owner, _ := s.Store.UserByID(r.Context(), ticket.UserID)
		s.sendContentMessage(r.Context(), owner.TelegramID, "support_reply_message", "Поддержка ответила в тикете «<b>{subject}</b>». Откройте Mini App.", map[string]string{"subject": ticket.Subject}, nil)
	} else {
		s.notifyAdminsContent(r.Context(), "ticket_message_admin_message", "Новое сообщение в тикете «<b>{subject}</b>».", map[string]string{"subject": ticket.Subject})
	}
	s.publishSupport(ticket.UserID, ticket.ID)
	writeJSON(w, 200, ticket)
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TariffID  string `json:"tariff_id"`
		Provider  string `json:"provider"`
		PromoCode string `json:"promo_code"`
	}
	if !decode(w, r, &body) {
		return
	}
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["payments"]) {
		writeError(w, 400, "Платежи временно отключены")
		return
	}
	tariff, err := s.Store.Tariff(r.Context(), body.TariffID)
	if err != nil || !tariff.Active {
		writeError(w, 404, "Тариф не найден")
		return
	}
	amount := tariff.PriceKopecks
	promoCode := ""
	if strings.TrimSpace(body.PromoCode) != "" && boolean(features["promo_codes"]) {
		promo, err := s.Store.PromoByCode(r.Context(), body.PromoCode)
		if err != nil {
			writeError(w, 400, "Промокод не найден")
			return
		}
		if err = validPromo(promo); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		amount = discounted(amount, promo.DiscountPercent)
		promoCode = promo.Code
	}
	snapshot := tariffDTO(tariff)
	p, err := s.Store.CreatePayment(r.Context(), store.Payment{UserID: current(r).ID, TariffID: tariff.ID, Provider: body.Provider, AmountKopecks: amount, PromoCode: promoCode, Snapshot: snapshot})
	if err != nil {
		writeError(w, 500, "Не удалось создать платёж")
		return
	}
	integrations, _ := s.Store.Setting(r.Context(), "integrations")
	var providerID, payURL string
	description := "TGS VPN — " + tariff.Name
	switch body.Provider {
	case "yookassa":
		providerID, payURL, err = s.Payments.YooKassaCreate(r.Context(), object(integrations["yookassa"]), p.ID, amount, description, s.Config.MiniAppURL("")+"?payment="+p.ID)
	case "cryptobot":
		providerID, payURL, err = s.Payments.CryptoBotCreate(r.Context(), object(integrations["cryptobot"]), p.ID, amount, description)
	default:
		known := false
		for _, name := range payments.AlternativeProviders {
			if body.Provider == name {
				known = true
				break
			}
		}
		if !known {
			err = fmt.Errorf("неизвестный способ оплаты")
			break
		}
		providerID, payURL, err = s.Payments.CreateAlternative(r.Context(), body.Provider, object(integrations[body.Provider]), payments.CreateRequest{
			MerchantOrderID: p.MerchantOrderID, AmountKopecks: amount, Description: description,
			ReturnURL: s.Config.MiniAppURL("") + "?payment=" + p.ID, WebhookBaseURL: s.Config.PublicBaseURL,
			TelegramID: current(r).TelegramID, Username: current(r).Username,
		})
	}
	if err != nil {
		_ = s.Store.FailPayment(r.Context(), p.ID)
		s.record(r.Context(), "payments", err.Error(), map[string]any{"provider": body.Provider, "payment_id": p.ID})
		writeError(w, 502, err.Error())
		return
	}
	if err = s.Store.SetPaymentProviderID(r.Context(), p.ID, providerID); err != nil {
		s.record(r.Context(), "payments", "Не удалось сохранить идентификатор платежа", map[string]any{"error": err.Error(), "payment_id": p.ID, "provider": body.Provider})
		writeError(w, 500, "Платёж создан, но не сохранён. Обратитесь в поддержку и не оплачивайте счёт повторно.")
		return
	}
	writeJSON(w, 200, map[string]any{"payment_id": p.ID, "pay_url": payURL, "amount_rub": float64(amount) / 100})
}
func (s *Server) paymentStatus(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Payment(r.Context(), r.PathValue("id"))
	u := current(r)
	if err != nil || (p.UserID != u.ID && !u.IsAdmin) {
		writeError(w, 404, "Платёж не найден")
		return
	}
	writeJSON(w, 200, paymentDTO(p))
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["server_status"]) {
		writeError(w, 404, "Функция выключена")
		return
	}
	client, err := s.remna(r.Context())
	if err != nil {
		writeError(w, 500, "Ошибка настроек")
		return
	}
	rows, err := client.Nodes(r.Context())
	if err != nil {
		s.record(r.Context(), "remnawave", "Ошибка получения нод", map[string]any{"error": err.Error()})
		writeError(w, 502, "Панель Remnawave недоступна")
		return
	}
	out := []map[string]any{}
	for _, item := range rows {
		online := boolean(item["isConnected"]) || boolean(item["isOnline"])
		if boolean(item["isDisabled"]) {
			online = false
		}
		name := text(item["name"])
		if name == "" {
			name = text(item["nodeName"])
		}
		out = append(out, map[string]any{"id": item["id"], "uuid": item["uuid"], "name": name, "country_code": item["countryCode"], "status": map[bool]string{true: "online", false: "offline"}[online]})
	}
	writeJSON(w, 200, out)
}
func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["devices"]) {
		writeError(w, 404, "Функция выключена")
		return
	}
	client, _ := s.remna(r.Context())
	remote, err := client.UserByTelegram(r.Context(), current(r).TelegramID)
	if err != nil || remote == nil {
		writeError(w, 502, "Не удалось получить устройства")
		return
	}
	rows, err := client.Devices(r.Context(), remote)
	if err != nil {
		writeError(w, 502, "Не удалось получить устройства")
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	features, _ := s.Store.Setting(r.Context(), "features")
	if !boolean(features["devices"]) {
		writeError(w, 404, "Функция выключена")
		return
	}
	client, _ := s.remna(r.Context())
	remote, err := client.UserByTelegram(r.Context(), current(r).TelegramID)
	if err != nil || remote == nil {
		writeError(w, 404, "Подписка не найдена")
		return
	}
	rows, err := client.DeleteDevice(r.Context(), remote, r.PathValue("hwid"))
	if err != nil {
		s.record(r.Context(), "remnawave", "Ошибка удаления устройства", map[string]any{"error": err.Error()})
		writeError(w, 502, "Не удалось удалить устройство")
		return
	}
	writeJSON(w, 200, map[string]any{"devices": rows})
}

func (s *Server) webhookYooKassa(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if !decode(w, r, &payload) {
		return
	}
	providerID := text(object(payload["object"])["id"])
	p, err := s.Store.PaymentByProvider(r.Context(), providerID)
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	settings, _ := s.Store.Setting(r.Context(), "integrations")
	remote, err := s.Payments.YooKassaGet(r.Context(), object(settings["yookassa"]), providerID)
	if err != nil {
		s.record(r.Context(), "yookassa", "Ошибка проверки webhook", map[string]any{"error": err.Error()})
		writeError(w, 502, "verification failed")
		return
	}
	if text(remote["status"]) == "succeeded" && boolean(remote["paid"]) {
		if err = s.completePayment(r.Context(), p); err != nil {
			s.record(r.Context(), "payments", "Не удалось выдать оплаченную подписку", map[string]any{"error": err.Error(), "payment_id": p.ID})
			writeError(w, 502, "provisioning failed")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (s *Server) webhookCryptoBot(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, 400, "bad body")
		return
	}
	settings, _ := s.Store.Setting(r.Context(), "integrations")
	cfg := object(settings["cryptobot"])
	if !payments.VerifyCryptoBot(text(cfg["token"]), raw, r.Header.Get("Crypto-Pay-API-Signature")) {
		writeError(w, 401, "invalid signature")
		return
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		writeError(w, 400, "bad json")
		return
	}
	if text(payload["update_type"]) == "invoice_paid" {
		invoice := object(payload["payload"])
		p, err := s.Store.Payment(r.Context(), text(invoice["payload"]))
		if err == nil && p.Provider == "cryptobot" {
			if err = s.completePayment(r.Context(), p); err != nil {
				s.record(r.Context(), "payments", "Не удалось выдать оплаченную подписку", map[string]any{"error": err.Error(), "payment_id": p.ID})
				writeError(w, 502, "provisioning failed")
				return
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) webhookAlternativePayment(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(strings.TrimSpace(r.PathValue("provider")))
	known := false
	for _, name := range payments.AlternativeProviders {
		if provider == name {
			known = true
			break
		}
	}
	if !known {
		writeError(w, http.StatusNotFound, "unknown provider")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad body")
		return
	}
	form := url.Values{}
	if provider == "freekassa" || provider == "pally" {
		form, err = url.ParseQuery(string(raw))
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad form")
			return
		}
	}
	settings, _ := s.Store.Setting(r.Context(), "integrations")
	result, err := s.Payments.ParseAlternativeWebhook(r.Context(), provider, object(settings[provider]), r.Header, raw, form)
	if err != nil {
		s.record(r.Context(), provider, "Webhook платежа отклонён", map[string]any{"error": err.Error()})
		writeError(w, http.StatusUnauthorized, "invalid webhook")
		return
	}
	var payment store.Payment
	if result.MerchantOrderID > 0 {
		payment, err = s.Store.PaymentByMerchantOrder(r.Context(), result.MerchantOrderID)
	} else if result.ExternalID != "" {
		payment, err = s.Store.PaymentByProvider(r.Context(), result.ExternalID)
	} else {
		err = sql.ErrNoRows
	}
	if err != nil || payment.Provider != provider {
		writeError(w, http.StatusNotFound, "payment not found")
		return
	}
	if strings.TrimSpace(result.Currency) != "" && !strings.EqualFold(result.Currency, "RUB") {
		writeError(w, http.StatusBadRequest, "currency mismatch")
		return
	}
	if result.Amount > 0 && math.Abs(result.Amount-float64(payment.AmountKopecks)/100) > 0.011 {
		s.record(r.Context(), provider, "Сумма webhook не совпала с платежом", map[string]any{"payment_id": payment.ID})
		writeError(w, http.StatusBadRequest, "amount mismatch")
		return
	}
	if result.Paid {
		if err = s.completePayment(r.Context(), payment); err != nil {
			s.record(r.Context(), "payments", "Не удалось выдать оплаченную подписку", map[string]any{"error": err.Error(), "payment_id": payment.ID})
			writeError(w, http.StatusBadGateway, "provisioning failed")
			return
		}
	} else if result.Cancelled {
		_ = s.Store.FailPayment(r.Context(), payment.ID)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if provider == "freekassa" || provider == "pally" {
		_, _ = w.Write([]byte("YES"))
	} else {
		_, _ = w.Write([]byte("OK"))
	}
}
func (s *Server) webhookRemnawave(w http.ResponseWriter, r *http.Request) {
	settings, _ := s.Store.Setting(r.Context(), "integrations")
	webhookSecret := text(object(settings["remnawave"])["webhook_secret"])
	if webhookSecret != "" && r.Header.Get("X-Webhook-Secret") != webhookSecret {
		writeError(w, http.StatusUnauthorized, "invalid webhook secret")
		return
	}
	var payload map[string]any
	if !decode(w, r, &payload) {
		return
	}
	data := object(payload["data"])
	id, _ := mapID(data["telegramId"])
	if id == 0 {
		id, _ = mapID(payload["telegramId"])
	}
	if id > 0 {
		if u, err := s.Store.UserByTelegram(r.Context(), id); err == nil {
			s.refresh(r.Context(), &u)
			s.publishAccount(u.ID)
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

var _ = sql.ErrNoRows
var _ = strconv.Itoa
