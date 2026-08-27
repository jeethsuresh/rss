package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Event struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
}

type EventHub struct {
	mu          sync.Mutex
	nextID      uint64
	subscribers map[uint64]chan Event
}

func NewEventHub() *EventHub {
	return &EventHub{subscribers: map[uint64]chan Event{}}
}

// Emit deliberately reduces tenant-sensitive article and story events to broad
// invalidations. The browser then reloads its authorized view through /v1/rpc.
func (h *EventHub) Emit(name string, payload any) {
	event := Event{Event: name, Payload: payload}
	switch name {
	case "article.updated", "article.removed":
		event = Event{Event: "articles.added", Payload: map[string]any{}}
	case "feed.updated", "feed.error", "articles.added":
		event.Payload = map[string]any{}
	case "story.updated", "ai.status", "ai.log":
		event.Payload = map[string]any{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, subscriber := range h.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (h *EventHub) subscribe() (<-chan Event, func()) {
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	ch := make(chan Event, 64)
	h.subscribers[id] = ch
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subscribers, id)
		h.mu.Unlock()
	}
}

func (a *API) eventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "STREAM_UNAVAILABLE", fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	events, unsubscribe := a.events.subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-r.Context().Done():
			return
		case event := <-events:
			encoded, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
