package auth

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"image/color"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"github.com/skip2/go-qrcode"
	"golang.org/x/sync/errgroup"
	"golang.org/x/text/unicode/norm"
)

func GenerateQRCodeBase64(otpAuthURL string) (string, error) {
	q, err := qrcode.New(otpAuthURL, qrcode.Medium)
	if err != nil {
		return "", err
	}
	q.BackgroundColor = color.Transparent
	q.ForegroundColor = color.Black
	var png []byte
	png, err = q.PNG(256)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(png), nil
}

func (h *AuthHandler) HandleInit2FA(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	user, ok := GetUser(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "panels",
		AccountName: user.Username,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	encryptedSecret, err := Encrypt([]byte(key.Secret()), h.secret.twoFactor)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	encryptedSecretHex := hex.EncodeToString(encryptedSecret)
	err = h.redis.Set(ctx, "2fa_pending:"+user.ID.String(), encryptedSecretHex, 10*time.Minute).Err()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	qrBase64, err := GenerateQRCodeBase64(key.URL())
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"secret":  key.Secret(),
		"qr_code": qrBase64,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}

const (
	UserCredentials2FA      = "totp"
	UserCredentialsRecovery = "recovery_code"
)

var (
	recoveryReplacer = strings.NewReplacer("—", "-", "–", "-", " ", "-")
)

func sanitizeRecoveryCode(input string) string {
	normalized := norm.NFC.String(input)

	lowered := strings.ToLower(normalized)

	replaced := recoveryReplacer.Replace(lowered)

	parts := strings.FieldsFunc(replaced, func(r rune) bool {
		return r == '-'
	})

	if len(parts) != 3 {
		return ""
	}

	return strings.Join(parts, "-")
}
func (h *AuthHandler) generateNewRecoveryCodes(ctx context.Context, lang string) (plainRecoveryCodes []string, hashes []string, err error) {
	words, err := h.i18nManager.PickWords(lang, 30)
	if err != nil {
		return nil, nil, err
	}

	plainRecoveryCodes = make([]string, 0, 10)

	for i := 0; i < 30; i += 3 {
		code := strings.Join(words[i:i+3], "-")
		plainRecoveryCodes = append(plainRecoveryCodes, sanitizeRecoveryCode(code))
	}

	g, groupCtx := errgroup.WithContext(ctx)
	hashes = make([]string, 10)
	g.SetLimit(runtime.GOMAXPROCS(0))

	for i, code := range plainRecoveryCodes {
		i, code := i, code
		g.Go(func() error {
			if groupCtx.Err() != nil {
				return nil
			}
			h, err := HashCredential(code)
			if err != nil {
				return err
			}
			hashes[i] = string(h)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return plainRecoveryCodes, hashes, nil
}
func writeRecoveryCodesResponse(w http.ResponseWriter, plainRecoveryCodes []string) *web.WebError {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	err := sonic.ConfigDefault.NewEncoder(w).Encode(map[string][]string{
		"recovery_codes": plainRecoveryCodes,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}
func (h *AuthHandler) HandleVerifyAndEnable2FA(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	ctx := r.Context()
	user, ok := GetUser(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}
	redisKey := "2fa_pending:" + user.ID.String()

	encryptedHexStr, err := h.redis.Get(ctx, redisKey).Result()
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	cipherText, err := hex.DecodeString(encryptedHexStr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	decryptedSecret, err := Decrypt(cipherText, h.secret.twoFactor)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if !totp.Validate(req.Code, string(decryptedSecret)) {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	b := &pgx.Batch{}

	b.Queue(`
	INSERT INTO user_credentials (user_id, kind, secret) VALUES ($1, $2, $3)
	ON CONFLICT (user_id, kind) WHERE kind IN ('passkey', 'totp')
	DO UPDATE SET
		secret = EXCLUDED.secret, 
		updated_at = NOW()
	`, user.ID, UserCredentials2FA, encryptedHexStr)

	lang := i18n.GetLangFromReq(r)
	plainRecoveryCodes, hashes, err := h.generateNewRecoveryCodes(ctx, lang)

	for _, hash := range hashes {
		b.Queue(`
		INSERT INTO user_credentials (user_id, kind, secret) VALUES ($1, $2, $3)`, user.ID, UserCredentialsRecovery, string(hash))
	}

	br := h.db.Pool.SendBatch(ctx, b)
	if _, err := br.Exec(); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	br.Close()

	h.redis.Del(ctx, redisKey)
	return writeRecoveryCodesResponse(w, plainRecoveryCodes)
}
func (h *AuthHandler) HandleLoginVerify2FA(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	ctx := r.Context()
	claims, ok := GetClaims(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	redisKey := "2fa_fail:" + claims.User.ID.String()

	failsStr, ttl, err := h.GetValueWithTTL(ctx, redisKey)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	if attempts, err := strconv.Atoi(failsStr); err != nil || attempts >= 5 {
		return web.NewError(http.StatusTooManyRequests, "too_many_attempts", err, map[string]int{"retry_after": int(ttl.Seconds())})
	}

	var encryptedHexStr string
	err = h.db.Pool.QueryRow(ctx, "SELECT secret FROM user_credentials WHERE user_id = $1 AND kind = $2", claims.User.ID, UserCredentials2FA).Scan(&encryptedHexStr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)

	}

	cipherText, err := hex.DecodeString(encryptedHexStr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	decryptedSecret, err := Decrypt(cipherText, h.secret.twoFactor)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)

	}

	if !totp.Validate(req.Code, string(decryptedSecret)) {
		h.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Incr(ctx, redisKey)
			pipe.Expire(ctx, redisKey, 15*time.Minute)
			return nil
		})
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	h.redis.Del(ctx, redisKey)
	if err := h.issueTokens(w, r, claims.User, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
		AccessTokenOptions: AccessTokenOptions{
			Remember: claims.RememberMe,
		},
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}

func (h *AuthHandler) VerifyRecoveryCode(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}
	userInput := sanitizeRecoveryCode(req.Code)
	if userInput == "" {
		return web.NewError(http.StatusBadRequest, "invalid_code", nil, nil)
	}

	ctx := r.Context()
	claims, ok := GetClaims(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	rows, err := h.db.Pool.Query(ctx,
		`SELECT id, secret FROM user_credentials WHERE user_id = $1 AND kind = $2`,
		claims.User.ID, UserCredentialsRecovery,
	)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	defer rows.Close()

	var matchedID uuid.UUID
	var allIDs []uuid.UUID

	for rows.Next() {
		var id uuid.UUID
		var storedHash string
		if err := rows.Scan(&id, &storedHash); err != nil {
			continue
		}
		allIDs = append(allIDs, id)

		if matchedID == uuid.Nil {
			if valid, _ := VerifyCredential(userInput, storedHash); valid {
				matchedID = id
			}
		}
	}

	if matchedID == uuid.Nil {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	remainingCodes := len(allIDs) - 1

	if matchedID == uuid.Nil {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	_, err = h.db.Pool.Exec(ctx, `DELETE FROM user_credentials WHERE id = $1`, matchedID)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if err := h.issueTokens(w, r, claims.User, TokenOptions{
		RotateCSRF: true,
		SendEmail:  true,
		AccessTokenOptions: AccessTokenOptions{
			Remember: claims.RememberMe,
		},
	}); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(map[string]int{
		"remaining": remainingCodes,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
func (h *AuthHandler) HandleRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	user, ok := GetUser(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	lang := i18n.GetLangFromReq(r)
	plainCodes, hashes, err := h.generateNewRecoveryCodes(ctx, lang)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM user_credentials WHERE user_id = $1 AND kind = $2`,
		user.ID, UserCredentialsRecovery)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_credentials (user_id, kind, secret)
		SELECT $1, $2, unnest($3::text[])`,
		user.ID, UserCredentialsRecovery, hashes,
	)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if err := tx.Commit(ctx); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return writeRecoveryCodesResponse(w, plainCodes)
}
