package server

import (
	"context"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func (rw *RouteWrapper) HandleListChats(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := auth.GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	rooms, err := rw.socialWrapper.GetChats(ctx, user.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "", err, nil)
	}

	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "chat-list"}, rooms)
}
