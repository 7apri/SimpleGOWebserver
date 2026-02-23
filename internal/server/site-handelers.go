package server

import (
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
)

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	lang, _ := i18n.GetLangFromContext(r.Context())

	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		server.templateMgr.Get(lang, "page", "404").Execute(w, nil)
		return
	}
	_, loggedIn := auth.GetUserFromContext(r.Context())

	if !loggedIn {
		http.Redirect(w, r, "/api/auth/refresh", http.StatusTemporaryRedirect)
		return
	}

	server.templateMgr.Get(lang, "page", "dashboard").Execute(w, nil)
}
func (server *Server) serveHtml(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.templateMgr.RenderPage(w, r, name, nil)
	})
}
