package bot

import (
	"context"
	"fmt"
	"log/slog"
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
	ref := ""
	if len(parts) > 1 && strings.HasPrefix(parts[0], "/start") {
		ref = parts[1]
	}
	u, err := b.Store.UpsertTelegramUser(ctx, b.telegramUser(m.From), b.Config.AdminTelegramIDs, ref)
	if err != nil {
		b.Logger.Error("upsert bot user", "error", err)
		return
	}
	if len(parts) > 0 && strings.HasPrefix(parts[0], "/") {
		command := strings.Split(parts[0], "@")[0]
		switch command {
		case "/start":
			b.start(ctx, m.Chat.ID, u)
		case "/myid":
			_ = b.Telegram.Send(ctx, m.Chat.ID, fmt.Sprintf("Ваш Telegram ID: %d", m.From.ID), nil)
		case "/broadcast":
			b.broadcastDraft(ctx, m, u)
		}
		return
	}
	if u.IsAdmin {
		b.captureBroadcast(ctx, m, u)
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
		_ = b.Telegram.Send(ctx, chatID, str(emergency, "message", str(content, "emergency_message", "Сервис временно недоступен.")), nil)
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
	_, err := b.Store.BeginBroadcastDraft(ctx, u.ID)
	if err != nil {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Не удалось создать черновик.", nil)
		return
	}
	_ = b.Telegram.Send(ctx, m.Chat.ID, "Отправьте следующим сообщением материал для рассылки. Фото, форматирование и премиум-эмодзи сохранятся.", nil)
}

func (b *Bot) captureBroadcast(ctx context.Context, m telegram.Message, u store.User) {
	draft, err := b.Store.AwaitingBroadcast(ctx, u.ID)
	if err != nil {
		return
	}
	summary := strings.TrimSpace(m.Text)
	if summary == "" {
		summary = strings.TrimSpace(m.Caption)
	}
	if summary == "" {
		summary = "Медиа-сообщение"
	}
	draft, err = b.Store.CaptureBroadcast(ctx, draft.ID, m.Chat.ID, m.MessageID, summary)
	if err != nil {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Не удалось сохранить сообщение. Попробуйте ещё раз.", nil)
		return
	}
	markup := map[string]any{"inline_keyboard": []any{
		[]any{telegram.CallbackButton("Подтвердить", "broadcast:confirm:"+draft.ID)},
		[]any{telegram.CallbackButton("Изменить", "broadcast:edit:"+draft.ID)},
	}}
	if _, err = b.Telegram.CopyMessage(ctx, m.Chat.ID, m.Chat.ID, m.MessageID, markup); err != nil {
		_ = b.Telegram.Send(ctx, m.Chat.ID, "Сообщение сохранено, но предпросмотр не создался. Нажмите «Изменить» в Mini App и повторите.", nil)
	}
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
	if err != nil || draft.AdminUserID != u.ID {
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Черновик недоступен")
		return
	}
	if parts[1] == "edit" {
		if draft.Status != "preview" && draft.Status != "confirmed" {
			_ = b.Telegram.AnswerCallback(ctx, q.ID, "Черновик уже изменён")
			return
		}
		_ = b.Store.SetBroadcastStatus(ctx, draft.ID, "awaiting")
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Отправьте новое сообщение")
		_ = b.Telegram.Send(ctx, q.Message.Chat.ID, "Отправьте новый вариант одним сообщением.", nil)
		return
	}
	if parts[1] != "confirm" || draft.Status != "preview" {
		_ = b.Telegram.AnswerCallback(ctx, q.ID, "Действие недоступно")
		return
	}
	_ = b.Store.SetBroadcastStatus(ctx, draft.ID, "confirmed")
	_ = b.Telegram.AnswerCallback(ctx, q.ID, "Сообщение подтверждено")
	markup := map[string]any{"inline_keyboard": []any{[]any{telegram.WebAppButton("Вернуться к рассылке", b.Config.MiniAppURL("admin:broadcast"))}}}
	_ = b.Telegram.Send(ctx, q.Message.Chat.ID, "Готово. Теперь добавьте кнопки, выполните тест и запустите рассылку в Mini App.", markup)
}
