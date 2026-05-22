package auth

import (
	"database/sql"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
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

	remember := false
	if c, err := r.Cookie("verify_remember"); err == nil {
		remember = (c.Value == "true")
	}

	const q = `
    WITH challenge_target AS (
        SELECT user_id, code_hash, attempts
        FROM user_challenges
        WHERE challenge_type = $1
            AND token_hash = $2
            AND expires_at > NOW()
            AND attempts < 5
        FOR UPDATE
    ),
    challenge_success AS (
        DELETE FROM user_challenges
        WHERE user_id = (SELECT user_id FROM challenge_target WHERE code_hash = $3)
            AND challenge_type = $1
        RETURNING user_id
    ),
    challenge_failure AS (
        UPDATE user_challenges
        SET attempts = attempts + 1,
            updated_at = NOW()
        WHERE user_id = (SELECT user_id FROM challenge_target WHERE code_hash != $3)
        AND challenge_type = $1
        RETURNING attempts
    ),
    updated_user AS (
        UPDATE users
        SET is_verified = TRUE
        WHERE id = (SELECT user_id FROM challenge_success)
        RETURNING id, role, username
    )
    SELECT 
        (SELECT user_id FROM challenge_target) IS NOT NULL AS found,
        COALESCE(
            (SELECT attempts FROM challenge_failure),
            (SELECT attempts FROM challenge_target),
            0
        ) AS current_attempts,
        u.id,
		u.role,
		u.username
    FROM (SELECT 1) AS dummy  
    LEFT JOIN updated_user u ON TRUE;`

	var (
		found           bool
		currentAttempts int
		u               UserPrint
	)

	var (
		nullID       uuid.NullUUID
		nullRole     sql.NullString
		nullUsername sql.NullString
	)

	err := h.db.Pool.QueryRow(r.Context(), q,
		email.ChallengeVerify,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
	).Scan(
		&found,
		&currentAttempts,
		&nullID,
		&nullRole,
	)

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database", err, nil)
	}

	if !found {
		return web.NewError(http.StatusGone, "session_expired", nil, nil)
	}

	if !nullID.Valid {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, map[string]any{
			"remaining_attempts": max(0, 5-currentAttempts),
		})
	}

	u.ID = nullID.UUID
	u.Role = nullRole.String
	u.Username = nullUsername.String

	http.SetCookie(w, &http.Cookie{Name: "verify_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "verify_code_tmp", MaxAge: -1, Path: "/"})

	if err := h.issueTokens(w, r, &u, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
		AccessTokenOptions: AccessTokenOptions{
			Remember: remember,
		},
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
