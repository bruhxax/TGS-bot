package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type liveEvent struct {
	Type         string `json:"type"`
	TicketID     string `json:"ticket_id,omitempty"`
	userID       int64
	notifyAdmins bool
}

type liveSubscriber struct {
	userID  int64
	isAdmin bool
	events  chan liveEvent
}

type liveHub struct {
	mu          sync.RWMutex
	subscribers map[*liveSubscriber]struct{}
}

func newLiveHub() *liveHub {
	return &liveHub{subscribers: make(map[*liveSubscriber]struct{})}
}

func (h *liveHub) subscribe(userID int64, isAdmin bool) (*liveSubscriber, func()) {
	subscriber := &liveSubscriber{userID: userID, isAdmin: isAdmin, events: make(chan liveEvent, 16)}
	h.mu.Lock()
	h.subscribers[subscriber] = struct{}{}
	h.mu.Unlock()
	return subscriber, func() {
		h.mu.Lock()
		delete(h.subscribers, subscriber)
		h.mu.Unlock()
	}
}

func (h *liveHub) publish(event liveEvent) {
	if h == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for subscriber := range h.subscribers {
		if event.userID != 0 && subscriber.userID != event.userID && !(event.notifyAdmins && subscriber.isAdmin) {
			continue
		}
		select {
		case subscriber.events <- event:
		default:
			// A fresh bootstrap or ticket fetch supersedes queued updates.
		}
	}
}

func (s *Server) publishAccount(userID int64) {
	if s.Events != nil {
		s.Events.publish(liveEvent{Type: "account", userID: userID})
	}
}

func (s *Server) publishBootstrap() {
	if s.Events != nil {
		s.Events.publish(liveEvent{Type: "bootstrap"})
	}
}

func (s *Server) publishSupport(userID int64, ticketID string) {
	if s.Events != nil {
		s.Events.publish(liveEvent{Type: "support", TicketID: ticketID, userID: userID, notifyAdmins: true})
	}
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusNotImplemented, "Поток обновлений недоступен")
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	u := current(r)
	subscriber, unsubscribe := s.Events.subscribe(u.ID, u.IsAdmin)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(w, "retry: 1500\ndata: {\"type\":\"ready\"}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-subscriber.events:
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			if _, err = fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
