package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan []byte

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client
}

// Client is a middleman between the websocket connection and the hub
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte
}

// newHub creates a new Hub
func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// run starts the hub
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// broadcastMessage sends a message to all registered clients
func (h *Hub) broadcastMessage(data interface{}) {
	message, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling broadcast message: %v", err)
		return
	}

	select {
	case h.broadcast <- message:
	default:
		// Channel is full, skip this broadcast
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing websocket connection: %v", err)
		}
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("Error setting read deadline: %v", err)
		return
	}
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Printf("Error setting read deadline in pong handler: %v", err)
		}
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}


// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if err := c.conn.Close(); err != nil {
			log.Printf("Error closing websocket connection: %v", err)
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.handleChannelClose()
				return
			}
			if !c.writeMessage(message) {
				return
			}

		case <-ticker.C:
			if !c.writePing() {
				return
			}
		}
	}
}

// handleChannelClose handles the case when the send channel is closed
func (c *Client) handleChannelClose() {
	if err := c.conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
		log.Printf("Error writing close message: %v", err)
	}
}

// writeMessage writes a message and any queued messages to the connection
func (c *Client) writeMessage(message []byte) bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		log.Printf("Error setting write deadline: %v", err)
		return false
	}

	w, err := c.conn.NextWriter(websocket.TextMessage)
	if err != nil {
		return false
	}

	if !c.writeMessageAndQueued(w, message) {
		if err := w.Close(); err != nil {
			log.Printf("Error closing writer: %v", err)
		}
		return false
	}

	if err := w.Close(); err != nil {
		log.Printf("Error closing writer: %v", err)
		return false
	}

	return true
}

// writeMessageAndQueued writes the message and any queued messages
func (c *Client) writeMessageAndQueued(w interface {
	Write([]byte) (int, error)
}, message []byte) bool {
	if _, err := w.Write(message); err != nil {
		log.Printf("Error writing message: %v", err)
		return false
	}

	// Add queued messages to the current websocket message
	n := len(c.send)
	for i := 0; i < n; i++ {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			log.Printf("Error writing newline: %v", err)
			return false
		}
		if _, err := w.Write(<-c.send); err != nil {
			log.Printf("Error writing queued message: %v", err)
			return false
		}
	}

	return true
}

// writePing sends a ping message to keep the connection alive
func (c *Client) writePing() bool {
	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return false
	}
	if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		return false
	}
	return true
}

// wsHandler handles websocket requests from the peer
func (s *Server) wsHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  s.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
