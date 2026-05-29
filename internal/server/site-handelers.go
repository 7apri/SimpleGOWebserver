package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/social"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func (rw *RouteWrapper) HandleRoot(w http.ResponseWriter, r *http.Request) *web.WebError {
	if r.URL.Path != "/" {
		return web.NewError(http.StatusNotFound, "err_not_found", nil, nil)
	}
	var (
		profile *social.UserProfile
		meta    string
	)
	user, ok := auth.GetUser(r.Context())
	if ok {
		meta += user.Username
		profile, _ = rw.socialWrapper.GetProfileByUsername(r.Context(), user.Username)
	}
	if profile != nil {
		meta = strconv.FormatInt(profile.UpdatedAt.Unix(), 16)
	}
	return rw.templateMgr.WriteTemplateHtmx(w, r, templates.TemplateKey{Kind: "page", Name: "main"}, templates.TemplateKey{Kind: "htmx", Name: "home"}, profile, profile, meta)
}

func (rw *RouteWrapper) HandleSignUp(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookie, err := r.Cookie("oauth_pending")

	if err == nil && cookie.Value != "" {
		claims, err := rw.authHandler.GetPendingAuthProviderClaims(cookie.Value)
		if err == nil && claims != nil {
			return rw.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "auth/finish-external"}, claims.AvatarURL, claims)
		}
	}
	return rw.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "auth/register"}, "", nil)
}

type RelationshipStatus string

const (
	StatusSelf      RelationshipStatus = "self"
	StatusFollowing RelationshipStatus = "following"
	StatusNotFollow RelationshipStatus = "not_following"
	StatusBlocked   RelationshipStatus = "blocked"
)

func (rw *RouteWrapper) HandleProfile(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	profile, err := rw.socialWrapper.GetProfileByUsername(ctx, r.PathValue("username"))
	if err != nil {
		slog.Error("Failed to fetch profile", "username", r.PathValue("username"), "err", err)
		return web.NewError(http.StatusNotFound, "user_not_found", err, nil)
	}
	var isFollowing bool
	user, ok := auth.GetUser(ctx)
	if ok && user.ID != profile.ID {
		isFollowing, err = rw.socialWrapper.IsFollowing(ctx, user.ID, profile.ID)
		if err != nil {
			slog.Error("Follow status check failed", "err", err)
		}
	}

	status := StatusNotFollow
	if user.ID == profile.ID {
		status = StatusSelf
	} else if isFollowing {
		status = StatusFollowing
	}

	data := struct {
		Profile *social.UserProfile
		Status  RelationshipStatus
	}{
		Profile: profile,
		Status:  status,
	}

	return rw.templateMgr.WriteTemplateHtmx(w, r, templates.TemplateKey{Kind: "page", Name: "main"}, templates.TemplateKey{Kind: "htmx", Name: "user-profile"}, profile, data, user.ID.String(), profile.ID.String(), string(status))
}

func (rw *RouteWrapper) serveHtmx(bodyName, fragmentName string) http.Handler {
	return rw.handlerHtml(func(w http.ResponseWriter, r *http.Request) *web.WebError {
		return rw.templateMgr.WriteTemplateHtmx(w, r, templates.TemplateKey{Kind: "page", Name: bodyName}, templates.TemplateKey{Kind: "htmx", Name: fragmentName}, nil, nil)
	})
}
func (rw *RouteWrapper) serveHtml(name string) http.Handler {
	return rw.handlerHtml(func(w http.ResponseWriter, r *http.Request) *web.WebError {
		return rw.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: name}, "", nil)
	})
}
func (rw *RouteWrapper) serveHtmlUser(name string) http.Handler {
	return rw.handlerHtml(func(w http.ResponseWriter, r *http.Request) *web.WebError {
		user, _ := auth.GetUser(r.Context())
		return rw.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: name}, "", user)
	})
}
