package socket_helper

import (
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn     *websocket.Conn
	send     chan Message
	IsClosed atomic.Bool
}

func NewClient(c *websocket.Conn) *Client {
	return &Client{
		conn: c,
		send: make(chan Message, 300),
	}
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
)

type Message struct {
	PhoneNumber string `json:"phone_number"`
	Message     string `json:"message"`
}

func (c *Client) ReadPump() error {
	c.conn.SetReadLimit(1024)

	defer func() {
		c.IsClosed.Store(true)
		close(c.send)
	}()

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}

	c.conn.SetPongHandler(func(_ string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			return err
		}
	}
}

func (c *Client) WritePump() error {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.IsClosed.Store(true)
		c.conn.Close()
	}()

	for {
		select {
		case <-ticker.C:
			if err := c.setWriteDeadline(writeWait); err != nil {
				return err
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return err
			}

		case msg, ok := <-c.send:
			if !ok {
				return nil
			}

			if err := c.setWriteDeadline(writeWait); err != nil {
				return err
			}

			if err := c.conn.WriteJSON(msg); err != nil {
				return err
			}
		}
	}
}

func (c *Client) Write(msg Message) {
	if c.IsClosed.Load() {
		return
	}

	c.send <- msg
}

func (c *Client) setWriteDeadline(duration time.Duration) error {
	return c.conn.SetWriteDeadline(time.Now().Add(duration))
}
