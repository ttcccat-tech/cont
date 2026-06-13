package storage

import (
	"encoding/json"
	"sync"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID     string
	UserID string
	Chan   chan string
}

// SSEHub manages all SSE client connections
type SSEHub struct {
	clients    map[string]SSEClient
	register   chan SSEClient
	unregister chan string
	broadcast  chan string
	mu         sync.RWMutex
}

// Global SSE hub instance
var Hub = &SSEHub{
	clients:    make(map[string]SSEClient),
	register:   make(chan SSEClient, 100),
	unregister: make(chan string, 100),
	broadcast:  make(chan string, 1000),
}

// Run starts the SSE hub event loop
func (h *SSEHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()

		case clientID := <-h.unregister:
			h.mu.Lock()
			if client, ok := h.clients[clientID]; ok {
				close(client.Chan)
				delete(h.clients, clientID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.Chan <- message:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the hub
func (h *SSEHub) Register(client SSEClient) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *SSEHub) Unregister(clientID string) {
	h.unregister <- clientID
}

// BroadcastToUser sends an event to all connections of a specific user
func (h *SSEHub) BroadcastToUser(userID, eventType string, data interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	msg := "event: " + eventType + "\ndata: " + string(payload) + "\n\n"
	h.mu.RLock()
	for _, client := range h.clients {
		if client.UserID == userID {
			select {
			case client.Chan <- msg:
			default:
			}
		}
	}
	h.mu.RUnlock()
}

// BroadcastAll sends an event to all connected clients (admin broadcast)
func (h *SSEHub) BroadcastAll(eventType string, data interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	msg := "event: " + eventType + "\ndata: " + string(payload) + "\n\n"
	h.broadcast <- msg
}

func init() {
	go Hub.Run()
}
