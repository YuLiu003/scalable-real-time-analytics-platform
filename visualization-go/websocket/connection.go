package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections - for production, you should implement proper origin checks
		return true
	},
}

// ServeWs handles websocket requests from clients
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// Log connection attempt
	log.Printf("WebSocket connection attempt from %s", r.RemoteAddr)

	// Extract tenant ID from query parameters
	tenantID := r.URL.Query().Get("tenant")
	if tenantID == "" {
		tenantID = "default"
	}

	log.Printf("WebSocket connection with tenant ID: %s", tenantID)

	// Set up response headers for better compatibility
	responseHeader := http.Header{}
	responseHeader.Add("Access-Control-Allow-Origin", "*")
	responseHeader.Add("Access-Control-Allow-Methods", "GET, OPTIONS")
	responseHeader.Add("Access-Control-Allow-Headers", "Content-Type")

	// Upgrade the connection
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	// Create the client with tenant information
	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		tenantID: tenantID,
	}

	// Register with the hub
	client.hub.register <- client

	log.Printf("WebSocket connection established for tenant %s from %s", tenantID, r.RemoteAddr)

	// Allow collection of memory referenced by the caller by doing all work in new goroutines.
	go client.writePump()
	go client.readPump()
}
