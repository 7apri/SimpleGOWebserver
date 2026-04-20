package auth

import (
	"errors"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/crypto"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *AuthHandler) CheckCodeReset(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}
	c, err := r.Cookie("reset_token")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	res, WebErr := h.verifyChallenge(r, consts.UserChallengeReset, c.Value, req.Code)
	if WebErr != nil {
		return WebErr
	}
	if res.UserID == uuid.Nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}
	if !res.Correct {
		if res.Attempts >= 5 {
			return web.NewError(http.StatusUnauthorized, "too_many_attempts", nil, nil)
		}
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, map[string]int{
			"remaining_attempts": res.Attempts - 5,
		})
	}

	err = h.secret.issueChallengeClaims(w, res.UserID, res.Has2FA, "reset")
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	status := "success"
	if res.Has2FA {
		status = "pending"
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	http.SetCookie(w, &http.Cookie{Name: "reset_token", MaxAge: -1, Path: "/"})
	return nil
}

func (h *AuthHandler) ConfirmReset(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, err := r.Cookie("reset_claims")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	claims, err := h.GetChallengeClaims(cookieT.Value)
	if err != nil || claims.Action != "reset" {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	if claims.MfaPending {
		return web.NewError(http.StatusUnauthorized, "pending_2fa", nil, nil)
	}

	var req struct {
		NewPassword string `json:"password"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}

	if err := validatePassword(req.NewPassword); err != nil {
		return web.NewError(http.StatusBadRequest, "password_invalid", err, nil)
	}

	newHash, err := crypto.HashCredential(req.NewPassword)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	const q = `
    WITH updated_user AS (
        UPDATE users
        SET is_verified = TRUE, updated_at = NOW()
        WHERE id = $1 AND deleted_at IS NULL
        RETURNING id, role, username, email, preferred_lang, avatar_url, updated_at
    ),
    update_password AS (
        INSERT INTO user_credentials (user_id, kind, secret)
        VALUES ($1, $2, $3)
        ON CONFLICT (user_id, kind) WHERE kind = $2
        DO UPDATE SET
            secret = EXCLUDED.secret,
            updated_at = NOW()
        RETURNING user_id
    ),
    delete_sessions AS (
        DELETE FROM refresh_sessions 
        WHERE user_id = $1
        AND EXISTS (SELECT 1 FROM update_password)
    )   
    SELECT 
        u.id, u.role, u.username, u.email, u.preferred_lang, u.avatar_url, u.updated_at
    FROM updated_user u
    WHERE EXISTS (SELECT 1 FROM update_password);`

	var (
		u            UserPrintTimestamp
		uEmail, lang string
	)

	err = h.db.Pool.QueryRow(r.Context(), q,
		claims.UserID,
		consts.UserCredentialsPassword,
		newHash,
	).Scan(&u.ID, &u.Role, &u.Username, &uEmail, &lang, &u.AvatarURL, &u.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	web.ClearCookies(w, "/", "reset_claims", "reset_token")

	go h.EmailManager.SendSecurityPasswordReset(email.UserDetail{
		UserContact: email.UserContact{
			Username: u.Username,
			Email:    uEmail,
		},
		Lang: lang,
	})

	if err := h.issueTokens(w, r, &u, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
