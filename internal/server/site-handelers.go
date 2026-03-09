package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/gorilla/websocket"
)

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) *web.WebError {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return server.templateMgr.RenderPage(w, r, templates.TemplateKey{Kind: "page", Name: "err/404"}, nil)
	}
	_, loggedIn := auth.GetUserFromContext(r.Context())

	if !loggedIn {
		http.Redirect(w, r, "/api/auth/refresh", http.StatusTemporaryRedirect)
		return nil
	}

	return server.templateMgr.RenderPage(w, r, templates.TemplateKey{Kind: "page", Name: "main"}, nil)
}

type pageTemplateData struct {
	key  templates.TemplateKey
	data any
}

type challengeUIConfig struct {
	tokenName   string
	codeName    string
	codeMaxAge  int
	tokenMaxAge int
	basePage    pageTemplateData
}

func setChallengeCookie(w http.ResponseWriter, cookieName, val string, cookieMaxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:  cookieName,
		Value: val,
		Path:  "/", MaxAge: cookieMaxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}
func cookieExists(r *http.Request, cookieName string) bool {
	if _, err := r.Cookie(cookieName); err == nil {
		return true
	}
	return false
}

func (server *Server) handleChallengeUI(
	cfg challengeUIConfig,
	onSuccess func(w http.ResponseWriter, r *http.Request) *web.WebError,
	onPending func(w http.ResponseWriter, r *http.Request) *web.WebError,
) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		q := r.URL.Query()
		t, c := q.Get("t"), q.Get("c")

		if t != "" || c != "" {
			if t != "" {
				setChallengeCookie(w, cfg.tokenName, t, cfg.tokenMaxAge)
			}
			if c != "" {
				setChallengeCookie(w, cfg.codeName, c, cfg.codeMaxAge)
			}
			q.Del("t")
			q.Del("c")
			target := r.URL.Path
			if qs := q.Encode(); qs != "" {
				target += "?" + qs
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return nil
		}

		hasToken := cookieExists(r, cfg.tokenName)
		hasCode := cookieExists(r, cfg.codeName)

		if hasToken && hasCode {
			return onSuccess(w, r)
		}
		if hasToken {
			return onPending(w, r)
		}

		return server.templateMgr.RenderPage(w, r, cfg.basePage.key, cfg.basePage.data)
	}
}

func (server *Server) serveHtml(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.templateMgr.RenderPage(w, r, templates.TemplateKey{Kind: "page", Name: name}, nil)
	})
}

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
