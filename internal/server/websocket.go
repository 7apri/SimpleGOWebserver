package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/gorilla/websocket"
)

func refreshWebsocket(refresh chan rune) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	var (
		clients  = make(map[*websocket.Conn]bool)
		mu       sync.Mutex
		upgrader = websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
	)

	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case t, ok := <-refresh:
				if !ok {
					return
				}
				msg := []byte(string(t))

				mu.Lock()
				activeClients := make([]*websocket.Conn, 0, len(clients))
				for c := range clients {
					activeClients = append(activeClients, c)
				}
				mu.Unlock()

				for _, client := range activeClients {
					client.SetWriteDeadline(time.Now().Add(time.Second * 2))

					err := client.WriteMessage(websocket.TextMessage, msg)
					if err != nil {
						slog.Warn("failed to notify client, cleaning up", "err", err)
						client.Close()

						mu.Lock()
						delete(clients, client)
						mu.Unlock()
					}
				}
			case <-ticker.C:
				mu.Lock()
				for client := range clients {
					err := client.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second*5))
					if err != nil {
						client.Close()
						delete(clients, client)
					}
				}
				mu.Unlock()
			}
		}
	}()
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "err:websocket_upgrade_fail", err, nil)
		}

		mu.Lock()
		clients[conn] = true
		mu.Unlock()

		defer func() {
			mu.Lock()
			delete(clients, conn)
			mu.Unlock()
			conn.Close()
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
		return nil
	}
}
