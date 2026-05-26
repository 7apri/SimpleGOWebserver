package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type ClientActionType uint8

const (
	ActionUnknown ClientActionType = iota
	ActionUnsubscribe
	ActionSubscribe
)

type Client struct {
	conn   *websocket.Conn
	egress chan []byte
	userID uuid.UUID
}

type ClientMessage struct {
	Action    ClientActionType `json:"a"`
	TopicType TopicType        `json:"t"`
	TopicID   uuid.UUID        `json:"i"`
}

func (c *Client) writePump() {
	ticker := time.NewTicker(45 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.egress:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump(h *WebsocketHub) {
	defer func() {
		h.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(payload, &msg); err == nil {
			if msg.Action == ActionSubscribe || msg.Action == ActionUnsubscribe {
				h.subscribe <- Subscription{
					Topic:  Topic{Type: msg.TopicType, ID: msg.TopicID},
					Client: c,
					Action: msg.Action,
				}
			}
		}
	}
}
