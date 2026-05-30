package server

import (
	"net/http"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/social"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/google/uuid"
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
		return web.NewError(http.StatusNotFound, "user_not_found", err, nil)
	}
	if follower.ID == followed.ID {
		return web.NewError(http.StatusBadRequest, "cant_follow_yourself", err, nil)
	}
	isFollowed, err := rw.socialWrapper.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "", err, nil)
	}
	followers, err := rw.socialWrapper.GetRealTimeFollowerCount(ctx, followed.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "", err, nil)
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "follow-button"}, struct {
		IsFollowed    bool
		Username      string
		FollowerCount int
	}{IsFollowed: isFollowed, Username: username, FollowerCount: followers})
}

func (rw *RouteWrapper) HandleCreatePost(w http.ResponseWriter, r *http.Request) *web.WebError {
	if err := r.ParseForm(); err != nil {
		return web.NewError(http.StatusBadRequest, "parse", err, nil)
	}
	ctx := r.Context()
	user, _ := auth.GetUser(ctx)
	profile, err := rw.socialWrapper.GetProfileByID(ctx, user.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	content := r.FormValue("content")
	mediaURLs := r.PostForm["media_urls[]"]

	var cleaned []string
	for _, url := range mediaURLs {
		if trimmed := strings.TrimSpace(url); trimmed != "" && strings.HasPrefix(trimmed, "http") {
			cleaned = append(cleaned, trimmed)
		}
	}

	post, err := rw.socialWrapper.CreatePost(ctx, social.CreatePostParams{
		Author:    profile.MapToAuthor(),
		Content:   content,
		MediaURLs: cleaned,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "post"}, post)
}

func (rw *RouteWrapper) HandleGetFeed(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	var (
		cursor,
		userID uuid.UUID
		err error
	)
	cursorStr := r.URL.Query().Get("cursor")
	if cursorStr != "" {
		cursor, err = uuid.Parse(cursorStr)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
	}
	if user, ok := auth.GetUser(ctx); ok {
		userID = user.ID
	}
	posts, err := rw.socialWrapper.GetGlobalFeed(ctx, userID, cursor, 20)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "feed"}, posts)
}
