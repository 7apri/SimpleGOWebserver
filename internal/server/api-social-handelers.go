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
	const limit = 20
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
	user, ok := auth.GetUser(ctx)
	if ok {
		userID = user.ID
	}
	posts, err := rw.socialWrapper.GetGlobalFeed(ctx, userID, cursor, limit)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	data := struct {
		Posts      []social.Post
		IsLoggedIn bool
		NextCursor *uuid.UUID
	}{
		IsLoggedIn: ok,
		Posts:      posts,
	}
	if len(posts) >= limit {
		id := posts[len(posts)-1].ID
		data.NextCursor = &id
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "feed"}, data)
}

func (rw *RouteWrapper) HandleLikePost(w http.ResponseWriter, r *http.Request) *web.WebError {
	postID, err := uuid.Parse(r.PathValue("postID"))
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	ctx := r.Context()
	user, _ := auth.GetUser(ctx)

	isLiked, _, err := rw.socialWrapper.ToggleLike(ctx, user.ID, postID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}
	postLikes, err := rw.socialWrapper.GetPostLikes(ctx, postID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	data := struct {
		PostID  uuid.UUID
		Likes   int64
		IsLiked bool
	}{
		PostID:  postID,
		Likes:   postLikes,
		IsLiked: isLiked,
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "post-like-btn"}, data)
}

func (rw *RouteWrapper) HandleRepostPost(w http.ResponseWriter, r *http.Request) *web.WebError {
	postID, err := uuid.Parse(r.PathValue("postID"))
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	ctx := r.Context()
	user, _ := auth.GetUser(ctx)

	isReposted, err := rw.socialWrapper.ToggleRepost(ctx, user.ID, postID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}
	count, err := rw.socialWrapper.GetPostReposts(ctx, postID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	data := struct {
		PostID    uuid.UUID
		Count     int64
		IsChecked bool
	}{
		PostID:    postID,
		Count:     count,
		IsChecked: isReposted,
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "post-repost-btn"}, data)
}
