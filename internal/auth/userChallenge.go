package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/crypto"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func GenerateChallenge() (*email.GeneratedChallenge, error) {
	rawToken, err := crypto.GenerateRandomString(32)
	if err != nil {
		return nil, err
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, err
	}
	rawCode := fmt.Sprintf("%06d", n.Int64())

	return &email.GeneratedChallenge{
		ChallengeRaw: email.ChallengeRaw{
			RawToken: rawToken,
			RawCode:  rawCode,
		},
		ChallengeHash: email.ChallengeHash{
			TokenHash: crypto.HashString(rawToken),
			CodeHash:  crypto.HashString(rawCode),
		},
	}, nil
}

type ChallengeClaims struct {
	UserID     uuid.UUID `json:"uid"`
	Action     string    `json:"act"`
	MfaPending bool      `json:"mfa"`
	jwt.RegisteredClaims
}

func (s *secretWrap) issueChallengeClaims(w http.ResponseWriter, userID uuid.UUID, mfaPending bool, name string) error {
	duration := 15 * time.Minute
	expiry := time.Now().Add(duration)
	claims := ChallengeClaims{
		UserID:     userID,
		MfaPending: mfaPending,
		Action:     name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    consts.BrandName,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.challenge)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     name + "_claims",
		Value:    token,
		Path:     "/",
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}

func (h *AuthHandler) GetChallengeClaims(tokenStr string) (*ChallengeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &ChallengeClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.secret.challenge, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*ChallengeClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if claims.Issuer != consts.BrandName {
		return nil, ErrInvalidIssuer
	}

	return claims, nil
}

type ChallengeResult struct {
	UserID   uuid.UUID
	Correct  bool
	Attempts int
	Has2FA   bool
}

func (h *AuthHandler) verifyChallenge(r *http.Request, cType consts.UserChallengeType, tokenRaw, codeRaw string) (*ChallengeResult, *web.WebError) {
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
	challenge_delete AS (
		DELETE FROM user_challenges
		WHERE challenge_type = $1 
			AND token_hash = $2 
			AND (SELECT code_hash FROM challenge_lookup) = $3
		RETURNING user_id
	),
	challenge_failure AS (
		UPDATE user_challenges
		SET attempts = attempts + 1, 
			updated_at = NOW()
		WHERE challenge_type = $1 
			AND token_hash = $2 
			AND (SELECT code_hash FROM challenge_lookup) != $3
		RETURNING attempts
	)
	SELECT 
		COALESCE((SELECT user_id FROM challenge_lookup), '00000000-0000-0000-0000-000000000000'::uuid) as user_id,
		((SELECT code_hash FROM challenge_lookup) = $3) IS TRUE AS is_correct,
		COALESCE(
			(SELECT attempts FROM challenge_failure),
			(SELECT attempts FROM challenge_lookup),
			0
		) AS current_attempts,
		EXISTS (
			SELECT 1 FROM user_credentials
			WHERE user_id = (SELECT user_id FROM challenge_lookup) 
			AND kind = 'totp'
		) AS has_2fa;`

	res := &ChallengeResult{}
	err := h.db.Pool.QueryRow(r.Context(), q,
		cType,
		crypto.HashString(tokenRaw),
		crypto.HashString(codeRaw),
	).Scan(&res.UserID, &res.Correct, &res.Attempts, &res.Has2FA)
	if err != nil {
		return nil, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return res, nil
}

func (h *AuthHandler) ProcessChallengeVerification(w http.ResponseWriter, r *http.Request, cType consts.UserChallengeType, tokenRaw, action, code string) (*ChallengeResult, *web.WebError) {
	res, err := h.verifyChallenge(r, cType, tokenRaw, code)
	if err != nil {
		return nil, err
	}

	if !res.Correct {
		if res.Attempts >= 5 {
			return res, web.NewError(http.StatusUnauthorized, "too_many_attempts", nil, nil)
		}
		return res, web.NewError(http.StatusUnauthorized, "invalid_code", nil, map[string]int{
			"remaining_attempts": res.Attempts - 5,
		})
	}

	if err := h.secret.issueChallengeClaims(w, res.UserID, res.Has2FA, action); err != nil {
		return res, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return res, nil
}

func (h *AuthHandler) tryLock(ctx context.Context, challengeType consts.UserChallengeType, key string, ttl time.Duration) (int, bool) {
	redisKey := fmt.Sprintf("rl:%s:%s", challengeType, crypto.HashString(key))

	success, err := h.redis.SetNX(ctx, redisKey, "1", ttl).Result()
	if err != nil {
		return 0, false
	}

	if !success {
		t, _ := h.redis.TTL(ctx, redisKey).Result()
		return int(t.Seconds()), true
	}

	return 0, false
}

func (h *AuthHandler) GetValueWithTTL(ctx context.Context, key string) (string, time.Duration, error) {
	pipe := h.redis.Pipeline()

	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)

	_, err := pipe.Exec(ctx)
	if errors.Is(err, redis.Nil) {
		return "0", 0, nil
	}
	if err != nil && err != redis.Nil {
		return "", 0, err
	}

	val, err := getCmd.Result()
	if err != nil {
		return "", 0, err
	}
	ttl, err := ttlCmd.Result()
	if err != nil {
		return "", 0, err
	}

	return val, ttl, nil
}

func (h *AuthHandler) isLimited(ctx context.Context, challengeType consts.UserChallengeType, keys ...string) (int, bool) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		redisKey := fmt.Sprintf("rl:%s:%s", challengeType, crypto.HashString(key))
		ttl, _ := h.redis.TTL(ctx, redisKey).Result()
		if ttl > 0 {
			return int(ttl.Seconds()), true
		}
	}
	return 0, false
}

func (h *AuthHandler) setRateLimit(ctx context.Context, challengeType consts.UserChallengeType, key string, ttl time.Duration) {
	redisKey := fmt.Sprintf("rl:%s:%s", challengeType, crypto.HashString(key))
	h.redis.Set(ctx, redisKey, "1", ttl)
}

func (h *AuthHandler) setTokenCookie(
	ctx context.Context,
	w http.ResponseWriter,
	challenge *email.GeneratedChallenge,
	challengeType consts.UserChallengeType,
	rateLimit time.Duration,
	expirationSec int,
	identifier,
	cookieName string,
) {
	h.setRateLimitsEmail(ctx, challengeType, identifier, challenge.TokenHash, rateLimit)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    challenge.RawToken,
		Path:     "/",
		MaxAge:   expirationSec,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
func (h *AuthHandler) setRateLimitsEmail(
	ctx context.Context,
	challengeType consts.UserChallengeType,
	identifier,
	tokenHash string,
	rateLimit time.Duration,
) {
	if identifier != "" {
		h.setRateLimit(ctx, challengeType, identifier, rateLimit)
	}
	h.setRateLimit(ctx, challengeType, tokenHash, rateLimit)
}

func (h *AuthHandler) InitEmailChallenge(
	challengeType consts.UserChallengeType,
	rateLimit time.Duration,
	expiration time.Duration,
	cookieName string,
	requireUnverified bool,
	sender func(challenge email.ChallengeRaw, user email.UserDetail),
) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		var req struct {
			Email string `json:"email"`
		}
		err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req)

		ctx := r.Context()
		user := email.UserDetail{
			UserContact: email.UserContact{
				Email: strings.ToLower(strings.TrimSpace(req.Email)),
			},
		}

		var tokenHashReq sql.NullString
		if cookie, err := r.Cookie(cookieName); err == nil {
			tokenHashReq = sql.NullString{String: crypto.HashString(cookie.Value), Valid: true}
		} else {
			tokenHashReq = sql.NullString{Valid: false}
		}

		if retryAfter, limited := h.isLimited(ctx, challengeType, user.Email); limited {
			return web.NewError(http.StatusTooManyRequests, "too_many_requests_email", nil, map[string]int{"retry_after": retryAfter})
		}

		challenge, err := GenerateChallenge()
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}

		const q = `
		WITH target_user AS (
			SELECT u.id, u.username, u.email, u.preferred_lang, u.is_verified, uc.updated_at
			FROM users u
			LEFT JOIN user_challenges uc ON uc.user_id = u.id AND uc.challenge_type = $2
			WHERE (u.email = $1) OR ($1 = '' AND uc.token_hash = $5)
			AND u.deleted_at IS NULL 
			ORDER BY (u.email = $1) DESC
			LIMIT 1
		),
		attempt AS (
			INSERT INTO user_challenges (user_id, challenge_type, token_hash, code_hash, expires_at, updated_at, attempts)
			SELECT id, $2, $3, $4, NOW() + ($7 * INTERVAL '1 second'), NOW(), 0 FROM target_user
			WHERE (NOT $6 OR target_user.is_verified = FALSE)
			ON CONFLICT (user_id, challenge_type) 
			DO UPDATE SET
				token_hash = EXCLUDED.token_hash,
				code_hash  = EXCLUDED.code_hash,
				expires_at = EXCLUDED.expires_at,
				updated_at = EXCLUDED.updated_at,
				attempts   = 0
			WHERE user_challenges.updated_at < NOW() - ($8 * INTERVAL '1 second')
			RETURNING user_id
		)
		SELECT 
			username, email, preferred_lang, is_verified,
			(SELECT count(*) FROM attempt) > 0 as was_updated,
			COALESCE(EXTRACT(EPOCH FROM (NOW() - updated_at)), 999)::int as seconds_since_update
		FROM target_user;`

		var isVerified, wasUpdated bool
		var secondsSinceUpdate int

		expSec := max(int(expiration/time.Second), 1)
		ratSec := max(int(rateLimit/time.Second), 1)

		err = h.db.Pool.QueryRow(ctx, q,
			user.Email,          // $1
			challengeType,       // $2
			challenge.TokenHash, // $3
			challenge.CodeHash,  // $4
			tokenHashReq,        // $5
			requireUnverified,   // $6
			expSec,              // $7
			ratSec,              // $8
		).Scan(&user.Username, &user.Email, &user.Lang, &isVerified, &wasUpdated, &secondsSinceUpdate)

		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				h.setTokenCookie(ctx, w, challenge, challengeType, rateLimit, expSec, user.Email, cookieName)
				w.WriteHeader(http.StatusAccepted)
				return nil
			}
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}

		if requireUnverified && isVerified {
			h.setTokenCookie(ctx, w, challenge, challengeType, rateLimit, expSec, user.Email, cookieName)
			h.setRateLimitsEmail(ctx, challengeType, user.Email, challenge.TokenHash, rateLimit)
			w.WriteHeader(http.StatusAccepted)
			return nil
		}

		if !wasUpdated {
			return web.NewError(http.StatusTooManyRequests, "too_many_requests_email", nil, map[string]any{
				"retry_after": max(60-secondsSinceUpdate, 1),
			})
		}

		h.setTokenCookie(ctx, w, challenge, challengeType, rateLimit, expSec, user.Email, cookieName)
		h.setRateLimitsEmail(ctx, challengeType, user.Email, challenge.TokenHash, rateLimit)

		go sender(challenge.ChallengeRaw, user)

		w.WriteHeader(http.StatusAccepted)
		return nil
	}
}
