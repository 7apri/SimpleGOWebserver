package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
        INSERT INTO room_keys (room_id, device_id, key_version, encrypted_room_key)
        VALUES ($1, $2, 1, $3)
        ON CONFLICT (room_id, device_id) DO NOTHING`

	for _, k := range payload.EncryptedKeys {
		batch.Queue(insertKey, roomID, k.DeviceID, k.EncryptedRoomKey)
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
