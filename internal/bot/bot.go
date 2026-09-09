package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"tgs-bot/internal/config"
	"tgs-bot/internal/store"
	"tgs-bot/internal/telegram"
)

type Bot struct {
	Config   config.Config
	Store    *store.Store
	Telegram *telegram.Client
	Logger   *slog.Logger
}

func New(cfg config.Config, st *store.Store, tg *telegram.Client, logger *slog.Logger) *Bot {
	return &Bot{Config: cfg, Store: st, Telegram: tg, Logger: logger}
}

func (b *Bot) Run(ctx context.Context) {
	if err := b.Telegram.DeleteWebhook(ctx); err != nil {
		b.Logger.Warn("delete webhook", "error", err)
	}
	if err := b.Telegram.SetName(ctx, "TGS-bot"); err != nil {
		b.Logger.Warn("set bot name", "error", err)
	}
	if err := b.Telegram.SetCommands(ctx); err != nil {
		b.Logger.Warn("set bot commands", "error", err)
	}
	if err := b.Telegram.SetMenuButton(ctx, b.Config.MiniAppURL("")); err != nil {
		b.Logger.Warn("set menu button", "error", err)
	}
	var offset int64
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.Telegram.GetUpdates(ctx, offset)
		if err != nil {
			b.Logger.Warn("get updates", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 15*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message != nil {
				b.onMessage(ctx, *u.Message)
			} else if u.CallbackQuery != nil {
				b.onCallback(ctx, *u.CallbackQuery)
			}
		}
	}
}

func (b *Bot) telegramUser(u telegram.User) store.TelegramUser {
	return store.TelegramUser{ID: u.ID, Username: u.Username, FirstName: u.FirstName, LanguageCode: u.LanguageCode}
}
func (b *Bot) onMessage(ctx context.Context, m telegram.Message) {
	parts := strings.Fields(m.Text)
	if len(parts) == 0 {
		return
	}
	ref := ""
	if strings.HasPrefix(parts[0], "/start") && len(parts) > 1 {
		ref = parts[1]
	}
	u, err := b.Store.UpsertTelegramUser(ctx, b.telegramUser(m.From), b.Config.AdminTelegramIDs, ref)
	if err != nil {
		b.Logger.Error("upsert bot user", "error", err)
		return
	}
	command := strings.Split(parts[0], "@")[0]
	switch command {
	case "/start":
		b.start(ctx, m.Chat.ID, u)
	case "/myid":
		_ = b.Telegram.Send(ctx, m.Chat.ID, fmt.Sprintf("Ваш Telegram ID: %d", m.From.ID), nil)
	case "/broadcast":
		b.broadcastDraft(ctx, m, u)
	}
}

func str(m map[string]any, key, fallback string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
func yes(m map[string]any, key string) bool { v, _ := m[key].(bool); return v }
func (b *Bot) start(ctx context.Context, chatID int64, u store.User) {
	content, _ := b.Store.Setting(ctx, "content")
	emergency, _ := b.Store.Setting(ctx, "emergency")
	if yes(emergency, "enabled") && !u.IsAdmin {
		_ = b.Telegram.Send(ctx, chatID, str(content, "emergency_message", "Сервис временно недоступен."), nil)
		return
	}
	features, _ := b.Store.Setting(ctx, "features")
	trial, _ := b.Store.Setting(ctx, "trial")
	rows := []any{}
	if yes(features, "trial") && yes(trial, "enabled") && !u.TrialUsed {
		rows = append(rows, []any{telegram.WebAppButton(str(content, "trial_button", "Бесплатный период"), b.Config.MiniAppURL("trial"))})
	}
	rows = append(rows, []any{telegram.WebAppButton(str(content, "cabinet_button", "Личный кабинет"), b.Config.MiniAppURL("home"))})
	if yes(features, "support") {
		rows = append(rows, []any{telegram.WebAppButton(str(content, "support_button", "Поддержка"), b.Config.MiniAppURL("support"))})
	}
	markup := map[string]any{"inline_keyboard": rows}
	message := str(content, "start_title", "Добро пожаловать в TGS VPN") + "\n\n" + str(content, "start_text", "Управляйте подпиской в Mini App.")
	_ = b.Telegram.Send(ctx, chatID, message, markup)
}

func (b *Bot) broadcastDraft(ctx context.Context, m telegram.Message, u store.User) {
	if !u.IsAdmin {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Команда доступна только администратору.", nil)
		return
	}
	raw := strings.TrimSpace(strings.TrimPrefix(m.Text, strings.Fields(m.Text)[0]))
	if raw == "" {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Формат:\n/broadcast Текст сообщения\nНазвание кнопки | https://example.com\n\nСтроки с «| https://» станут кнопками.", nil)
		return
	}
	lines := strings.Split(raw, "\n")
	messageLines := []string{}
	buttons := []map[string]string{}
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			label := strings.TrimSpace(parts[0])
			link := strings.TrimSpace(parts[1])
			if parsed, err := url.ParseRequestURI(link); err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && label != "" {
				buttons = append(buttons, map[string]string{"text": label, "url": link})
				continue
			}
		}
		messageLines = append(messageLines, line)
	}
	message := strings.TrimSpace(strings.Join(messageLines, "\n"))
	if message == "" {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Текст рассылки пуст.", nil)
		return
	}
	draft, err := b.Store.CreateBroadcast(ctx, u.ID, message, buttons)
	if err != nil {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Не удалось создать черновик.", nil)
		return
	}
	previewRows := []any{}
	for _, button := range buttons {
		previewRows = append(previewRows, []any{telegram.URLButton(button["text"], button["url"])})
	}
	previewRows = append(previewRows, []any{telegram.CallbackButton("Отправить всем", "broadcast:send:"+draft.ID), telegram.CallbackButton("Отмена", "broadcast:cancel:"+draft.ID)})
	_ = b.Telegram.Send(ctx, m.Chat.ID, "Предпросмотр рассылки:\n\n"+message, map[string]any{"inline_keyboard": previewRows})
}

func (b *Bot) onCallback(ctx context.Context, q telegram.CallbackQuery) {
	u, err := b.Store.UserByTelegram(ctx, q.From.ID)
	if err != nil || !u.IsAdmin {
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Недостаточно прав")
		return
	}
	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 || parts[0] != "broadcast" {
		return
	}
	draft, err := b.Store.Broadcast(ctx, parts[2])
	if err != nil || draft.AdminUserID != u.ID || draft.Status != "draft" {
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Черновик недоступен")
		return
	}
	if parts[1] == "cancel" {
		_ = b.Store.FinishBroadcast(ctx, draft.ID, "cancelled", 0, 0)
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Рассылка отменена")
		return
	}
	_ = b.Telegram.AnswerCallback(ctx, q.ID, "Рассылка запущена")
	go b.sendBroadcast(context.Background(), q.Message.Chat.ID, draft)
}

func (b *Bot) sendBroadcast(ctx context.Context, adminChat int64, draft store.Broadcast) {
	ids, err := b.Store.AllTelegramIDs(ctx)
	if err != nil {
		_ = b.Telegram.Send(ctx, adminChat, "Не удалось получить список пользователей.", nil)
		return
	}
	rows := []any{}
	for _, button := range draft.Buttons {
		rows = append(rows, []any{telegram.URLButton(button["text"], button["url"])})
	}
	var markup any
	if len(rows) > 0 {
		markup = map[string]any{"inline_keyboard": rows}
	}
	sent, failed := 0, 0
	_ = b.Store.FinishBroadcast(ctx, draft.ID, "sending", 0, 0)
	for _, id := range ids {
		if err = b.Telegram.Send(ctx, id, draft.Text, markup); err != nil {
			failed++
		} else {
			sent++
		}
		time.Sleep(40 * time.Millisecond)
	}
	_ = b.Store.FinishBroadcast(ctx, draft.ID, "completed", sent, failed)
	_ = b.Telegram.Send(ctx, adminChat, fmt.Sprintf("Рассылка завершена. Доставлено: %d, ошибок: %d.", sent, failed), nil)
}
