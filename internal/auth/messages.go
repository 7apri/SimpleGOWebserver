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
