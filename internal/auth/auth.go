package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

type accessSecretWrap struct {
	accessSecret []byte
}
type UserClaims struct {
	User *UserPrint
	jwt.RegisteredClaims
}
type UserPrint struct {
	ID   uuid.UUID `json:"id"`
	Role string    `json:"role"`
}

func generateRandomToken() (string, error) {
	b := make([]byte, 48)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

const (
	argonPasses  = 1
	argonThreads = 2
	argonMemory  = 32 * 1024
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argonPasses, argonMemory, argonThreads, 32)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=32768,t=2,p=1$%s$%s", b64Salt, b64Hash), nil
}
func VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid hash format")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	comparisonHash := argon2.IDKey([]byte(password), salt, argonPasses, argonMemory, argonThreads, uint32(len(decodedHash)))

	return subtle.ConstantTimeCompare(decodedHash, comparisonHash) == 1, nil
}

func (s *accessSecretWrap) GenerateAccess(user *UserPrint) (string, time.Time, error) {
	expiry := time.Now().Add(15 * time.Minute)
	claims := UserClaims{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "weather-app",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.accessSecret)
	return token, expiry, err
}

func (s *accessSecretWrap) ValidateAccess(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
		return s.accessSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*UserClaims), nil
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, user *UserPrint) {
	var refresh string

	access, exp, err := h.secret.GenerateAccess(user)
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	refresh, err = generateRandomToken()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	ctx, cancelCtx := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelCtx()

	h.db.Pool.Exec(ctx,
		"INSERT INTO refresh_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		user.ID, refresh, time.Now().Add(30*24*time.Hour))

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Expires:  exp,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
		HttpOnly: true, Secure: true,
		Path:     "/api/auth/refresh",
		SameSite: http.SameSiteLaxMode,
	})
}
