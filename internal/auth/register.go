package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req registerRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "err:invalid_json", err, nil)
	}
	ctx := r.Context()

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return web.NewError(http.StatusBadRequest, "err:email_invalid", nil, nil)
	}
	if len(req.Password) < 8 {
		return web.NewError(http.StatusBadRequest, "err:password_too_short", nil, nil)
	}

	if ttl, limited := h.tryLock(ctx, email.ChallengeVerify, req.Email, time.Minute); limited {
		return web.NewError(http.StatusTooManyRequests, "err:too_many_requests_email", nil, map[string]any{"retry_after": ttl})
	}

	hashed, err := HashPassword(req.Password)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}
	challenge, err := GenerateChallenge()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	lang := i18n.GetLangFromReq(r)

	const registerQuery = `
    WITH new_user AS (
        INSERT INTO users (email, username, preferred_lang, is_verified)
        VALUES ($1, $2, $3, FALSE)
        RETURNING id
    ),
    new_credentials AS (
        INSERT INTO user_credentials (user_id, kind, secret)
        SELECT id, $4, $5 FROM new_user
    )
    INSERT INTO user_challenges (user_id, challenge_type, code_hash, token_hash, expires_at)
    SELECT id, $6, $7, $8, NOW() + INTERVAL '15 minutes' FROM new_user
    RETURNING user_id;`

	var userID uuid.UUID
	const maxAttempts = 5

	userDetail := email.UserDetail{
		Lang: lang,
		UserContact: email.UserContact{
			Email:    req.Email,
			Username: req.Username,
		},
	}

	for i := range maxAttempts {
		err = h.db.Pool.QueryRow(ctx, registerQuery,
			strings.ToLower(req.Email), // $1
			userDetail.Username,        // $2
			lang,                       // $3
			UserCredentialsPassword,    // $4 ('kind')
			hashed,                     // $5 ('secret')
			email.ChallengeVerify,      // $6
			challenge.CodeHash,         // $7
			challenge.TokenHash,        // $8
		).Scan(&userID)

		if err == nil {
			h.setTokenCookie(ctx, w, challenge, email.ChallengeVerify, time.Minute, 900, req.Email, "verify_token")

			go h.EmailManager.SendVerificationEmail(challenge.ChallengeRaw, userDetail)
			w.WriteHeader(http.StatusAccepted)
			return nil
		}

		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "email") {
				var existingUsername string
				_ = h.db.Pool.QueryRow(ctx, "SELECT username FROM users WHERE email = $1", req.Email).Scan(&existingUsername)

				if existingUsername != "" {
					userDetail.Username = existingUsername
				}

				h.setTokenCookie(ctx, w, challenge, email.ChallengeVerify, time.Minute, 900, req.Email, "verify_token")
				go h.EmailManager.SendAccountExistsEmail(userDetail)
				w.WriteHeader(http.StatusAccepted)
				return nil
			}
			if strings.Contains(pgErr.ConstraintName, "username") {
				switch i {
				case 0, 1:
					userDetail.Username = fmt.Sprintf("%s%d", req.Username, i+1)
				case 2:
					userDetail.Username = fmt.Sprintf("%s%d", req.Username, util.RandomInt(99))
				default:
					userDetail.Username = fmt.Sprintf("%s-%d", req.Username, time.Now().Unix()%1000)
				}
				continue
			}
		}

		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}
	return web.NewError(http.StatusConflict, "err:username_unavailable", nil, nil)
}
