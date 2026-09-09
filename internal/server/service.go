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

type entitlement struct {
	Days, TrafficGB, DeviceLimit int
	InternalSquads               []string
	ExternalSquadUUID            string
}

func (s *Server) applyEntitlement(ctx context.Context, u *store.User, e entitlement) error {
	client, err := s.remna(ctx)
	if err != nil {
		return err
	}
	if !client.Configured() {
		return fmt.Errorf("Remnawave не настроен")
	}
	now := time.Now().UTC()
	base := now
	if u.ExpiresAt != nil && u.ExpiresAt.After(now) {
		base = *u.ExpiresAt
	}
	expire := base.Add(time.Duration(e.Days) * 24 * time.Hour)
	traffic := u.TrafficLimitBytes + int64(e.TrafficGB)*gigabyte
	if e.TrafficGB == 0 || (u.TrafficLimitBytes == 0 && u.SubscriptionStatus == "ACTIVE") {
		traffic = 0
	}
	if e.DeviceLimit < 0 {
		e.DeviceLimit = 0
	}
	r := remnawave.Entitlement{TelegramID: u.TelegramID, Username: fmt.Sprintf("tgs_%d", u.TelegramID), ExpireAt: expire, TrafficBytes: traffic, DeviceLimit: e.DeviceLimit, InternalSquads: e.InternalSquads, ExternalSquadUUID: e.ExternalSquadUUID}
	remote, err := client.UserByTelegram(ctx, u.TelegramID)
	if err != nil {
		return err
	}
	if remote == nil {
		remote, err = client.CreateUser(ctx, r)
	} else {
		remote, err = client.UpdateUser(ctx, remote, r)
	}
	if err != nil {
		s.record(ctx, "remnawave", "Ошибка выдачи подписки", map[string]any{"error": err.Error(), "telegram_id": u.TelegramID})
		return err
	}
	u.ExpiresAt = &expire
	u.TrafficLimitBytes = traffic
	u.DeviceLimit = e.DeviceLimit
	u.SubscriptionStatus = "ACTIVE"
	syncRemote(u, remote)
	return s.Store.UpdateSubscription(ctx, *u)
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
	u, err := s.Store.UserByID(ctx, p.UserID)
	if err != nil {
		return err
	}
	e := entitlement{Days: number(p.Snapshot["days"], 0), TrafficGB: number(p.Snapshot["traffic_gb"], 0), DeviceLimit: number(p.Snapshot["device_limit"], 1), InternalSquads: stringsList(p.Snapshot["internal_squads"]), ExternalSquadUUID: text(p.Snapshot["external_squad_uuid"])}
	if err = s.applyEntitlement(ctx, &u, e); err != nil {
		return err
	}
	if err = s.Store.CompleteClaimedPayment(ctx, p.ID); err != nil {
		return err
	}
	_ = s.Store.IncrementPromo(ctx, p.PromoCode)
	if u.ReferredByID != nil && !u.ReferralRewarded {
		sys, _ := s.Store.Setting(ctx, "system")
		if boolean(sys["reward_after_payment"]) {
			if ref, e2 := s.Store.UserByID(ctx, *u.ReferredByID); e2 == nil {
				if e2 = s.applyEntitlement(ctx, &ref, entitlement{Days: number(sys["referral_days"], 0), TrafficGB: number(sys["referral_traffic_gb"], 0), DeviceLimit: ref.DeviceLimit}); e2 == nil {
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
