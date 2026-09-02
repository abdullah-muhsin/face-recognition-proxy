package monitor

import (
	"encoding/json"
	"sync"

	"github.com/itplus/pushsdk-gateway/internal/activity"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub { return &Hub{clients: make(map[chan []byte]struct{})} }

// Publish broadcasts an activity record after it has been committed to
// PostgreSQL. Raw protocol bodies never enter this metadata-only channel.
func (h *Hub) Publish(event activity.Event) {
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
