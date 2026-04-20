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

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/crypto"
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

func (h *AuthHandler) validateUserTOTP(ctx context.Context, userID uuid.UUID, code string) *web.WebError {
	redisKey := "2fa_fail:" + userID.String()

	failsStr, ttl, err := h.GetValueWithTTL(ctx, redisKey)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	if attempts, err := strconv.Atoi(failsStr); err == nil && attempts >= 5 {
		return web.NewError(http.StatusTooManyRequests, "too_many_attempts", nil, map[string]int{"retry_after": int(ttl.Seconds())})
	}

	var encryptedHexStr string
	err = h.db.Pool.QueryRow(ctx, "SELECT secret FROM user_credentials WHERE user_id = $1 AND kind = $2", userID, consts.UserCredentials2FA).Scan(&encryptedHexStr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	cipherText, err := hex.DecodeString(encryptedHexStr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	decryptedSecret, err := crypto.Decrypt(cipherText, h.secret.twoFactor)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if !totp.Validate(code, string(decryptedSecret)) {
		h.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Incr(ctx, redisKey)
			pipe.Expire(ctx, redisKey, 15*time.Minute)
			return nil
		})
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	h.redis.Del(ctx, redisKey)
	return nil
}

func (h *AuthHandler) generateBlindIndex(code string) string {
	return crypto.HashString(code + h.secret.mfaPepper)
}

func (h *AuthHandler) validateUserRecovery(ctx context.Context, userID uuid.UUID, code string) (int, *web.WebError) {
	userInput := sanitizeRecoveryCode(code)
	if userInput == "" {
		return 0, web.NewError(http.StatusBadRequest, "invalid_code", nil, nil)
	}

	blindIndex := h.generateBlindIndex(userInput)

	var matchedID uuid.UUID
	var storedHash string
	var remainingCount int

	err := h.db.Pool.QueryRow(ctx, `
    WITH user_codes AS (
        SELECT id, secret, blind_index
        FROM user_credentials
        WHERE user_id = $1 AND kind = $2
    )
    SELECT id, secret, (SELECT COUNT(*) FROM user_codes)
    FROM user_codes
    WHERE blind_index = $3`,
		userID, consts.UserCredentialsRecovery, blindIndex,
	).Scan(&matchedID, &storedHash, &remainingCount)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
		}
		return 0, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	valid, err := crypto.VerifyCredential(userInput, storedHash)
	if err != nil {
		return 0, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	if !valid {
		return 0, web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}

	_, err = h.db.Pool.Exec(ctx, `DELETE FROM user_credentials WHERE id = $1`, matchedID)
	if err != nil {
		return 0, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return max(remainingCount-1, 0), nil
}

func (h *AuthHandler) HandleInit2FA(w http.ResponseWriter, r *http.Request) *web.WebError {
	ctx := r.Context()
	user, ok := GetUser(ctx)
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      consts.BrandName,
		AccountName: user.Username,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	encryptedSecret, err := crypto.Encrypt([]byte(key.Secret()), h.secret.twoFactor)
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
func (h *AuthHandler) generateNewRecoveryCodes(ctx context.Context, lang string) (plainRecoveryCodes []string, indexes []string, hashes []string, err error) {
	words, err := h.i18nManager.PickWords(lang, 30)
	if err != nil {
		return nil, nil, nil, err
	}

	plainRecoveryCodes = make([]string, 0, 10)

	for i := 0; i < 30; i += 3 {
		code := strings.Join(words[i:i+3], "-")
		plainRecoveryCodes = append(plainRecoveryCodes, sanitizeRecoveryCode(code))
	}

	g, groupCtx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))

	hashes = make([]string, 10)
	indexes = make([]string, 10)

	for i, code := range plainRecoveryCodes {
		i, code := i, code
		g.Go(func() error {
			select {
			case <-groupCtx.Done():
				return groupCtx.Err()
			default:
			}
			if groupCtx.Err() != nil {
				return nil
			}
			hash, err := crypto.HashCredential(code)
			if err != nil {
				return err
			}
			hashes[i] = string(hash)
			indexes[i] = h.generateBlindIndex(code)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, nil, web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return plainRecoveryCodes, hashes, indexes, nil
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

	decryptedSecret, err := crypto.Decrypt(cipherText, h.secret.twoFactor)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if !totp.Validate(req.Code, string(decryptedSecret)) {
		return web.NewError(http.StatusUnauthorized, "invalid_code", nil, nil)
	}
	lang := i18n.GetLangFromReq(r)
	plainRecoveryCodes, hashes, indexes, err := h.generateNewRecoveryCodes(ctx, lang)
	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	defer tx.Rollback(ctx)

	b := &pgx.Batch{}

	b.Queue(`
		INSERT INTO user_credentials (user_id, kind, secret) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, kind) WHERE kind IN ('passkey', 'totp')
		DO UPDATE SET secret = EXCLUDED.secret, updated_at = NOW()
	`, user.ID, consts.UserCredentials2FA, encryptedHexStr)

	b.Queue(`
    INSERT INTO user_credentials (user_id, kind, secret, blind_index)
    SELECT $1, $2, unnest($3::text[]), unnest($4::text[])`,
		user.ID, consts.UserCredentialsRecovery, hashes, indexes,
	)

	br := tx.SendBatch(ctx, b)
	if err := br.Close(); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if err := tx.Commit(ctx); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

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

	claims, ok := GetClaims(r.Context())
	if !ok {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	if err := h.validateUserTOTP(r.Context(), claims.User.ID, req.Code); err != nil {
		return err
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
	return nil
}
func (h *AuthHandler) HandleResetVerify2FA(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	cookieT, err := r.Cookie("reset_claims")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	claims, err := h.GetChallengeClaims(cookieT.Value)
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}
	if !claims.MfaPending {
		return nil
	}

	if err := h.validateUserTOTP(r.Context(), claims.UserID, req.Code); err != nil {
		return err
	}

	if err := h.secret.issueChallengeClaims(w, claims.UserID, false, "reset"); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", nil, nil)
	}

	return nil
}
func (h *AuthHandler) HandleVerifyRecoveryCode(w http.ResponseWriter, r *http.Request) *web.WebError {
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

	remainingCodes, webErr := h.validateUserRecovery(ctx, claims.User.ID, req.Code)
	if webErr != nil {
		return webErr
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
	err := sonic.ConfigDefault.NewEncoder(w).Encode(map[string]int{
		"remaining": remainingCodes,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
func (h *AuthHandler) HandleResetVerifyRecoveryCode(w http.ResponseWriter, r *http.Request) *web.WebError {
	var req struct {
		Code string `json:"code"`
	}
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", nil, nil)
	}

	cookieT, err := r.Cookie("reset_claims")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}

	claims, err := h.GetChallengeClaims(cookieT.Value)
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", nil, nil)
	}
	if !claims.MfaPending {
		return nil
	}

	remainingCodes, webErr := h.validateUserRecovery(r.Context(), claims.UserID, req.Code)
	if webErr != nil {
		return webErr
	}

	if err := h.secret.issueChallengeClaims(w, claims.UserID, false, "reset"); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", nil, nil)
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
	plainCodes, hashes, indexes, err := h.generateNewRecoveryCodes(ctx, lang)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM user_credentials WHERE user_id = $1 AND kind = $2`,
		user.ID, consts.UserCredentialsRecovery)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	_, err = tx.Exec(ctx, `
    INSERT INTO user_credentials (user_id, kind, secret, blind_index)
    SELECT $1, $2, unnest($3::text[]), unnest($4::text[])`,
		user.ID, consts.UserCredentialsRecovery, hashes, indexes,
	)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	if err := tx.Commit(ctx); err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return writeRecoveryCodesResponse(w, plainCodes)
}
