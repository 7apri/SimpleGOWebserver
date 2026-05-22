package websocket

import (
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/google/uuid"
)

func (h *WebsocketHub) HandleWS(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, _ := auth.GetUser(r.Context())
	conn, err := h.upgr.Upgrade(w, r, nil)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "upgrade", err, nil)
	}

	client := &Client{
		conn:   conn,
		egress: make(chan []byte, 32),
		userID: user.ID,
	}

	h.subscribe <- Subscription{
		Topic:  Topic{Type: TopicGlobal, ID: uuid.Nil},
		Client: client,
		Action: ActionSubscribe,
	}

	go client.writePump()
	go client.readPump(h)

	return nil
}

func (h *WebsocketHub) Run() {
	for {
		select {
		case sub := <-h.subscribe:
			h.mu.Lock()
			switch sub.Action {
			case ActionSubscribe:
				if h.topics[sub.Topic] == nil {
					h.topics[sub.Topic] = make(map[*Client]struct{})
				}
				h.topics[sub.Topic][sub.Client] = struct{}{}

				if h.clientToTopics[sub.Client] == nil {
					h.clientToTopics[sub.Client] = make(map[Topic]struct{})
				}
				h.clientToTopics[sub.Client][sub.Topic] = struct{}{}

			case ActionUnsubscribe:
				if subs, ok := h.topics[sub.Topic]; ok {
					delete(subs, sub.Client)
					if len(subs) == 0 {
						delete(h.topics, sub.Topic)
					}
				}
				if clientTopics, ok := h.clientToTopics[sub.Client]; ok {
					delete(clientTopics, sub.Topic)
					if len(clientTopics) == 0 {
						delete(h.clientToTopics, sub.Client)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.topics[msg.Topic]; ok {
				for client := range clients {
					select {
					case client.egress <- msg.Payload:
					default:
					}
				}
			}
			h.mu.RUnlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if clientTopics, ok := h.clientToTopics[c]; ok {
				for topic := range clientTopics {
					if subs, ok := h.topics[topic]; ok {
						delete(subs, c)
						if len(subs) == 0 {
							delete(h.topics, topic)
						}
					}
				}
				delete(h.clientToTopics, c)
			}
			h.mu.Unlock()

			close(c.egress)
		}
	}
}
