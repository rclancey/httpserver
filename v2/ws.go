package httpserver

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMessageSize = 512
)

var newline = []byte{'\n'}
var upgrader = websocket.Upgrader{
	ReadBufferSize: 1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(req *http.Request) bool {
		return true
	},
}

type Hub interface {
	Run()
	Stop()
	Closed() bool
	Register(*Client)
	Unregister(*Client)
	Broadcast([]byte)
	BroadcastEvent(interface{})
}

type GenericHub struct {
	clients map[*Client]bool
	stopper chan bool
	broadcast chan []byte
	register chan *Client
	unregister chan *Client
	closed bool
	Events chan interface{}
}

func NewGenericHub(source chan interface{}) *GenericHub {
	return &GenericHub{
		clients: make(map[*Client]bool),
		stopper: make(chan bool),
		broadcast: make(chan []byte),
		register: make(chan *Client),
		unregister: make(chan *Client),
		closed: false,
		Events: source,
	}
}

func (h *GenericHub) Run() {
	h.closed = false
	stopped := false
	for !stopped {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
		case evt, ok := <-h.Events:
			if !ok {
				stopped = true
			}
			message, _ := json.Marshal(evt)
			toClose := []*Client{}
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					toClose = append(toClose, client)
				}
			}
			for _, client := range toClose {
				log.Println("can't send to client", client, "closing")
				close(client.Send)
				delete(h.clients, client)
			}
		case message := <-h.broadcast:
			toClose := []*Client{}
			for client := range h.clients {
				select {
				case client.Send <- message:
				default:
					toClose = append(toClose, client)
				}
			}
			for _, client := range toClose {
				log.Println("can't send to client", client, "closing")
				close(client.Send)
				delete(h.clients, client)
			}
		case <-h.stopper:
			stopped = true
		}
	}
	h.closed = true
	toClose := []*Client{}
	for client := range h.clients {
		toClose = append(toClose, client)
	}
	for _, client := range toClose {
		close(client.Send)
		delete(h.clients, client)
	}
}

func (h *GenericHub) Stop() {
	h.stopper <- true
}

func (h *GenericHub) Closed() bool {
	return h.closed
}

func (h *GenericHub) Register(c *Client) {
	h.register <- c
}

func (h *GenericHub) Unregister(c *Client) {
	h.unregister <- c
}

func (h *GenericHub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

func (h *GenericHub) BroadcastEvent(evt interface{}) {
	msg, _ := json.Marshal(evt)
	h.Broadcast(msg)
}

type Client struct {
	hub Hub
	conn *websocket.Conn
	Send chan []byte
	isOpen bool
}

func (c *Client) Open(conn *websocket.Conn) error {
	c.conn = conn
	c.isOpen = true
	c.hub.Register(c)
	return nil
}

func (c *Client) Close() {
	isOpen := c.isOpen
	c.isOpen = false
	hub := c.hub
	c.hub = nil
	conn := c.conn
	c.conn = nil
	if hub != nil {
		hub.Unregister(c)
	}
	if conn != nil && isOpen {
		conn.WriteMessage(websocket.CloseMessage, []byte{})
		conn.Close()
	}
}

func (c *Client) SetReadLimit(size int64) {
	conn := c.conn
	if conn != nil && c.isOpen {
		conn.SetReadLimit(size)
	}
}

func (c *Client) SetReadDeadline(t time.Time) error {
	conn := c.conn
	if conn != nil && c.isOpen {
		return conn.SetReadDeadline(t)
	}
	return nil
}

func (c *Client) SetPongHandler(f func(string) error) {
	conn := c.conn
	if conn != nil && c.isOpen {
		conn.SetPongHandler(f)
	}
}

func (c *Client) ReadMessage() (int, []byte, error) {
	conn := c.conn
	if conn != nil && c.isOpen {
		return conn.ReadMessage()
	}
	return -1, nil, errors.New("can't read on closed websocket")
}

func (c *Client) SetWriteDeadline(t time.Time) error {
	conn := c.conn
	if conn != nil && c.isOpen {
		return conn.SetWriteDeadline(t)
	}
	return nil
}

func (c *Client) NextWriter(messageType int) (io.WriteCloser, error) {
	conn := c.conn
	if conn != nil && c.isOpen {
		return conn.NextWriter(messageType)
	}
	return nil, errors.New("can't write to closed websocket")
}

func (c *Client) WriteMessage(messageType int, data []byte) error {
	conn := c.conn
	if conn != nil && c.isOpen {
		return conn.WriteMessage(messageType, data)
	}
	return errors.New("can't write to closed websocket")
}

func (c *Client) ReadPump() {
	defer c.Close()
	c.SetReadLimit(maxMessageSize)
	c.SetReadDeadline(time.Now().Add(pongWait))
	c.SetPongHandler(func(string) error {
		c.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if c.hub == nil {
			return
		}
		_, message, err := c.ReadMessage()
		if err != nil {
			if err == websocket.ErrCloseSent {
				log.Println("client closed websocket")
			} else {
				log.Println("websocket read error:", err)
			}
			return
		}
		hub := c.hub
		if hub != nil {
			hub.Broadcast(message)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Println("source channel closed, shutting down websocket")
				return
			}
			w, err := c.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println("error getting websocket writer:", err)
				return
			}
			w.Write(message)
			// flush queued messages
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.Send)
			}
			err = w.Close()
			if err != nil {
				log.Println("websocket write error:", err)
				return
			}
		case <-ticker.C:
			c.SetWriteDeadline(time.Now().Add(writeWait))
			err := c.WriteMessage(websocket.PingMessage, nil);
			if err != nil {
				log.Println("websocket ping error:", err)
				return
			}
		}
	}
}

func ServeWS(hub Hub, w http.ResponseWriter, req *http.Request) (interface{}, error) {
	client := &Client{hub: hub, Send: make(chan []byte, 256)}
	return client, nil
}

