package server

import (
	"context"
	"net/http"
	"strings"
	"time"

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

func (rw *RouteWrapper) HandleDeletePost(w http.ResponseWriter, r *http.Request) *web.WebError {
	postID, err := uuid.Parse(r.PathValue("postID"))
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	ctx := r.Context()
	user, _ := auth.GetUser(ctx)

	err = rw.socialWrapper.DeletePost(ctx, postID, user.ID)
	if err != nil {
		switch err {
		case social.ErrDatabase:
			return web.NewError(http.StatusInternalServerError, "", err, nil)
		case social.ErrPostNotFound:
			return web.NewError(http.StatusNotFound, "", err, nil)
		default:
			return web.NewError(http.StatusUnauthorized, "unauthorized", err, nil)
		}
	}

	return nil
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
		ID         uuid.UUID
		LikesCount int64
		IsLiked    bool
	}{
		ID:         postID,
		LikesCount: postLikes,
		IsLiked:    isLiked,
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
		ID           uuid.UUID
		RepostsCount int64
		IsReposted   bool
	}{
		ID:           postID,
		RepostsCount: count,
		IsReposted:   isReposted,
	}
	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "post-repost-btn"}, data)
}

func (rw *RouteWrapper) HandleSendMessage(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := auth.GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}
	roomID := r.PathValue("roomID")

	if err := r.ParseForm(); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_form_data", nil, nil)
	}

	contentEncrypted := r.FormValue("content_encrypted")
	nonce := r.FormValue("nonce")

	if contentEncrypted == "" || nonce == "" {
		return web.NewError(http.StatusBadRequest, "missing_cryptographic_payload", nil, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	const q = `
		INSERT INTO messages (room_id, sender_id, content_encrypted, nonce, key_version)
		SELECT $1, $2, $3, $4, 1
		WHERE EXISTS (SELECT 1 FROM room_participants WHERE room_id = $1 AND user_id = $2)
		RETURNING id, created_at`

	var msgID uuid.UUID
	var createdAt time.Time

	err := rw.database.Pool.QueryRow(ctx, q, roomID, user.ID, contentEncrypted, nonce).Scan(&msgID, &createdAt)
	if err != nil {
		return web.NewError(http.StatusForbidden, "forbidden", nil, nil)
	}

	data := map[string]interface{}{
		"ID":               msgID,
		"SenderUsername":   user.Username,
		"CreatedAt":        createdAt,
		"ContentEncrypted": contentEncrypted,
		"Nonce":            nonce,
		"RoomID":           roomID,
	}

	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "chat-message"}, data)
}

type MessageRow struct {
	ID               uuid.UUID
	SenderID         uuid.UUID
	SenderUsername   string
	ContentEncrypted string
	Nonce            string
	RoomID           uuid.UUID
	CreatedAt        time.Time
}

func (rw *RouteWrapper) HandleFetchMessages(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := auth.GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}
	roomID := r.PathValue("roomID")

	since := r.URL.Query().Get("since")
	cursor := r.URL.Query().Get("cursor")

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	var q string
	var args []interface{}

	if since != "" {
		q = `
            SELECT m.id, u.username, m.content_encrypted, m.nonce, m.room_id, m.created_at
            FROM messages m
            JOIN users u ON m.sender_id = u.id
            WHERE m.room_id = $1 AND m.created_at > $2
              AND EXISTS (SELECT 1 FROM room_participants WHERE room_id = $1 AND user_id = $3)
            ORDER BY m.created_at ASC`
		args = []interface{}{roomID, since, user.ID}
	} else {
		if cursor == "" {
			cursor = time.Now().Format(time.RFC3339)
		}
		q = `
            SELECT m.id, u.username, m.content_encrypted, m.nonce, m.room_id, m.created_at
            FROM messages m
            JOIN users u ON m.sender_id = u.id
            WHERE m.room_id = $1 AND m.created_at < $2
              AND EXISTS (SELECT 1 FROM room_participants WHERE room_id = $1 AND user_id = $3)
            ORDER BY m.created_at DESC
            LIMIT 20`
		args = []interface{}{roomID, cursor, user.ID}
	}

	rows, err := rw.database.Pool.Query(ctx, q, args...)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database error", nil, nil)
	}
	defer rows.Close()

	var messages []MessageRow
	for rows.Next() {
		var m MessageRow
		if err := rows.Scan(&m.ID, &m.SenderUsername, &m.ContentEncrypted, &m.Nonce, &m.RoomID, &m.CreatedAt); err == nil {
			messages = append(messages, m)
		}
	}

	isPolling := since != ""

	if isPolling && len(messages) > 0 {
		_, _ = rw.database.Pool.Exec(ctx,
			"UPDATE room_participants SET last_read_at = NOW() WHERE room_id = $1 AND user_id = $2",
			roomID, user.ID)

		latest := messages[len(messages)-1].CreatedAt
		w.Header().Set("X-Latest-Timestamp", latest.Format(time.RFC3339))
	}

	return rw.templateMgr.WriteTemplateWeb(w, r, templates.TemplateKey{Kind: "htmx", Name: "chat-message-list"}, map[string]interface{}{
		"Messages":  messages,
		"IsPolling": isPolling,
	})
}
