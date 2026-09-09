package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"tgs-bot/internal/config"
	"tgs-bot/internal/payments"
	"tgs-bot/internal/remnawave"
	"tgs-bot/internal/store"
	"tgs-bot/internal/telegram"
)

const gigabyte int64 = 1024 * 1024 * 1024

type Server struct {
	Config      config.Config
	Store       *store.Store
	Payments    *payments.Provider
	Telegram    *telegram.Client
	Logger      *slog.Logger
	BotUsername string
}

func New(cfg config.Config, st *store.Store, logger *slog.Logger) *Server {
	return &Server{Config: cfg, Store: st, Payments: payments.New(), Telegram: telegram.New(cfg.TelegramToken), Logger: logger, BotUsername: "rwTGS_bot"}
}

func (s *Server) RunMaintenance(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.recoverPayments(ctx)
		s.recoverBroadcasts(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) recoverBroadcasts(ctx context.Context) {
	rows, err := s.Store.SendingBroadcasts(ctx)
	if err != nil {
		s.Logger.Warn("list sending broadcasts", "error", err)
		return
	}
	for _, draft := range rows {
		admin, userErr := s.Store.UserByID(ctx, draft.AdminUserID)
		if userErr == nil {
			go s.sendBroadcastCopies(context.Background(), admin.TelegramID, draft)
		}
	}
}

func (s *Server) recoverPayments(ctx context.Context) {
	rows, err := s.Store.StaleProcessingPayments(ctx)
	if err != nil {
		s.Logger.Warn("list stale payments", "error", err)
		return
	}
	for _, payment := range rows {
		if err = s.completePayment(ctx, payment); err != nil {
			s.record(ctx, "payments", "Не удалось восстановить зависшее начисление", map[string]any{"error": err.Error(), "payment_id": payment.ID})
		}
	}
}

func (s *Server) remna(ctx context.Context) (*remnawave.Client, error) {
	settings, err := s.Store.Setting(ctx, "integrations")
	if err != nil {
		return nil, err
	}
	return remnawave.New(settings, s.Config.RemnawaveURL, s.Config.RemnawaveToken), nil
}

func number(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, e := n.Int64()
		if e == nil {
			return int(i)
		}
	case string:
		i, e := strconv.Atoi(n)
		if e == nil {
			return i
		}
	}
	return fallback
}
func text(v any) string {
	if x, ok := v.(string); ok {
		return x
	}
	return ""
}
func boolean(v any) bool { x, _ := v.(bool); return x }
func object(v any) map[string]any {
	x, _ := v.(map[string]any)
	if x == nil {
		return map[string]any{}
	}
	return x
}
func stringsList(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if out, ok := v.([]string); ok {
			return out
		}
		return nil
	}
	out := []string{}
	for _, item := range raw {
		if value, ok := item.(string); ok && value != "" {
			out = append(out, value)
		}
	}
	return out
}
func mapID(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, e := n.Int64()
		return i, e == nil
	}
	return 0, false
}

func (s *Server) record(ctx context.Context, source, message string, details map[string]any) {
	s.Logger.Warn(message, "source", source)
	if err := s.Store.Diagnostic(ctx, source, message, details); err != nil {
		s.Logger.Error("store diagnostic", "error", err)
	}
}

func syncRemote(u *store.User, remote map[string]any) {
	if id, ok := mapID(remote["id"]); ok {
		u.RemnawaveUserID = &id
	}
	u.RemnawaveUserUUID = text(remote["uuid"])
	if v := text(remote["username"]); v != "" {
		u.RemnawaveUsername = v
	}
	if v := text(remote["subscriptionUrl"]); v != "" {
		u.SubscriptionURL = v
	}
	if v := text(remote["status"]); v != "" {
		u.SubscriptionStatus = v
	}
	if raw := text(remote["expireAt"]); raw != "" {
		if t, e := time.Parse(time.RFC3339Nano, raw); e == nil {
			u.ExpiresAt = &t
		}
	}
	if n, ok := mapID(remote["usedTrafficBytes"]); ok {
		u.TrafficUsedBytes = n
	}
	if n, ok := mapID(remote["trafficLimitBytes"]); ok {
		u.TrafficLimitBytes = n
	}
	if n, ok := mapID(remote["hwidDeviceLimit"]); ok {
		u.DeviceLimit = int(n)
	}
}

func (s *Server) refresh(ctx context.Context, u *store.User) {
	client, e := s.remna(ctx)
	if e != nil || !client.Configured() {
		return
	}
	remote, e := client.UserByTelegram(ctx, u.TelegramID)
	if e != nil {
		s.record(ctx, "remnawave", "Не удалось обновить подписку", map[string]any{"error": e.Error(), "telegram_id": u.TelegramID})
		return
	}
	if remote != nil {
		syncRemote(u, remote)
		if e = s.Store.UpdateSubscription(ctx, *u); e != nil {
			s.Logger.Error("update local subscription", "error", e)
		}
	}
}

func (s *Server) connectedDeviceCount(ctx context.Context, u store.User) int {
	client, err := s.remna(ctx)
	if err != nil || !client.Configured() || (u.RemnawaveUserID == nil && u.RemnawaveUserUUID == "") {
		return 0
	}
	remote := map[string]any{"uuid": u.RemnawaveUserUUID}
	if u.RemnawaveUserID != nil {
		remote["id"] = *u.RemnawaveUserID
	}
	devices, err := client.Devices(ctx, remote)
	if err != nil {
		return 0
	}
	return len(devices)
}

type trafficMode string

const (
	trafficKeep      trafficMode = "keep"
	trafficAdd       trafficMode = "add"
	trafficPlan      trafficMode = "plan"
	trafficUnlimited trafficMode = "unlimited"
)

type entitlement struct {
	Days              int
	TrafficGB         int
	TrafficMode       trafficMode
	DeviceLimit       *int
	InternalSquads    []string
	ExternalSquadUUID string
	UpdateSquads      bool
	Activate          bool
}

type entitlementPlan struct {
	Status            *string
	ExpireAt          *time.Time
	TrafficBytes      *int64
	DeviceLimit       *int
	InternalSquads    *[]string
	ExternalSquadUUID *string
}

func entitlementTraffic(current int64, status string, expires *time.Time, gb int, mode trafficMode, now time.Time) (int64, bool) {
	amount := int64(gb) * gigabyte
	active := status == "ACTIVE" && expires != nil && expires.After(now)
	switch mode {
	case trafficUnlimited:
		return 0, true
	case trafficPlan:
		if gb == 0 || (active && current == 0) {
			return 0, true
		}
		if active {
			return current + amount, true
		}
		return amount, true
	case trafficAdd:
		if gb <= 0 {
			return current, false
		}
		if active && current == 0 {
			return 0, true
		}
		return current + amount, true
	default:
		return current, false
	}
}

func prepareEntitlement(u store.User, e entitlement, now time.Time) entitlementPlan {
	plan := entitlementPlan{}
	if e.Activate {
		status := "ACTIVE"
		plan.Status = &status
	}
	if e.Days > 0 {
		base := now
		if u.ExpiresAt != nil && u.ExpiresAt.After(now) {
			base = *u.ExpiresAt
		}
		expires := base.Add(time.Duration(e.Days) * 24 * time.Hour)
		plan.ExpireAt = &expires
	}
	if traffic, update := entitlementTraffic(u.TrafficLimitBytes, u.SubscriptionStatus, u.ExpiresAt, e.TrafficGB, e.TrafficMode, now); update {
		plan.TrafficBytes = &traffic
	}
	if e.DeviceLimit != nil {
		limit := *e.DeviceLimit
		if limit < 0 {
			limit = 0
		}
		plan.DeviceLimit = &limit
	}
	if e.UpdateSquads {
		internal := append([]string(nil), e.InternalSquads...)
		external := e.ExternalSquadUUID
		plan.InternalSquads = &internal
		plan.ExternalSquadUUID = &external
	}
	return plan
}

func planMap(plan entitlementPlan) map[string]any {
	out := map[string]any{}
	if plan.Status != nil {
		out["status"] = *plan.Status
	}
	if plan.ExpireAt != nil {
		out["expire_at"] = plan.ExpireAt.Format(time.RFC3339Nano)
	}
	if plan.TrafficBytes != nil {
		out["traffic_bytes"] = strconv.FormatInt(*plan.TrafficBytes, 10)
	}
	if plan.DeviceLimit != nil {
		out["device_limit"] = *plan.DeviceLimit
	}
	if plan.InternalSquads != nil {
		out["internal_squads"] = *plan.InternalSquads
	}
	if plan.ExternalSquadUUID != nil {
		out["external_squad_uuid"] = *plan.ExternalSquadUUID
	}
	return out
}

func planFromMap(value map[string]any) (entitlementPlan, error) {
	plan := entitlementPlan{}
	if status := text(value["status"]); status != "" {
		plan.Status = &status
	}
	if raw := text(value["expire_at"]); raw != "" {
		expires, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return plan, err
		}
		plan.ExpireAt = &expires
	}
	if raw := text(value["traffic_bytes"]); raw != "" {
		traffic, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return plan, err
		}
		plan.TrafficBytes = &traffic
	}
	if _, ok := value["device_limit"]; ok {
		limit := number(value["device_limit"], 0)
		plan.DeviceLimit = &limit
	}
	if _, ok := value["internal_squads"]; ok {
		internal := stringsList(value["internal_squads"])
		plan.InternalSquads = &internal
	}
	if _, ok := value["external_squad_uuid"]; ok {
		external := text(value["external_squad_uuid"])
		plan.ExternalSquadUUID = &external
	}
	return plan, nil
}

func (s *Server) applyPreparedEntitlement(ctx context.Context, u *store.User, plan entitlementPlan) error {
	client, err := s.remna(ctx)
	if err != nil {
		return err
	}
	if !client.Configured() {
		return fmt.Errorf("Remnawave не настроен")
	}
	r := remnawave.Entitlement{TelegramID: u.TelegramID, Username: fmt.Sprintf("tgs_%d", u.TelegramID), Status: plan.Status, ExpireAt: plan.ExpireAt, TrafficBytes: plan.TrafficBytes, DeviceLimit: plan.DeviceLimit, InternalSquads: plan.InternalSquads, ExternalSquadUUID: plan.ExternalSquadUUID}
	remote, err := client.UserByTelegram(ctx, u.TelegramID)
	if err != nil {
		return err
	}
	if remote == nil {
		if plan.Status == nil || plan.ExpireAt == nil {
			return fmt.Errorf("пользователь Remnawave ещё не создан")
		}
		remote, err = client.CreateUser(ctx, r)
	} else {
		remote, err = client.UpdateUser(ctx, remote, r)
	}
	if err != nil {
		s.record(ctx, "remnawave", "Ошибка выдачи подписки", map[string]any{"error": err.Error(), "telegram_id": u.TelegramID})
		return err
	}
	if plan.ExpireAt != nil {
		u.ExpiresAt = plan.ExpireAt
	}
	if plan.TrafficBytes != nil {
		u.TrafficLimitBytes = *plan.TrafficBytes
	}
	if plan.DeviceLimit != nil {
		u.DeviceLimit = *plan.DeviceLimit
	}
	if plan.Status != nil {
		u.SubscriptionStatus = *plan.Status
	}
	syncRemote(u, remote)
	return s.Store.UpdateSubscription(ctx, *u)
}

func (s *Server) applyEntitlement(ctx context.Context, u *store.User, e entitlement) error {
	return s.Store.WithUserLock(ctx, u.ID, func() error {
		latest, err := s.Store.UserByID(ctx, u.ID)
		if err != nil {
			return err
		}
		if err = s.applyPreparedEntitlement(ctx, &latest, prepareEntitlement(latest, e, time.Now().UTC())); err != nil {
			return err
		}
		*u = latest
		return nil
	})
}

func (s *Server) completePayment(ctx context.Context, p store.Payment) error {
	if p.Status == "succeeded" {
		return nil
	}
	claimed, err := s.Store.ClaimPayment(ctx, p.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	ok := false
	defer func() {
		if !ok {
			_ = s.Store.ReleasePaymentClaim(context.Background(), p.ID)
		}
	}()
	var u store.User
	err = s.Store.WithUserLock(ctx, p.UserID, func() error {
		fresh, lockErr := s.Store.Payment(ctx, p.ID)
		if lockErr != nil {
			return lockErr
		}
		p = fresh
		u, lockErr = s.Store.UserByID(ctx, p.UserID)
		if lockErr != nil {
			return lockErr
		}
		var plan entitlementPlan
		if len(p.Entitlement) > 0 {
			plan, lockErr = planFromMap(p.Entitlement)
		} else {
			limit := number(p.Snapshot["device_limit"], 1)
			request := entitlement{Days: number(p.Snapshot["days"], 0), TrafficGB: number(p.Snapshot["traffic_gb"], 0), TrafficMode: trafficPlan, DeviceLimit: &limit, InternalSquads: stringsList(p.Snapshot["internal_squads"]), ExternalSquadUUID: text(p.Snapshot["external_squad_uuid"]), UpdateSquads: true, Activate: true}
			plan = prepareEntitlement(u, request, time.Now().UTC())
			p, lockErr = s.Store.SavePaymentEntitlement(ctx, p.ID, planMap(plan))
			if lockErr == nil {
				plan, lockErr = planFromMap(p.Entitlement)
			}
		}
		if lockErr != nil {
			return lockErr
		}
		if lockErr = s.applyPreparedEntitlement(ctx, &u, plan); lockErr != nil {
			return lockErr
		}
		return s.Store.CompleteClaimedPayment(ctx, p.ID)
	})
	if err != nil {
		return err
	}
	_ = s.Store.IncrementPromo(ctx, p.PromoCode)
	if u.ReferredByID != nil && !u.ReferralRewarded {
		sys, _ := s.Store.Setting(ctx, "system")
		if boolean(sys["reward_after_payment"]) {
			if ref, e2 := s.Store.UserByID(ctx, *u.ReferredByID); e2 == nil {
				bonusGB := number(sys["referral_traffic_gb"], 0)
				if e2 = s.applyEntitlement(ctx, &ref, entitlement{Days: number(sys["referral_days"], 0), TrafficGB: bonusGB, TrafficMode: trafficAdd, Activate: number(sys["referral_days"], 0) > 0}); e2 == nil {
					_ = s.Store.SetReferralRewarded(ctx, u.ID)
				} else {
					s.record(ctx, "referrals", "Не удалось выдать реферальный бонус", map[string]any{"error": e2.Error(), "referrer_user_id": ref.ID, "referred_user_id": u.ID})
				}
			}
		}
	}
	ok = true
	_ = s.Telegram.Send(ctx, u.TelegramID, "Оплата прошла. Подписка обновлена.", nil)
	s.notifyAdmins(ctx, fmt.Sprintf("Новая оплата: %s — %.2f ₽ (%s)", u.FirstName, float64(p.AmountKopecks)/100, p.Provider))
	return nil
}

func (s *Server) notifyAdmins(ctx context.Context, message string) {
	settings, _ := s.Store.Setting(ctx, "integrations")
	n := object(settings["notifications"])
	if boolean(n["enabled"]) && text(n["bot_token"]) != "" && text(n["chat_id"]) != "" {
		id, _ := strconv.ParseInt(text(n["chat_id"]), 10, 64)
		_ = telegram.New(text(n["bot_token"])).Send(ctx, id, message, nil)
		return
	}
	for id := range s.Config.AdminTelegramIDs {
		_ = s.Telegram.Send(ctx, id, message, nil)
	}
}

func validPromo(p store.Promo) error {
	if !p.Active {
		return fmt.Errorf("Промокод выключен")
	}
	if p.MaxUses > 0 && p.Uses >= p.MaxUses {
		return fmt.Errorf("Лимит промокода исчерпан")
	}
	if p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("Срок промокода истёк")
	}
	return nil
}
func discounted(kopecks int64, percent int) int64 {
	percent = max(0, min(100, percent))
	out := int64(math.Round(float64(kopecks) * float64(100-percent) / 100))
	if out < 100 {
		return 100
	}
	return out
}
func isNotFound(err error) bool { return errorsIs(err, sql.ErrNoRows) }
func errorsIs(err, target error) bool {
	return err == target || strings.Contains(fmt.Sprint(err), fmt.Sprint(target))
}
