package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/social"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (rw *RouteWrapper) HandleListChats(w http.ResponseWriter, r *http.Request) *web.WebError {
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

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	rooms, err := rw.socialWrapper.GetChats(ctx, user.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "", err, nil)
	}

	return rw.templateMgr.WriteTemplateHtmx(w, r,
		templates.TemplateKey{Kind: "page", Name: "main"},
		templates.TemplateKey{Kind: "htmx", Name: "chat-list"},
		profile, rooms, templates.PageMeta{}, meta,
	)
}

type InitDMResponse struct {
	RoomID string `json:"room_id"`
	IsNew  bool   `json:"is_new"`
}

func (rw *RouteWrapper) HandleInitDM(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := auth.GetUser(r.Context())
	if !ok || user == nil {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}

	targetUserID, err := uuid.Parse(r.PathValue("targetUserID"))
	if err != nil {
		return web.NewError(http.StatusBadRequest, "invalid target user", nil, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	var existingRoomID uuid.UUID
	const checkQ = `
        SELECT r.id FROM rooms r
        JOIN room_participants p1 ON r.id = p1.room_id AND p1.user_id = $1
        JOIN room_participants p2 ON r.id = p2.room_id AND p2.user_id = $2
        WHERE r.type = 'dm' LIMIT 1`

	err = rw.database.Pool.QueryRow(ctx, checkQ, user.ID, targetUserID).Scan(&existingRoomID)

	w.Header().Set("Content-Type", "application/json")

	if err == nil {
		sonic.ConfigDefault.NewEncoder(w).Encode(InitDMResponse{RoomID: existingRoomID.String(), IsNew: false})
		return nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return web.NewError(http.StatusInternalServerError, "database check failed", err, nil)
	}

	tx, err := rw.database.Pool.Begin(ctx)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "tx start fail", err, nil)
	}
	defer tx.Rollback(ctx)

	var newRoomID uuid.UUID
	err = tx.QueryRow(ctx, `INSERT INTO rooms (type, name, created_by) VALUES ('dm', 'DM', $1) RETURNING id`, user.ID).Scan(&newRoomID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "room creation failed", err, nil)
	}

	if user.ID == targetUserID {
		_, err = tx.Exec(ctx, `INSERT INTO room_participants (room_id, user_id) VALUES ($1, $2)`, newRoomID, user.ID)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO room_participants (room_id, user_id) VALUES ($1, $2), ($1, $3)`, newRoomID, user.ID, targetUserID)
	}

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "participant insertion failed", err, nil)
	}

	if err := tx.Commit(ctx); err != nil {
		return web.NewError(http.StatusInternalServerError, "tx commit failed", err, nil)
	}

	sonic.ConfigDefault.NewEncoder(w).Encode(InitDMResponse{RoomID: newRoomID.String(), IsNew: true})
	return nil
}

type UploadKeysPayload struct {
	EncryptedKeys []auth.EncryptedRoomKeyPayload `json:"encrypted_keys"`
}

func (rw *RouteWrapper) HandleUploadRoomKeys(w http.ResponseWriter, r *http.Request) *web.WebError {
	roomID, err := uuid.Parse(r.PathValue("roomID"))
	if err != nil {
		return web.NewError(http.StatusBadRequest, "invalid room ID format", err, nil)
	}

	cookie, err := r.Cookie("device_id")
	if err != nil {
		return web.NewError(http.StatusBadRequest, "no device id cookie", err, nil)
	}

	var payload UploadKeysPayload
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&payload); err != nil {
		return web.NewError(http.StatusBadRequest, "malformed payload", err, nil)
	}

	if len(payload.EncryptedKeys) == 0 {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	batch := &pgx.Batch{}
	const insertKey = `
    INSERT INTO room_keys (room_id, device_id, key_sender_device_id, key_version, encrypted_room_key)
    VALUES ($1, $2, $3, 1, $4)
    ON CONFLICT (room_id, device_id, key_version) DO NOTHING`

	for _, k := range payload.EncryptedKeys {
		batch.Queue(insertKey, roomID, k.DeviceID, cookie.Value, k.EncryptedRoomKey)
	}

	br := rw.database.Pool.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(payload.EncryptedKeys); i++ {
		if _, err := br.Exec(); err != nil {
			return web.NewError(http.StatusInternalServerError, "failed to insert key batch", err, nil)
		}
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (rw *RouteWrapper) fetchInitialMessages(ctx context.Context, roomID uuid.UUID, userID uuid.UUID) []MessageRow {
	const q = `
        SELECT m.id, u.username, m.content_encrypted, m.nonce, m.room_id, m.created_at, m.sender_id
        FROM (
            SELECT id, sender_id, content_encrypted, nonce, room_id, created_at 
            FROM messages 
            WHERE room_id = $1 
            ORDER BY created_at DESC 
            LIMIT 20
        ) m
        JOIN users u ON m.sender_id = u.id
        WHERE EXISTS (
            SELECT 1 FROM room_participants 
            WHERE room_id = $1 AND user_id = $2
        )
        ORDER BY m.created_at ASC`

	rows, err := rw.database.Pool.Query(ctx, q, roomID, userID)
	if err != nil {
		log.Printf("Error fetching initial messages: %v", err)
		return nil
	}
	defer rows.Close()

	var messages []MessageRow
	for rows.Next() {
		var m MessageRow
		err := rows.Scan(&m.ID, &m.SenderUsername, &m.ContentEncrypted, &m.Nonce, &m.RoomID, &m.CreatedAt, &m.SenderID)
		if err == nil {
			messages = append(messages, m)
		}
	}
	return messages
}

func (rw *RouteWrapper) HandleChat(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, _ := auth.GetUser(r.Context())
	profile, _ := rw.socialWrapper.GetProfileByUsername(r.Context(), user.Username)
	roomID, err := uuid.Parse(r.PathValue("chatID"))
	if err != nil {
		return web.NewError(http.StatusBadRequest, "invalid room ID format", err, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	var roomName string
	const q = `
        SELECT r.name 
        FROM rooms r
        JOIN room_participants rp ON r.id = rp.room_id
        WHERE r.id = $1 AND rp.user_id = $2`

	err = rw.database.Pool.QueryRow(ctx, q, roomID, user.ID).Scan(&roomName)
	if err != nil {
		return web.NewError(http.StatusNotFound, "Room not found or unauthorized", nil, nil)
	}

	messages := rw.fetchInitialMessages(ctx, roomID, user.ID)

	data := map[string]interface{}{
		"RoomID":        roomID,
		"RoomName":      roomName,
		"Messages":      messages,
		"CurrentUserID": user.ID,
	}

	return rw.templateMgr.WriteTemplateHtmx(w, r,
		templates.TemplateKey{Kind: "page", Name: "main"},
		templates.TemplateKey{Kind: "htmx", Name: "chat"},
		profile, data, templates.PageMeta{},
	)
}
