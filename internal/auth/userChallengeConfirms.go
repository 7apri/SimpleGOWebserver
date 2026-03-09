package auth

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *AuthHandler) ConfirmVerify(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, errT := r.Cookie("verify_token")
	cookieC, errC := r.Cookie("verify_code_tmp")

	if errT != nil || errC != nil {
		return web.NewError(http.StatusUnauthorized, "err:session_expired", nil, nil)
	}

	const q = `
	WITH increment_attempt AS (
		UPDATE user_challenges
		SET attempts = attempts + 1,
			updated_at = NOW()
		WHERE challenge_type = $1
			AND token_hash = $2
			AND expires_at > NOW()
			AND attempts < 5
		RETURNING user_id, code_hash, attempts
	),
	verification AS (
		SELECT user_id FROM increment_attempt 
		WHERE code_hash = $3
	),
	updated_user AS (
		UPDATE users
		SET is_verified = TRUE
		WHERE id = (SELECT user_id FROM verification)
		RETURNING id, role
	),
	cleanup AS (
		DELETE FROM user_challenges 
		WHERE user_id = (SELECT id FROM updated_user) 
		AND challenge_type = $1
	)
	SELECT id, role FROM updated_user;`

	var user UserPrint
	err := h.db.Pool.QueryRow(r.Context(), q,
		email.ChallengeVerify,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
	).Scan(&user.ID, &user.Role)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return web.NewError(http.StatusUnauthorized, "err:verification_failed", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	http.SetCookie(w, &http.Cookie{Name: "verify_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "verify_code_tmp", MaxAge: -1, Path: "/"})

	if err = h.issueTokens(w, r, &user); err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	return nil
}

func (h *AuthHandler) ConfirmReset(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, errT := r.Cookie("reset_token")
	cookieC, errC := r.Cookie("reset_code_tmp")

	if errT != nil || errC != nil {
		return web.NewError(http.StatusUnauthorized, "err:session_expired", nil, nil)
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "err:invalid_json", err, nil)
	}

	newHash, err := HashPassword(req.NewPassword)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	const q = `
	WITH target AS (
		UPDATE user_challenges
		SET attempts = attempts + 1,
			updated_at = NOW()
		WHERE challenge_type = $1 
		AND token_hash = $3
		AND expires_at > NOW()
		RETURNING user_id, code_hash, attempts
	),
	validation AS (
		UPDATE users
		SET is_verified = TRUE
		WHERE id = (SELECT user_id FROM target WHERE code_hash = $4 AND attempts <= 5)
		RETURNING id, role
	),
	upsert_cred AS (
		INSERT INTO user_credentials (user_id, kind, secret, updated_at)
		SELECT id, $2, $5, NOW() FROM validation
		ON CONFLICT (user_id, kind) DO UPDATE SET 
			secret = EXCLUDED.secret, 
			updated_at = NOW()
		RETURNING user_id
	)
	SELECT 
		(SELECT user_id FROM target) IS NOT NULL as found,
		(SELECT attempts FROM target) as current_attempts,
		v.id, 
		v.role
	FROM (SELECT 1) AS dummy
	LEFT JOIN validation v ON TRUE;`

	var (
		found           bool
		currentAttempts sql.NullInt64
		userID          uuid.NullUUID
		userRole        sql.NullString
	)
	ctx := r.Context()

	err = h.db.Pool.QueryRow(ctx, q,
		email.ChallengeReset,
		UserCredentialsPassword,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
		newHash,
	).Scan(&found, &currentAttempts, &userID, &userRole)

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	if !found {
		return web.NewError(http.StatusGone, "err:session_expired", nil, nil)
	}

	if currentAttempts.Int64 > 5 {
		return web.NewError(http.StatusTooManyRequests, "err:too_many_attempts", nil, nil)
	}

	if !userID.Valid {
		return web.NewError(http.StatusUnauthorized, "err:invalid_code", nil, map[string]any{
			"remaining_attempts": max(0, 5-int(currentAttempts.Int64)),
		})
	}

	http.SetCookie(w, &http.Cookie{Name: "reset_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "reset_code_tmp", MaxAge: -1, Path: "/"})

	if err := h.issueTokens(w, r, &UserPrint{
		ID:   userID.UUID,
		Role: userRole.String,
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
