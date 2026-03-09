package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req loginRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "err:invalid_json", err, nil)
	}

	var (
		user           UserPrint
		isVerified     bool
		passHash       sql.NullString
		oauthProviders []string
	)

	const q = `
    SELECT 
        u.id, u.role, u.is_verified,
        (SELECT secret FROM user_credentials WHERE user_id = u.id AND kind = $2) as password_hash,
        (SELECT COALESCE(array_agg(kind), '{}') FROM user_credentials WHERE user_id = u.id AND kind != $2) as oauth_providers
    FROM users u
    WHERE (u.email = $1 OR u.username = $1) AND u.deleted_at IS NULL`

	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))

	err := h.db.Pool.QueryRow(r.Context(), q, identifier, UserCredentialsPassword).Scan(
		&user.ID,
		&user.Role,
		&isVerified,
		&passHash,
		&oauthProviders,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "err:invalid_credentials", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	if !passHash.Valid {
		return web.NewError(http.StatusForbidden, "err:use_oauth", nil, map[string]any{
			"allowed": oauthProviders,
		})
	}

	match, err := VerifyPassword(req.Password, passHash.String)
	if err != nil || !match {
		return web.NewError(http.StatusUnauthorized, "err:invalid_credentials", nil, nil)
	}

	if !isVerified {
		return web.NewError(http.StatusForbidden, "err:account_not_verified", nil, nil)
	}

	if err := h.issueTokens(w, r, &user); err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	return nil
}
