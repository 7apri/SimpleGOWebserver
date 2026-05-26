package websocket

import (
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/google/uuid"
)

func (h *WebsocketHub) HandleWS(w http.ResponseWriter, r *http.Request) *web.WebError {
	conn, err := h.upgr.Upgrade(w, r, nil)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "upgrade", err, nil)
	}
	user, ok := auth.GetUser(r.Context())
	usrID := uuid.Nil
	if ok {
		usrID = user.ID
	}
	client := &Client{
		conn:   conn,
		egress: make(chan []byte, 32),
		userID: usrID,
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
		case msg := <-h.broadcast:
			if clients, ok := h.topics[msg.Topic]; ok {
				for client := range clients {
					select {
					case client.egress <- msg.Payload:
					default:
					}
				}
			}
		case c := <-h.unregister:
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
			close(c.egress)
		}
	}
}
