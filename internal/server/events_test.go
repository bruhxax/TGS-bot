package server

import (
	"testing"
	"time"
)

func TestLiveHubRoutesTicketEventsToOwnerAndAdmins(t *testing.T) {
	hub := newLiveHub()
	owner, unsubscribeOwner := hub.subscribe(11, false)
	defer unsubscribeOwner()
	admin, unsubscribeAdmin := hub.subscribe(22, true)
	defer unsubscribeAdmin()
	other, unsubscribeOther := hub.subscribe(33, false)
	defer unsubscribeOther()

	hub.publish(liveEvent{Type: "support", TicketID: "ticket-1", userID: 11, notifyAdmins: true})
	assertLiveEvent(t, owner.events, "support")
	assertLiveEvent(t, admin.events, "support")
	select {
	case event := <-other.events:
		t.Fatalf("unrelated user received event: %#v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestLiveHubBroadcastsBootstrapEvents(t *testing.T) {
	hub := newLiveHub()
	first, unsubscribeFirst := hub.subscribe(1, false)
	defer unsubscribeFirst()
	second, unsubscribeSecond := hub.subscribe(2, true)
	defer unsubscribeSecond()

	hub.publish(liveEvent{Type: "bootstrap"})
	assertLiveEvent(t, first.events, "bootstrap")
	assertLiveEvent(t, second.events, "bootstrap")
}

func assertLiveEvent(t *testing.T, events <-chan liveEvent, eventType string) {
	t.Helper()
	select {
	case event := <-events:
		if event.Type != eventType {
			t.Fatalf("event type = %q; want %q", event.Type, eventType)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %q event", eventType)
	}
}
