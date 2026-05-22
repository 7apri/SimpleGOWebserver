package websocket

import (
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type TopicType uint8

const (
	TopicUser TopicType = iota
	TopicPost
	TopicGlobal
)

type Topic struct {
	Type TopicType
	ID   uuid.UUID
}

type TopicMessage struct {
	Topic   Topic
	Payload []byte
}

type Subscription struct {
	Topic  Topic
	Client *Client
	Action ActionType
}

type WebsocketHub struct {
	topics         map[Topic]map[*Client]struct{}
	clientToTopics map[*Client]map[Topic]struct{}

	broadcast  chan TopicMessage
	subscribe  chan Subscription
	unregister chan *Client
	mu         sync.RWMutex
	upgr       websocket.Upgrader
}

func NewWebsocketHub() *WebsocketHub {
	return &WebsocketHub{
		topics:         make(map[Topic]map[*Client]struct{}),
		clientToTopics: make(map[*Client]map[Topic]struct{}),
		broadcast:      make(chan TopicMessage, 512),
		subscribe:      make(chan Subscription, 128),
		unregister:     make(chan *Client, 64),
		upgr: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

func (h *WebsocketHub) Broadcast(topicType TopicType, topicID uuid.UUID, payload []byte) bool {
	msg := TopicMessage{
		Topic: Topic{
			Type: topicType,
			ID:   topicID,
		},
		Payload: payload,
	}

	select {
	case h.broadcast <- msg:
		return true
	default:
		return false
	}
}
