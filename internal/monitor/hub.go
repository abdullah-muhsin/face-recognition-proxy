package monitor

import (
	"encoding/json"
	"sync"
	"time"
)

// Event is intentionally metadata-only. Protocol bodies can contain biometric
// data or credentials; they remain neither a database ledger nor a browser feed.
type Event struct {
	At       time.Time      `json:"at"`
	Kind     string         `json:"kind"`
	Terminal string         `json:"terminal,omitempty"`
	Message  string         `json:"message"`
	Fields   map[string]any `json:"fields,omitempty"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[chan []byte]struct{})} }

func (h *Hub) Publish(event Event) {
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client <- payload:
		default:
		}
	}
}

func (h *Hub) Subscribe() (<-chan []byte, func()) {
	client := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return client, func() { once.Do(func() { h.mu.Lock(); delete(h.clients, client); close(client); h.mu.Unlock() }) }
}
