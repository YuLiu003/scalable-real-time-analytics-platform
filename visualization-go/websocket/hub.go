package websocket

import (
	"log"
	"sync"

	"visualization-go/metrics"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Channel for registering new clients
	register chan *Client

	// Channel for unregistering clients
	unregister chan *Client

	// Channel for broadcasting messages to clients
	broadcast chan []byte

	// Mutex for thread-safe operations
	mu sync.Mutex
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
	}
}

// Run starts the hub, handling client connections and message broadcasting
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			metrics.WebSocketConnections.Inc()
			log.Printf("Client connected, total connections: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				metrics.WebSocketConnections.Dec()
				log.Printf("Client disconnected, total connections: %d", len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
					metrics.WebSocketMessages.Inc()
				default:
					close(client.send)
					delete(h.clients, client)
					metrics.WebSocketConnections.Dec()
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastMessage sends a message to all connected clients
func (h *Hub) BroadcastMessage(message []byte) {
	h.broadcast <- message
}
