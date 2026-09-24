package internal

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Hub menampung semua klien WebSocket yang terautentikasi
// dan menyiarkan pesan (reading sensor / status lampu) ke semuanya.
type Hub struct {
	Clients    map[*WSClient]bool
	Broadcast  chan []byte
	Register   chan *WSClient
	Unregister chan *WSClient
	stop       chan struct{}
}

type WSClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func NewHub() *Hub {
	return &Hub{
		Clients:    map[*WSClient]bool{},
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *WSClient),
		Unregister: make(chan *WSClient),
		stop:       make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.Register:
			h.Clients[c] = true
		case c := <-h.Unregister:
			if _, ok := h.Clients[c]; ok {
				delete(h.Clients, c)
				close(c.send)
			}
		case msg := <-h.Broadcast:
			for c := range h.Clients {
				select {
				case c.send <- msg:
				default: // buffer penuh -> putuskan klien lambat
					delete(h.Clients, c)
					close(c.send)
				}
			}
		case <-h.stop:
			for c := range h.Clients {
				close(c.send)
				delete(h.Clients, c)
			}
			return
		}
	}
}

func (h *Hub) Stop() { close(h.stop) }

// ServeWS meng-upgrade koneksi HTTP ke WebSocket.
// JWT sudah divalidasi oleh middleware Auth sebelumnya.
func (h *Hub) ServeWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ws] upgrade gagal: %v", err)
		return
	}
	client := &WSClient{hub: h, conn: conn, send: make(chan []byte, 256)}
	h.Register <- client
	go client.writePump()
	go client.readPump()
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}