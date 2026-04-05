package auth

import (
	"errors"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/email"
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

	res, err := h.verifyChallenge(r, email.ChallengeVerify, "verify_token", req.Code)
	if err != nil {
		return err
	}

	if !res.Correct {
		if res.Attempts >= 5 {
			return web.NewError(http.StatusUnauthorized, "too_many_attempts", nil, nil)
		}
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "verify_code_tmp",
		Value: req.Code,
		Path:  "/", MaxAge: 300, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func (h *AuthHandler) ConfirmVerify(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, errT := r.Cookie("verify_token")
	cookieC, errC := r.Cookie("verify_code_tmp")

	if errT != nil || errC != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	const q = `
	WITH challenge_lookup AS (
		SELECT user_id, code_hash, attempts
		FROM user_challenges
		WHERE challenge_type = $1
			AND token_hash = $2
			AND expires_at > NOW()
			AND attempts < 5
		FOR UPDATE
	),
	verification_success AS (
		DELETE FROM user_challenges
		WHERE challenge_type = $1
			AND token_hash = $2
			AND (SELECT code_hash FROM challenge_lookup) = $3
		RETURNING user_id
	),
	verification_failure AS (
		UPDATE user_challenges
		SET attempts = attempts + 1, 
			updated_at = NOW()
		WHERE challenge_type = $1
		AND token_hash = $2
		AND (SELECT code_hash FROM challenge_lookup) != $3
	),
	updated_user AS (
		UPDATE users
		SET is_verified = TRUE
		WHERE id = (SELECT user_id FROM verification_success)
		RETURNING id, role, username
	)
	SELECT id, role, username FROM updated_user;`

	var user UserPrint
	err := h.db.Pool.QueryRow(r.Context(), q,
		email.ChallengeVerify,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
	).Scan(&user.ID, &user.Role, &user.Username)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "verification_failed", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	http.SetCookie(w, &http.Cookie{Name: "verify_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "verify_code_tmp", MaxAge: -1, Path: "/"})

	if err = h.issueTokens(w, r, &user, true); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
