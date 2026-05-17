package services

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"ipms/internal/models"
)

// Hub manages all connected WebSocket clients
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

func (h *Hub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
	log.Printf("[WS] Client connected — total: %d", h.Count())

	// Send welcome message
	h.sendTo(conn, &models.WSMessage{
		Type: "connected",
		Data: map[string]string{"message": "IPMS real-time feed active"},
		TS:   time.Now().Format(time.RFC3339),
	})
}

func (h *Hub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
	conn.Close()
	log.Printf("[WS] Client disconnected — total: %d", h.Count())
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msgType string, data interface{}) {
	msg := &models.WSMessage{
		Type: msgType,
		Data: data,
		TS:   time.Now().Format(time.RFC3339),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	dead := []*websocket.Conn{}
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			dead = append(dead, conn)
		}
	}
	h.mu.RUnlock()

	// Clean up dead connections
	if len(dead) > 0 {
		h.mu.Lock()
		for _, conn := range dead {
			delete(h.clients, conn)
			conn.Close()
		}
		h.mu.Unlock()
	}
}

func (h *Hub) sendTo(conn *websocket.Conn, msg *models.WSMessage) {
	payload, _ := json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, payload)
}
