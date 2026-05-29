package server

import (
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func (rw *RouteWrapper) HandleFollow(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	username := r.PathValue("username")
	follower, ok := auth.GetUser(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "no_session", nil, nil)
	}
	followed, err := rw.socialWrapper.GetProfileByUsername(ctx, username)
	if err != nil {
		return web.NewError(http.StatusNotFound, "User not found", err, nil)
	}
	isFollowed, err := rw.socialWrapper.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "", err, nil)
	}
	return rw.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "htmx", Name: "follow-button"}, isFollowed)
}
