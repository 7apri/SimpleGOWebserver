package auth

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var ErrPasswordShort = errors.New("password_too_short")
var ErrPasswordLong = errors.New("password_too_long")
var ErrPasswordSimple = errors.New("password_too_simple")

func validatePassword(password string) error {
	if len(password) < 10 {
		return ErrPasswordShort
	}
	if len(password) > 72 {
		return ErrPasswordLong
	}

	var (
		hasUpper  bool
		hasDigit  bool
		hasSymbol bool
	)

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}

		if hasUpper && hasDigit && hasSymbol {
			return nil
		}
	}
	return ErrPasswordSimple
}

func (h *AuthHandler) GetAvailableSuggestions(ctx context.Context, lang, base string) ([]string, error) {
	candidates := make([]string, 0, 5)

	candidates = append(candidates, fmt.Sprintf("%s-%03d", base, rand.Intn(1000)))
	if usernames, err := h.i18nManager.GetUsernames(lang, base, 3); err == nil {
		candidates = append(candidates, usernames...)
	}
	candidates = append(candidates, base+strconv.Itoa(rand.Intn(1000)))

	rows, err := h.db.Pool.Query(ctx, "SELECT username FROM users WHERE username = ANY($1)", candidates)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	taken := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			taken[strings.ToLower(name)] = struct{}{}
		}
	}

	available := make([]string, 0, 3)
	for _, c := range candidates {
		if _, ok := taken[strings.ToLower(c)]; !ok {
			available = append(available, c)
		}
		if len(available) >= 3 {
			break
		}
	}

	return available, nil
}
func (h *AuthHandler) VerifyUsername(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Username string
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || strings.Contains(req.Username, "@") {
		return web.NewError(http.StatusBadRequest, "username_invalid", nil, nil)
	}

	var exists bool
	ctx := r.Context()
	err := h.db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", req.Username).Scan(&exists)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database_error", err, nil)
	}
	if !exists {
		return nil
	}

	suggestions, err := h.GetAvailableSuggestions(ctx, i18n.GetLangFromReq(r), req.Username)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "database_error", err, nil)
	}

	return web.NewError(http.StatusConflict, "username_unavailable", nil, suggestions)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req registerRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}
	ctx := r.Context()

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		return web.NewError(http.StatusBadRequest, "email_invalid", nil, nil)
	}
	if err := validatePassword(req.Password); err != nil {
		return web.NewError(http.StatusBadRequest, "", err, nil)
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || strings.Contains(req.Username, "@") {
		return web.NewError(http.StatusBadRequest, "username_invalid", nil, nil)
	}
	if ttl, limited := h.tryLock(ctx, email.ChallengeVerify, req.Email, time.Minute); limited {
		return web.NewError(http.StatusTooManyRequests, "too_many_requests_email", nil, map[string]any{"retry_after": ttl})
	}

	hashed, err := HashCredential(req.Password)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	challenge, err := GenerateChallenge()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
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
		return web.NewError(http.StatusConflict, "username_unavailable", nil, nil)
	}

	return web.NewError(http.StatusInternalServerError, "internal", err, nil)
}
