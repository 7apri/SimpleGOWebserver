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
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	res, err := h.verifyChallenge(r, email.ChallengeReset, "reset_token", req.Code)
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
		Name:  "reset_code_tmp",
		Value: req.Code,
		Path:  "/", MaxAge: 300, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	status := "success"
	if res.Has2FA {
		access, exp, err := h.secret.GenerateAccess(res.User, true)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "access_token",
			Value:    access,
			Expires:  exp,
			HttpOnly: true,
			Secure:   true,
			Path:     "/",
			SameSite: http.SameSiteLaxMode,
		})
		status = "pending"
	}

	w.Header().Set("Content-Type", "application/json")
	e := sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
	if e != nil {
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
	),
	upsert_cred AS (
		INSERT INTO user_credentials (user_id, kind, secret, updated_at)
		SELECT id, $4, $5, NOW() FROM updated_user
		ON CONFLICT (user_id, kind) WHERE kind IN ('passkey', 'totp')
		DO UPDATE SET
			secret = EXCLUDED.secret, 
			updated_at = NOW()
	)
	SELECT 
		(SELECT id FROM updated_user) IS NOT NULL AS found,
		COALESCE(
			(SELECT attempts FROM challenge_failure),
			(SELECT attempts FROM challenge_target)
		) AS current_attempts,
		u.id, 
		u.role, 
		u.username
	FROM (SELECT 1) AS dummy  
	LEFT JOIN updated_user u ON TRUE;`

	var (
		found           bool
		currentAttempts sql.NullInt64
		userID          uuid.NullUUID
		userRole        sql.NullString
		userName        sql.NullString
	)
	ctx := r.Context()

	err = h.db.Pool.QueryRow(ctx, q,
		email.ChallengeReset,
		HashString(cookieT.Value),
		HashString(cookieC.Value),
		UserCredentialsPassword,
		newHash,
	).Scan(&found, &currentAttempts, &userID, &userRole, &userName)

	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if !found {
		return web.NewError(http.StatusGone, "session_expired", nil, nil)
	}

	if currentAttempts.Int64 > 5 {
		return web.NewError(http.StatusTooManyRequests, "too_many_attempts", nil, nil)
	}

	if !userID.Valid {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, map[string]any{
			"remaining_attempts": max(0, 5-int(currentAttempts.Int64)),
		})
	}

	http.SetCookie(w, &http.Cookie{Name: "reset_token", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "reset_code_tmp", MaxAge: -1, Path: "/"})

	if err := h.issueTokens(w, r, &UserPrint{
		ID:       userID.UUID,
		Role:     userRole.String,
		Username: userName.String,
	}, true); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
