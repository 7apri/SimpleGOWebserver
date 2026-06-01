package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

type RegisterCryptoPayload struct {
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

func (h *AuthHandler) HandleRegisterCryptoIdentity(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}

	var payload RegisterCryptoPayload
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&payload); err != nil {
		return web.NewError(http.StatusBadRequest, "decode", err, nil)
	}

	devID, err := uuid.Parse(payload.DeviceID)
	if err != nil || payload.PublicKey == "" {
		return web.NewError(http.StatusBadRequest, "invalid", err, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	const q = `
		UPDATE user_devices 
		SET public_key = $1, last_seen = NOW()
		WHERE device_id = $2 AND user_id = $3`

	res, err := h.db.Pool.Exec(ctx, q, payload.PublicKey, devID, user.ID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	if res.RowsAffected() == 0 {
		return web.NewError(http.StatusForbidden, "forbidden", nil, nil)
	}

	return nil
}

type KeyFetchRequest struct {
	UserIDs []uuid.UUID `json:"user_ids"`
}

type DevicePublicKeyInfo struct {
	DeviceID  uuid.UUID `json:"device_id"`
	UserID    uuid.UUID `json:"user_id"`
	PublicKey string    `json:"public_key"`
}

func (h *AuthHandler) HandleFetchParticipantKeys(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}

	var req KeyFetchRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "decode", err, nil)
	}

	req.UserIDs = append(req.UserIDs, user.ID)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	const q = `
		SELECT device_id, user_id, public_key 
		FROM user_devices 
		WHERE user_id = ANY($1) AND public_key != 'PENDING'`
	rows, err := h.db.Pool.Query(ctx, q, req.UserIDs)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}
	defer rows.Close()

	var devices []DevicePublicKeyInfo
	for rows.Next() {
		var d DevicePublicKeyInfo
		if err := rows.Scan(&d.DeviceID, &d.UserID, &d.PublicKey); err == nil {
			devices = append(devices)
		}
	}

	return web.SendJSON(w, http.StatusOK, devices)
}

type EncryptedRoomKeyPayload struct {
	DeviceID         uuid.UUID `json:"device_id"`
	EncryptedRoomKey string    `json:"encrypted_room_key"`
}

type RoomType string

const (
	RoomTypeDm    RoomType = "dm"
	RoomTypeGroup RoomType = "group"
)

type CreateRoomPayload struct {
	Type          RoomType                  `json:"type"`
	Name          string                    `json:"name"`
	Participants  []uuid.UUID               `json:"participants"`
	EncryptedKeys []EncryptedRoomKeyPayload `json:"encrypted_keys"`
}

func (h *AuthHandler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) *web.WebError {
	user, ok := GetUser(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "unauthorized", nil, nil)
	}

	var payload CreateRoomPayload
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&payload); err != nil {
		return web.NewError(http.StatusBadRequest, "decode", err, nil)
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*2)
	defer cancel()

	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database_tx", err, nil)
	}
	defer tx.Rollback(ctx)

	var roomID uuid.UUID
	err = tx.QueryRow(ctx, "INSERT INTO rooms (type, name) VALUES ($1, $2) RETURNING id", payload.Type, payload.Name).Scan(&roomID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	payload.Participants = append(payload.Participants, user.ID)
	for _, uID := range payload.Participants {
		_, err = tx.Exec(ctx, "INSERT INTO room_participants (room_id, user_id) VALUES ($1, $2)", roomID, uID)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "database", err, nil)
		}
	}

	for _, k := range payload.EncryptedKeys {
		const insertKey = `
			INSERT INTO room_keys (room_id, device_id, key_version, encrypted_room_key)
			VALUES ($1, $2, 1, $3)`
		_, err = tx.Exec(ctx, insertKey, roomID, k.DeviceID, k.EncryptedRoomKey)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "database", err, nil)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return web.NewError(http.StatusInternalServerError, "database_tx", err, nil)
	}

	w.Header().Set("HX-Redirect", "/chat/"+roomID.String())
	w.WriteHeader(http.StatusCreated)
	return nil
}
