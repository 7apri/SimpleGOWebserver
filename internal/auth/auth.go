package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mileusna/useragent"
	"golang.org/x/crypto/argon2"
)

type secretWrap struct {
	access    []byte
	provider  []byte
	twoFactor []byte
}
type UserClaims struct {
	User       *UserPrint `json:"usr"`
	Pending2FA bool       `json:"p2fa,omitempty"`
	RememberMe bool       `json:"rem,omitempty"`
	jwt.RegisteredClaims
}

type UserPrint struct {
	ID       uuid.UUID `json:"id"`
	Role     string    `json:"role"`
	Username string    `json:"username"`
}

const (
	UserCredentialsPassword = "passkey"
)

func GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
func HashString(token string) string {
	hashed := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hashed[:])
}
func Encrypt(plainText []byte, masterKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plainText, nil), nil
}
func Decrypt(cipherText []byte, masterKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(cipherText) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, actualCipherText := cipherText[:nonceSize], cipherText[nonceSize:]
	return gcm.Open(nil, nonce, actualCipherText, nil)
}

const (
	argonPasses  = 3
	argonThreads = 2
	argonMemory  = 1024 * 64 // 64 MB
)

func HashCredential(credential string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(credential), salt, argonPasses, argonMemory, argonThreads, 32)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonPasses, argonThreads, b64Salt, b64Hash), nil
}
func VerifyCredential(credential, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid hash format")
	}

	var memory, passes, threads int

	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &passes, &threads)
	if err != nil {
		return false, errors.New("invalid argon2 parameters in hash")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	comparisonHash := argon2.IDKey(
		[]byte(credential),
		salt,
		uint32(passes),
		uint32(memory),
		uint8(threads),
		uint32(len(decodedHash)),
	)

	return subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1, nil
}

func (s *secretWrap) GenerateAccess(user *UserPrint, opt AccessTokenOptions) (string, time.Time, *UserClaims, error) {
	duration := 15 * time.Minute
	if opt.IsPending {
		duration = 5 * time.Minute
	}
	expiry := time.Now().Add(duration)
	claims := UserClaims{
		User:       user,
		Pending2FA: opt.IsPending,
		RememberMe: opt.Remember,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    consts.Brand,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.access)
	return token, expiry, &claims, err
}

var (
	ErrTokenExpired  = errors.New("token expired")
	ErrInvalidToken  = errors.New("token invalid")
	ErrInvalidClaims = errors.New("claims invalid")
	ErrInvalidIssuer = errors.New("issuer invalid")
)

func (s *secretWrap) ValidateAccess(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.access, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if claims.Issuer != consts.Brand {
		return nil, ErrInvalidIssuer
	}

	return claims, nil
}

type TokenOptions struct {
	RotateCSRF bool
	SendEmail  bool
	AccessTokenOptions
}

type AccessTokenOptions struct {
	Remember  bool
	IsPending bool
}

func (h *AuthHandler) issueAccessToken(w http.ResponseWriter, user *UserPrint, opt AccessTokenOptions) (*UserClaims, error) {
	access, _, claims, err := h.secret.GenerateAccess(user, opt)
	if err != nil {
		return nil, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		MaxAge:   0,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	return claims, nil
}

func restoreNilUUID(id uuid.UUID) interface{} {
	if id == uuid.Nil {
		return nil
	}
	return id
}

func (h *AuthHandler) deviceNeedsPublicKey(ctx context.Context, deviceID uuid.UUID) bool {
	var pk string
	err := h.db.Pool.QueryRow(ctx, `SELECT public_key FROM user_devices WHERE device_id = $1`, deviceID).Scan(&pk)
	return err != nil || pk == "PENDING"
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, r *http.Request, user *UserPrint, options TokenOptions) (*UserClaims, error) {
	if options.RotateCSRF {
		setCSRFCookie(w)
	}
	claims, err := h.issueAccessToken(w, user, options.AccessTokenOptions)
	if err != nil {
		return nil, err
	}

	var (
		deviceID      uuid.UUID
		mustSetCookie bool
	)
	if cookie, err := r.Cookie("device_id"); err == nil {
		deviceID, _ = uuid.Parse(cookie.Value)
	}

	if deviceID == uuid.Nil {
		var err error
		deviceID, err = uuid.NewRandom()
		if err != nil {
			return nil, fmt.Errorf("failed to generate device id: %w", err)
		}
		mustSetCookie = true
	}

	refresh, err := GenerateRandomString(32)
	if err != nil {
		return nil, err
	}
	refreshHash := HashString(refresh)
	ip := web.GetClientIP(r)
	ua := useragent.Parse(r.UserAgent())
	deviceName := ua.Name + " on " + ua.OS

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := h.db.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		isNewDevice bool
		publicKey   string
	)

	const upsertDevice = `
		INSERT INTO user_devices (device_id, user_id, public_key, device_name)
		VALUES ($1, $2, 'PENDING', $3)
		ON CONFLICT (device_id) DO UPDATE SET last_seen = NOW()
		RETURNING public_key, (user_devices.created_at = user_devices.last_seen)`

	err = tx.QueryRow(ctx, upsertDevice, deviceID, user.ID, deviceName).Scan(&publicKey, &isNewDevice)
	if err != nil {
		return nil, fmt.Errorf("device upsert: %w", err)
	}

	const insertSession = `
        INSERT INTO refresh_sessions 
        (user_id, token_hash, expires_at, ip_address, remember_me, device_id) 
        VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = tx.Exec(ctx, insertSession,
		user.ID, refreshHash, time.Now().Add(30*24*time.Hour), ip, options.Remember, restoreNilUUID(deviceID))
	if err != nil {
		return nil, fmt.Errorf("session insert: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit failed: %w", err)
	}

	if mustSetCookie {
		http.SetCookie(w, &http.Cookie{
			Name:     "device_id",
			Value:    deviceID.String(),
			MaxAge:   31536000,
			Path:     "/",
			HttpOnly: false,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	if isNewDevice && options.SendEmail {
		// add email thingy here
	}

	const refreshDuration = 30 * 24 * time.Hour
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		MaxAge:   int(refreshDuration.Seconds()),
		Expires:  time.Now().Add(refreshDuration),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	return claims, nil
}
