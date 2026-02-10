package server

import (
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
)

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		server.templates["404.html"].Execute(w, nil)
		return
	}
	_, loggedIn := auth.GetUserFromContext(r.Context())

	if !loggedIn {
		http.Redirect(w, r, "/login?next=/", http.StatusFound)
		return
	}

	server.templates["dashboard.html"].Execute(w, nil)
}
func (server *Server) serveHtml(path string, data any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.templates[path].Execute(w, data)
	})
}
