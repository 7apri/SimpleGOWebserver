package auth

import (
	"errors"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5"
)

func (h *AuthHandler) CheckCodeVerify(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	c, err := r.Cookie("verify_token")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	_, Werr := h.ProcessChallengeVerification(w, r, consts.UserChallengeVerify, c.Value, "verify", req.Code)

	if Werr == nil {
		http.SetCookie(w, &http.Cookie{Name: "verify_token", MaxAge: -1, Path: "/"})
	}

	return Werr
}

func (h *AuthHandler) ConfirmVerify(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, errT := r.Cookie("verify_claims")
	if errT != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	claims, err := h.GetChallengeClaims(cookieT.Value)
	if err != nil || claims.Action != "verify" {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	remember := false
	if c, err := r.Cookie("verify_remember"); err == nil {
		remember = (c.Value == "true")
	}

	var user UserPrintTimestamp

	const q = `
     UPDATE users
        SET is_verified = TRUE
        WHERE id = $1
        RETURNING id, role, username, avatar_url, updated_at;`

	err = h.db.Pool.QueryRow(r.Context(), q, claims.UserID).Scan(
		&user.ID,
		&user.Role,
		&user.Username,
		&user.AvatarURL,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "user_not_found", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	web.ClearCookies(w, "/", "verify_claims", "verify_remember")

	if err := h.issueTokens(w, r, &user, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
		AccessTokenOptions: AccessTokenOptions{
			Remember: remember,
		},
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
