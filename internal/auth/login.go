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
	Remember   bool   `json:"remember"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req loginRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}

	var (
		user           UserPrint
		isVerified     bool
		passHash       sql.NullString
		has2FA         bool
		oauthProviders []string
	)

	const q = `
	SELECT 
		u.id, 
		u.role,
		u.username,
		u.is_verified,
		c.secret AS password_hash,
		EXISTS (SELECT 1 FROM user_credentials WHERE user_id = u.id AND kind = $2) AS has_two_fa,
		COALESCE(
			(SELECT array_agg(kind) 
			FROM user_credentials 
			WHERE user_id = u.id AND kind NOT IN ($2, $3, $4)), 
			'{}'
		) AS oauth_providers
	FROM users u
	LEFT JOIN user_credentials c ON c.user_id = u.id AND c.kind = $4
	WHERE (u.email = $1 OR u.username = $1) 
	AND u.deleted_at IS NULL
	LIMIT 1;`

	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))

	err := h.db.Pool.QueryRow(r.Context(), q, identifier, UserCredentials2FA, UserCredentialsRecovery, UserCredentialsPassword).Scan(
		&user.ID,
		&user.Role,
		&user.Username,
		&isVerified,
		&passHash,
		&has2FA,
		&oauthProviders,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "invalid_credentials", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}
	if !isVerified {
		return web.NewError(http.StatusForbidden, "account_not_verified", nil, nil)
	}
	if !passHash.Valid {
		return web.NewError(http.StatusForbidden, "use_oauth", nil, map[string]any{
			"allowed": oauthProviders,
		})
	}

	match, err := VerifyCredential(req.Password, passHash.String)
	if err != nil || !match {
		return web.NewError(http.StatusUnauthorized, "invalid_credentials", nil, nil)
	}

	var status string
	if has2FA {
		err = h.issueAccessToken(w, &user, AccessTokenOptions{
			IsPending: true,
			Remember:  req.Remember,
		})
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "pending"
	} else {
		if err := h.issueTokens(w, r, &user, TokenOptions{
			RotateCSRF: true,
			SendEmail:  true,
			AccessTokenOptions: AccessTokenOptions{
				Remember: req.Remember,
			},
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "success"
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
