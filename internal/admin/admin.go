package admin

import (
	"encoding/json"
	"net/http"
	"sync"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/helpers"

	"github.com/gorilla/websocket"
)

// LogHub maintains the set of active clients and broadcasts messages to clients.
type LogHub struct {
	// Registered clients.
	clients map[*websocket.Conn]bool

	// Inbound log messages from helpers.
	broadcast chan map[string]any

	// Register requests from the clients.
	register chan *websocket.Conn

	// Unregister requests from clients.
	unregister chan *websocket.Conn

	// Mutex to protect the map during heavy concurrency
	mu sync.Mutex
}

// Global instance
var Hub = &LogHub{
	broadcast:  make(chan map[string]any, 256), // Buffer 256 logs
	register:   make(chan *websocket.Conn),
	unregister: make(chan *websocket.Conn),
	clients:    make(map[*websocket.Conn]bool),
}

// Upgrader config
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now (adjust for production security)
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Run starts the Hub loop. Call this once in main.go.
func (h *LogHub) Run() {
	// Hook into the helpers package
	helpers.LogStreamHook = func(entry map[string]any) {
		// Non-blocking send! If channel is full (no consumers or slow), drop the log.
		// We strictly avoid blocking the main application flow for logging.
		select {
		case h.broadcast <- entry:
		default:
			// Buffer full, dropping log for WS to prevent app freeze
		}
	}

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Marshal JSON once
			data, err := json.Marshal(message)
			if err != nil {
				continue
			}

			h.mu.Lock()
			for client := range h.clients {
				// Write JSON to websocket
				err := client.WriteMessage(websocket.TextMessage, data)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// HandleLogs is the HTTP Handler for /admin/logs
func (h *LogHub) HandleLogs(w http.ResponseWriter, r *http.Request) {
	// 1. Auth Check (LoggedIn user only)
	_, ok := auth.GetUserIdFromContext(w, r)
	if !ok {
		// Error response handled by helper inside GetUserId... if not, send one:
		// http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Upgrade HTTP to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		helpers.LogError("LogHub", "Failed to upgrade WS", "error", err.Error())
		return
	}

	// 3. Register Client
	h.register <- conn

	// Send a welcome message
	conn.WriteJSON(map[string]string{"msg": "Connected to Log Stream"})

	// 4. Keep connection alive until client disconnects
	// Since this is one-way (server -> client), we just read until error (close)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	// 5. Cleanup
	h.unregister <- conn
}
