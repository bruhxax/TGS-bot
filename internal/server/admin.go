package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"tgs-bot/internal/store"
)

var settingKeys = map[string]bool{"content": true, "features": true, "trial": true, "integrations": true, "theme": true, "system": true, "emergency": true, "language": true, "more_order": true}
var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var secretSettingKeys = map[string]bool{"token": true, "secret_key": true, "bot_token": true, "webhook_secret": true}

const maskedSecret = "••••••••"

func (s *Server) registerAdmin(mux *http.ServeMux) {
	mux.Handle("GET /api/admin/overview", s.admin(http.HandlerFunc(s.adminOverview)))
	mux.Handle("GET /api/admin/settings/{key}", s.admin(http.HandlerFunc(s.adminGetSetting)))
	mux.Handle("PUT /api/admin/settings/{key}", s.admin(http.HandlerFunc(s.adminSaveSetting)))
	mux.Handle("GET /api/admin/squads", s.admin(http.HandlerFunc(s.adminSquads)))
	mux.Handle("POST /api/admin/integrations/{kind}/test", s.admin(http.HandlerFunc(s.adminTestIntegration)))
	mux.Handle("GET /api/admin/tariffs", s.admin(http.HandlerFunc(s.adminTariffs)))
	mux.Handle("POST /api/admin/tariffs", s.admin(http.HandlerFunc(s.adminCreateTariff)))
	mux.Handle("PUT /api/admin/tariffs/{id}", s.admin(http.HandlerFunc(s.adminUpdateTariff)))
	mux.Handle("DELETE /api/admin/tariffs/{id}", s.admin(http.HandlerFunc(s.adminDeleteTariff)))
	mux.Handle("GET /api/admin/users", s.admin(http.HandlerFunc(s.adminUsers)))
	mux.Handle("PATCH /api/admin/users/{id}", s.admin(http.HandlerFunc(s.adminUpdateUser)))
	mux.Handle("GET /api/admin/promos", s.admin(http.HandlerFunc(s.adminPromos)))
	mux.Handle("POST /api/admin/promos", s.admin(http.HandlerFunc(s.adminCreatePromo)))
	mux.Handle("DELETE /api/admin/promos/{id}", s.admin(http.HandlerFunc(s.adminDeletePromo)))
	mux.Handle("GET /api/admin/tickets", s.admin(http.HandlerFunc(s.adminTickets)))
	mux.Handle("PATCH /api/admin/tickets/{id}", s.admin(http.HandlerFunc(s.adminTicketStatus)))
	mux.Handle("GET /api/admin/diagnostics", s.admin(http.HandlerFunc(s.adminDiagnostics)))
	mux.Handle("PATCH /api/admin/diagnostics/{id}", s.admin(http.HandlerFunc(s.adminResolveDiagnostic)))
	mux.Handle("GET /api/admin/broadcast", s.admin(http.HandlerFunc(s.adminBroadcast)))
	mux.Handle("POST /api/admin/broadcast/draft", s.admin(http.HandlerFunc(s.adminBroadcastDraft)))
	mux.Handle("PUT /api/admin/broadcast/{id}/buttons", s.admin(http.HandlerFunc(s.adminBroadcastButtons)))
	mux.Handle("POST /api/admin/broadcast/{id}/test", s.admin(http.HandlerFunc(s.adminBroadcastTest)))
	mux.Handle("POST /api/admin/broadcast/{id}/send", s.admin(http.HandlerFunc(s.adminBroadcastSend)))
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	value, err := s.Store.Overview(r.Context())
	if err != nil {
		writeError(w, 500, "Не удалось загрузить статистику")
		return
	}
	writeJSON(w, 200, value)
}

func mask(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			if secretSettingKeys[key] && text(item) != "" {
				out[key] = maskedSecret
			} else {
				out[key] = mask(item)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = mask(item)
		}
		return out
	default:
		return value
	}
}

func preserveMaskedSecrets(current, incoming map[string]any) map[string]any {
	for key, value := range incoming {
		if nested, ok := value.(map[string]any); ok {
			previous, _ := current[key].(map[string]any)
			incoming[key] = preserveMaskedSecrets(previous, nested)
			continue
		}
		if secretSettingKeys[key] && text(value) == maskedSecret {
			incoming[key] = current[key]
		}
	}
	return incoming
}

func (s *Server) adminGetSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !settingKeys[key] {
		writeError(w, 404, "Раздел не найден")
		return
	}
	value, err := s.Store.Setting(r.Context(), key)
	if err != nil {
		writeError(w, 500, "Не удалось загрузить настройки")
		return
	}
	out := map[string]any{"key": key, "value": mask(value)}
	if key == "theme" {
		out["theme_templates"] = store.ThemeTemplates
	}
	writeJSON(w, 200, out)
}
func (s *Server) adminSaveSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !settingKeys[key] {
		writeError(w, 404, "Раздел не найден")
		return
	}
	var body struct {
		Value map[string]any `json:"value"`
	}
	if !decode(w, r, &body) {
		return
	}
	if key == "integrations" {
		current, err := s.Store.Setting(r.Context(), key)
		if err != nil {
			writeError(w, 500, "Не удалось загрузить текущие настройки")
			return
		}
		body.Value = preserveMaskedSecrets(current, body.Value)
	}
	if key == "theme" {
		for _, name := range []string{"accent", "background", "surface", "surface_alt", "text", "muted"} {
			if value := text(body.Value[name]); value != "" && !colorPattern.MatchString(value) {
				writeError(w, 400, "Некорректный цвет")
				return
			}
		}
	}
	if key == "language" {
		lang := text(body.Value["default"])
		if lang != "ru" && lang != "en" {
			writeError(w, 400, "Доступны языки ru и en")
			return
		}
	}
	if key == "content" {
		for _, prefix := range []string{"trial_button", "cabinet_button", "support_button"} {
			style := text(body.Value[prefix+"_style"])
			if style != "" && style != "primary" && style != "success" && style != "danger" {
				writeError(w, 400, "Неизвестный цвет кнопки")
				return
			}
			if emoji := strings.TrimSpace(text(body.Value[prefix+"_emoji_id"])); emoji != "" && !customEmojiPattern.MatchString(emoji) {
				writeError(w, 400, "Некорректный ID премиум-эмодзи")
				return
			}
		}
	}
	value, err := s.Store.SaveSetting(r.Context(), key, body.Value)
	if err != nil {
		writeError(w, 500, "Не удалось сохранить настройки")
		return
	}
	out := map[string]any{"key": key, "value": mask(value)}
	if key == "theme" {
		out["theme_templates"] = store.ThemeTemplates
	}
	s.publishBootstrap()
	writeJSON(w, 200, out)
}

func (s *Server) adminSquads(w http.ResponseWriter, r *http.Request) {
	client, err := s.remna(r.Context())
	if err != nil {
		writeError(w, 500, "Ошибка настроек")
		return
	}
	internal, err := client.InternalSquads(r.Context())
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	external, err := client.ExternalSquads(r.Context())
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	normalize := func(rows []map[string]any) []map[string]string {
		out := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			uuid := text(row["uuid"])
			if uuid == "" {
				uuid = fmt.Sprint(row["id"])
			}
			name := text(row["name"])
			if name == "" {
				name = text(row["title"])
			}
			if uuid != "" && uuid != "<nil>" {
				if name == "" {
					name = uuid
				}
				out = append(out, map[string]string{"uuid": uuid, "name": name})
			}
		}
		return out
	}
	writeJSON(w, 200, map[string]any{"internal": normalize(internal), "external": normalize(external)})
}
func (s *Server) adminTestIntegration(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	settings, _ := s.Store.Setting(r.Context(), "integrations")
	var err error
	switch kind {
	case "remnawave":
		client, _ := s.remna(r.Context())
		var rows []map[string]any
		rows, err = client.Nodes(r.Context())
		if err == nil {
			writeJSON(w, 200, map[string]any{"ok": true, "message": fmt.Sprintf("Remnawave отвечает, нод: %d", len(rows))})
			return
		}
	case "yookassa", "cryptobot":
		err = s.Payments.Test(r.Context(), kind, object(settings[kind]))
		if err == nil {
			writeJSON(w, 200, map[string]any{"ok": true, "message": map[string]string{"yookassa": "ЮKassa отвечает", "cryptobot": "CryptoBot отвечает"}[kind]})
			return
		}
	default:
		writeError(w, 404, "Интеграция не найдена")
		return
	}
	s.record(r.Context(), kind, "Проверка интеграции не пройдена", map[string]any{"error": err.Error()})
	writeError(w, 502, "Проверка не пройдена")
}

func (s *Server) adminTariffs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Tariffs(r.Context(), true)
	if err != nil {
		writeError(w, 500, "Не удалось загрузить тарифы")
		return
	}
	out := []any{}
	for _, item := range rows {
		out = append(out, tariffDTO(item))
	}
	writeJSON(w, 200, out)
}

type tariffRequest struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	PriceRUB          float64  `json:"price_rub"`
	Days              int      `json:"days"`
	TrafficGB         int      `json:"traffic_gb"`
	DeviceLimit       int      `json:"device_limit"`
	InternalSquads    []string `json:"internal_squads"`
	ExternalSquadUUID string   `json:"external_squad_uuid"`
	Active            bool     `json:"active"`
	Pinned            bool     `json:"pinned"`
	Position          int      `json:"position"`
}

func validateTariff(v tariffRequest) error {
	v.Name = strings.TrimSpace(v.Name)
	if len([]rune(v.Name)) < 2 || len([]rune(v.Name)) > 96 {
		return fmt.Errorf("название тарифа: 2–96 символов")
	}
	if v.PriceRUB <= 0 || v.PriceRUB > 10000000 {
		return fmt.Errorf("некорректная цена")
	}
	if v.Days <= 0 || v.Days > 3650 {
		return fmt.Errorf("некорректный срок")
	}
	if v.TrafficGB < 0 || v.DeviceLimit < 0 {
		return fmt.Errorf("лимиты не могут быть отрицательными")
	}
	return nil
}
func tariffFrom(v tariffRequest) store.Tariff {
	return store.Tariff{Name: strings.TrimSpace(v.Name), Description: strings.TrimSpace(v.Description), PriceKopecks: int64(v.PriceRUB*100 + 0.5), Days: v.Days, TrafficGB: v.TrafficGB, DeviceLimit: v.DeviceLimit, InternalSquads: v.InternalSquads, ExternalSquadUUID: v.ExternalSquadUUID, Active: v.Active, Pinned: v.Pinned, Position: v.Position}
}
func (s *Server) adminCreateTariff(w http.ResponseWriter, r *http.Request) {
	var body tariffRequest
	if !decode(w, r, &body) {
		return
	}
	if err := validateTariff(body); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	item, err := s.Store.SaveTariff(r.Context(), tariffFrom(body))
	if err != nil {
		writeError(w, 500, "Не удалось создать тариф")
		return
	}
	s.publishBootstrap()
	writeJSON(w, 201, tariffDTO(item))
}
func (s *Server) adminUpdateTariff(w http.ResponseWriter, r *http.Request) {
	var body tariffRequest
	if !decode(w, r, &body) {
		return
	}
	if err := validateTariff(body); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	item := tariffFrom(body)
	item.ID = r.PathValue("id")
	item, err := s.Store.SaveTariff(r.Context(), item)
	if err != nil {
		writeError(w, 404, "Тариф не найден")
		return
	}
	s.publishBootstrap()
	writeJSON(w, 200, tariffDTO(item))
}
func (s *Server) adminDeleteTariff(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteTariff(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, 500, "Не удалось удалить тариф")
		return
	}
	s.publishBootstrap()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Users(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, 500, "Не удалось загрузить пользователей")
		return
	}
	out := []any{}
	for _, u := range rows {
		item := adminUserDTO(u)
		item["referral_count"], _ = s.Store.ReferralCount(r.Context(), u.ID)
		out = append(out, item)
	}
	writeJSON(w, 200, out)
}
func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "Некорректный ID")
		return
	}
	var body struct {
		AddDays            int    `json:"add_days"`
		AddTrafficGB       int    `json:"add_traffic_gb"`
		DeviceLimit        *int   `json:"device_limit"`
		Blocked            *bool  `json:"blocked"`
		Admin              *bool  `json:"admin"`
		SubscriptionStatus string `json:"subscription_status"`
		Reason             string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.AddDays < 0 || body.AddDays > 3650 || body.AddTrafficGB < 0 || body.AddTrafficGB > 1000000 {
		writeError(w, 400, "Некорректные лимиты")
		return
	}
	body.SubscriptionStatus = strings.ToUpper(strings.TrimSpace(body.SubscriptionStatus))
	body.Reason = strings.TrimSpace(body.Reason)
	if len([]rune(body.Reason)) > 500 {
		writeError(w, 400, "Причина слишком длинная")
		return
	}
	if body.SubscriptionStatus != "" && body.SubscriptionStatus != "ACTIVE" && body.SubscriptionStatus != "DISABLED" {
		writeError(w, 400, "Доступны статусы ACTIVE и DISABLED")
		return
	}
	target, err := s.Store.UserByID(r.Context(), id)
	if err != nil {
		writeError(w, 404, "Пользователь не найден")
		return
	}
	if body.Admin != nil && target.ID == current(r).ID {
		body.Admin = nil
	}
	if body.SubscriptionStatus != "" {
		client, remnaErr := s.remna(r.Context())
		if remnaErr == nil && client.Configured() {
			remote, findErr := client.UserByTelegram(r.Context(), target.TelegramID)
			if findErr != nil {
				writeError(w, 502, "Remnawave не применил статус")
				return
			}
			if remote != nil {
				updated, updateErr := client.UpdateStatus(r.Context(), remote, body.SubscriptionStatus)
				if updateErr != nil {
					s.record(r.Context(), "remnawave", "Не удалось изменить статус пользователя", map[string]any{"error": updateErr.Error(), "telegram_id": target.TelegramID})
					writeError(w, 502, "Remnawave не применил статус")
					return
				}
				syncRemote(&target, updated)
			}
		}
	}
	if err = s.Store.UpdateUserAdmin(r.Context(), id, body.Blocked, body.Admin, body.SubscriptionStatus); err != nil {
		writeError(w, 500, "Не удалось обновить пользователя")
		return
	}
	if body.AddDays > 0 || body.AddTrafficGB > 0 || body.DeviceLimit != nil {
		mode := trafficKeep
		if body.AddTrafficGB > 0 {
			mode = trafficAdd
		}
		if err = s.applyEntitlement(r.Context(), &target, entitlement{Days: body.AddDays, TrafficGB: body.AddTrafficGB, TrafficMode: mode, DeviceLimit: body.DeviceLimit, Activate: body.AddDays > 0 && body.SubscriptionStatus != "DISABLED"}); err != nil {
			writeError(w, 502, "Remnawave не применил изменения")
			return
		}
	}
	details := map[string]any{"add_days": body.AddDays, "add_traffic_gb": body.AddTrafficGB, "subscription_status": body.SubscriptionStatus}
	if body.Blocked != nil {
		details["cabinet_blocked"] = *body.Blocked
	}
	if body.Admin != nil {
		details["is_admin"] = *body.Admin
	}
	if body.DeviceLimit != nil {
		details["device_limit"] = *body.DeviceLimit
	}
	if auditErr := s.Store.AdminAudit(r.Context(), current(r).ID, target.ID, "update_user", body.Reason, details); auditErr != nil {
		s.record(r.Context(), "admin", "Не удалось записать действие администратора", map[string]any{"error": auditErr.Error(), "target_user_id": target.ID})
	}
	if body.Blocked != nil && *body.Blocked && body.SubscriptionStatus == "DISABLED" {
		s.sendContentMessage(r.Context(), target.TelegramID, "full_block_message", "Доступ к кабинету и VPN временно заблокирован администратором.", nil, nil)
	}
	target, _ = s.Store.UserByID(r.Context(), id)
	s.publishAccount(target.ID)
	writeJSON(w, 200, adminUserDTO(target))
}

func (s *Server) adminPromos(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Promos(r.Context())
	if err != nil {
		writeError(w, 500, "Не удалось загрузить промокоды")
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) adminCreatePromo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code            string     `json:"code"`
		DiscountPercent int        `json:"discount_percent"`
		MaxUses         int        `json:"max_uses"`
		Active          bool       `json:"active"`
		ExpiresAt       *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.Code = strings.ToUpper(strings.TrimSpace(body.Code))
	if len(body.Code) < 2 || len(body.Code) > 64 || body.DiscountPercent < 0 || body.DiscountPercent > 100 || body.MaxUses < 0 {
		writeError(w, 400, "Некорректные параметры промокода")
		return
	}
	item, err := s.Store.CreatePromo(r.Context(), store.Promo{Code: body.Code, DiscountPercent: body.DiscountPercent, MaxUses: body.MaxUses, Active: body.Active, ExpiresAt: body.ExpiresAt})
	if err != nil {
		writeError(w, 409, "Промокод уже существует")
		return
	}
	writeJSON(w, 201, item)
}
func (s *Server) adminDeletePromo(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeletePromo(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, 404, "Промокод не найден")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) adminTickets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Tickets(r.Context(), nil)
	if err != nil {
		writeError(w, 500, "Не удалось загрузить тикеты")
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) adminTicketStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Status != "open" && body.Status != "answered" && body.Status != "closed" {
		writeError(w, 400, "Неизвестный статус")
		return
	}
	if err := s.Store.SetTicketStatus(r.Context(), r.PathValue("id"), body.Status); err != nil {
		writeError(w, 404, "Тикет не найден")
		return
	}
	ticket, _ := s.Store.Ticket(r.Context(), r.PathValue("id"))
	if owner, err := s.Store.UserByID(r.Context(), ticket.UserID); err == nil {
		ticket.User = &owner
	}
	s.publishSupport(ticket.UserID, ticket.ID)
	writeJSON(w, 200, ticket)
}
func (s *Server) adminDiagnostics(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.Diagnostics(r.Context())
	if err != nil {
		writeError(w, 500, "Не удалось загрузить диагностику")
		return
	}
	writeJSON(w, 200, rows)
}
func (s *Server) adminResolveDiagnostic(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.ResolveDiagnostic(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, 404, "Запись не найдена")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

var _ = sql.ErrNoRows
