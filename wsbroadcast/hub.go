package wsbroadcast

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// HandlerFunc is used for callbacks
type HandlerFunc func() interface{}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Register requests from the clients.
	register chan *Client

	// Messages to the client
	broadcast chan interface{}

	// Unregister requests from clients.
	unregister chan *Client

	// onRegister callbacks
	onRegister []HandlerFunc
}

// NewHub Create a new hub for the handler
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan interface{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Broadcast message to the clients
func (h *Hub) Broadcast(m interface{}) {
	h.broadcast <- m
}

// OnRegister calls a handler when a client registers.
func (h *Hub) OnRegister(f HandlerFunc) {
	h.onRegister = append(h.onRegister, f)
}

// Run the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			for _, c := range h.onRegister {
				client.send <- c()
			}
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024 * 1024 * 200,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ServeWs handles websocket requests from the peer.
func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: h, conn: conn, send: make(chan interface{}, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
