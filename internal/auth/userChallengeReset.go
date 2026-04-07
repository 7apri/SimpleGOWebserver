package auth

import (
	"database/sql"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

func (h *AuthHandler) CheckCodeReset(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}

	res, WebErr := h.verifyChallenge(r, email.ChallengeReset, "reset_token", req.Code)
	if WebErr != nil {
		return WebErr
	}
	if !res.Correct {
		if res.Attempts >= 5 {
			return web.NewError(http.StatusUnauthorized, "too_many_attempts", nil, nil)
		}
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "reset_code_tmp",
		Value: req.Code,
		Path:  "/", MaxAge: 300, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	var status string
	if res.Has2FA {
		err := h.issueAccessToken(w, res.User, AccessTokenOptions{
			IsPending: true,
			Remember:  false,
		})
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "pending"
	} else {
		if err := h.issueTokens(w, r, res.User, TokenOptions{
			RotateCSRF: true,
			SendEmail:  true,
			AccessTokenOptions: AccessTokenOptions{
				Remember: false,
			},
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "success"
	}

	w.Header().Set("Content-Type", "application/json")
	err := sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}

func (h *AuthHandler) ConfirmReset(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookieT, errT := r.Cookie("reset_token")
	cookieC, errC := r.Cookie("reset_code_tmp")

	if errT != nil || errC != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	var req struct {
		NewPassword string `json:"password"`
	}

	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}

	if err := validatePassword(req.NewPassword); err != nil {
		return web.NewError(http.StatusBadRequest, "", err, nil)
	}

	newHash, err := HashCredential(req.NewPassword)

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	const q = `
    WITH challenge_target AS (
        SELECT user_id, code_hash, attempts
        FROM user_challenges
        WHERE challenge_type = $1
            AND token_hash = $2
            AND expires_at > NOW()
        FOR UPDATE
    ),
    challenge_success AS (
        DELETE FROM user_challenges
        WHERE user_id IN (SELECT user_id FROM challenge_target WHERE code_hash = $3)
            AND challenge_type = $1
        RETURNING user_id
    ),
    challenge_failure AS (
        UPDATE user_challenges
        SET attempts = attempts + 1,
            updated_at = NOW()
        WHERE user_id IN (SELECT user_id FROM challenge_target WHERE code_hash != $3)
        AND challenge_type = $1
        RETURNING attempts
    ),
    updated_user AS (
        UPDATE users
        SET is_verified = TRUE
        WHERE id IN (SELECT user_id FROM challenge_success)
        RETURNING id, role, username, avatar_url, updated_at
    ),
	insert_cred AS (
		INSERT INTO user_credentials (user_id, kind, secret)
    	SELECT id, $4, $5 FROM updated_user
		ON CONFLICT (user_id, kind) WHERE kind = $4
		DO UPDATE SET
			secret = EXCLUDED.secret
	)
    SELECT 
        EXISTS(SELECT 1 FROM challenge_target) AS found,
        COALESCE(
            (SELECT attempts FROM challenge_failure LIMIT 1),
            (SELECT attempts FROM challenge_target  LIMIT 1),
            0
        ) AS current_attempts,
        u.id, 
        u.role, 
        u.username,
        u.avatar_url,
        u.updated_at
    FROM (SELECT 1) AS dummy  
    LEFT JOIN updated_user u ON TRUE;`

	var (
		found           bool
		currentAttempts int
		u               UserPrintTimestamp
	)

	var (
		nullID        uuid.NullUUID
		nullRole      sql.NullString
		nullName      sql.NullString
		nullAvatar    sql.NullString
		nullUpdatedAt sql.NullTime
	)

	err = h.db.Pool.QueryRow(r.Context(), q,
		email.ChallengeReset,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
		UserCredentialsPassword,
		newHash,
	).Scan(
		&found,
		&currentAttempts,
		&nullID,
		&nullRole,
		&nullName,
		&nullAvatar,
		&nullUpdatedAt,
	)

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if !found {
		return web.NewError(http.StatusGone, "session_expired", nil, nil)
	}

	if currentAttempts >= 5 {
		return web.NewError(http.StatusUnauthorized, "too_many_attempts", nil, map[string]int{
			"remaining_attempts": 0,
		})
	}

	if !nullID.Valid {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, map[string]int{
			"remaining_attempts": max(0, 5-currentAttempts),
		})
	}

	u.ID = nullID.UUID
	u.Role = nullRole.String
	u.Username = nullName.String
	u.AvatarURL = nullAvatar.String
	u.UpdatedAt = nullUpdatedAt.Time

	http.SetCookie(w, &http.Cookie{Name: "reset_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "reset_code_tmp", MaxAge: -1, Path: "/"})

	if err := h.issueTokens(w, r, &u, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
