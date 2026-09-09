package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"tgs-bot/internal/store"
	"tgs-bot/internal/telegram"
)

var customEmojiPattern = regexp.MustCompile(`^[0-9]{1,32}$`)

func (s *Server) adminBroadcast(w http.ResponseWriter, r *http.Request) {
	draft, err := s.Store.LatestBroadcast(r.Context(), current(r).ID)
	if err != nil {
		if store.IsNotFound(err) {
			writeJSON(w, 200, map[string]any{"draft": nil, "bot_url": "https://t.me/" + s.BotUsername})
			return
		}
		writeError(w, 500, "Не удалось загрузить рассылку")
		return
	}
	writeJSON(w, 200, map[string]any{"draft": draft, "bot_url": "https://t.me/" + s.BotUsername})
}

func (s *Server) adminBroadcastDraft(w http.ResponseWriter, r *http.Request) {
	u := current(r)
	draft, err := s.Store.BeginBroadcastDraft(r.Context(), u.ID)
	if err != nil {
		writeError(w, 500, "Не удалось создать черновик")
		return
	}
	if err = s.sendContentMessage(r.Context(), u.TelegramID, "broadcast_prompt_message", "Отправьте следующим сообщением материал для рассылки. Фото, форматирование и премиум-эмодзи сохранятся.", nil, nil); err != nil {
		writeError(w, 502, "Не удалось открыть ввод в Telegram")
		return
	}
	writeJSON(w, 200, map[string]any{"draft": draft, "bot_url": "https://t.me/" + s.BotUsername})
}

func validateBroadcastButtons(buttons []map[string]string) error {
	if len(buttons) > 8 {
		return fmt.Errorf("можно добавить не больше 8 кнопок")
	}
	for _, button := range buttons {
		label := strings.TrimSpace(button["text"])
		if len([]rune(label)) < 1 || len([]rune(label)) > 64 {
			return fmt.Errorf("название кнопки: 1–64 символа")
		}
		link, err := url.ParseRequestURI(strings.TrimSpace(button["url"]))
		if err != nil || (link.Scheme != "http" && link.Scheme != "https") {
			return fmt.Errorf("укажите корректную ссылку кнопки")
		}
		style := button["style"]
		if style != "" && style != "primary" && style != "success" && style != "danger" {
			return fmt.Errorf("неизвестный цвет кнопки")
		}
		if emoji := strings.TrimSpace(button["icon_custom_emoji_id"]); emoji != "" && !customEmojiPattern.MatchString(emoji) {
			return fmt.Errorf("некорректный ID премиум-эмодзи")
		}
	}
	return nil
}

func (s *Server) adminBroadcastButtons(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Buttons []map[string]string `json:"buttons"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := validateBroadcastButtons(body.Buttons); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	draft, err := s.Store.Broadcast(r.Context(), r.PathValue("id"))
	if err != nil || draft.AdminUserID != current(r).ID {
		writeError(w, 404, "Черновик не найден")
		return
	}
	for _, button := range body.Buttons {
		button["text"] = strings.TrimSpace(button["text"])
		button["url"] = strings.TrimSpace(button["url"])
		button["icon_custom_emoji_id"] = strings.TrimSpace(button["icon_custom_emoji_id"])
	}
	draft, err = s.Store.SaveBroadcastButtons(r.Context(), draft.ID, body.Buttons)
	if err != nil {
		writeError(w, 409, "Сначала подтвердите сообщение в Telegram")
		return
	}
	writeJSON(w, 200, draft)
}

func broadcastMarkup(buttons []map[string]string) any {
	if len(buttons) == 0 {
		return nil
	}
	rows := make([]any, 0, len(buttons))
	for _, button := range buttons {
		rows = append(rows, []any{telegram.StyledURLButton(button["text"], button["url"], button["style"], button["icon_custom_emoji_id"])})
	}
	return map[string]any{"inline_keyboard": rows}
}

func (s *Server) adminBroadcastTest(w http.ResponseWriter, r *http.Request) {
	draft, err := s.Store.Broadcast(r.Context(), r.PathValue("id"))
	if err != nil || draft.AdminUserID != current(r).ID || draft.SourceMessageID == 0 {
		writeError(w, 404, "Подтверждённое сообщение не найдено")
		return
	}
	if _, err = s.Telegram.CopyMessage(r.Context(), current(r).TelegramID, draft.SourceChatID, draft.SourceMessageID, broadcastMarkup(draft.Buttons)); err != nil {
		s.record(r.Context(), "broadcast", "Тестовая рассылка не отправлена", map[string]any{"error": err.Error(), "broadcast_id": draft.ID})
		writeError(w, 502, "Не удалось отправить тест")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) adminBroadcastSend(w http.ResponseWriter, r *http.Request) {
	draft, err := s.Store.Broadcast(r.Context(), r.PathValue("id"))
	if err != nil || draft.AdminUserID != current(r).ID || draft.SourceMessageID == 0 {
		writeError(w, 404, "Подтверждённое сообщение не найдено")
		return
	}
	claimed, err := s.Store.ClaimBroadcast(r.Context(), draft.ID)
	if err != nil {
		writeError(w, 500, "Не удалось запустить рассылку")
		return
	}
	if !claimed {
		writeError(w, 409, "Подтвердите сообщение в Telegram или дождитесь текущей рассылки")
		return
	}
	go s.sendBroadcastCopies(context.Background(), current(r).TelegramID, draft)
	writeJSON(w, 202, map[string]any{"ok": true, "status": "sending"})
}

func (s *Server) sendBroadcastCopies(ctx context.Context, adminChatID int64, draft store.Broadcast) {
	if err := s.Store.PrepareBroadcastDeliveries(ctx, draft.ID); err != nil {
		_ = s.Store.FinishBroadcast(ctx, draft.ID, "failed", 0, 0)
		s.sendContentMessage(ctx, adminChatID, "broadcast_start_error_message", "Рассылка не запущена: не удалось получить пользователей.", nil, nil)
		return
	}
	markup := broadcastMarkup(draft.Buttons)
	for {
		deliveries, err := s.Store.ClaimBroadcastDeliveries(ctx, draft.ID)
		if err != nil {
			s.record(ctx, "broadcast", "Не удалось получить очередь рассылки", map[string]any{"error": err.Error(), "broadcast_id": draft.ID})
			return
		}
		if len(deliveries) == 0 {
			break
		}
		for _, delivery := range deliveries {
			_, sendErr := s.Telegram.CopyMessage(ctx, delivery.ChatID, draft.SourceChatID, draft.SourceMessageID, markup)
			message := ""
			if sendErr != nil {
				message = sendErr.Error()
				if len(message) > 500 {
					message = message[:500]
				}
			}
			_ = s.Store.FinishBroadcastDelivery(ctx, draft.ID, delivery.ChatID, sendErr == nil, message)
			time.Sleep(40 * time.Millisecond)
		}
	}
	sent, failed, remaining, err := s.Store.BroadcastDeliveryCounts(ctx, draft.ID)
	if err != nil || remaining > 0 {
		return
	}
	_ = s.Store.FinishBroadcast(ctx, draft.ID, "completed", sent, failed)
	s.sendContentMessage(ctx, adminChatID, "broadcast_complete_message", "Рассылка завершена. Доставлено: <b>{sent}</b>, ошибок: <b>{failed}</b>.", map[string]string{"sent": fmt.Sprint(sent), "failed": fmt.Sprint(failed)}, nil)
}
