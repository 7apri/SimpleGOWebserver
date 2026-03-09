package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/mileusna/useragent"
	"golang.org/x/crypto/argon2"
)

type accessSecretWrap struct {
	accessSecret []byte
}
type UserClaims struct {
	User *UserPrint
	jwt.RegisteredClaims
}
type UserPrintBig struct {
	UserPrint
	UserDetail
}
type UserDetail struct {
	UserContact
	Lang string `json:"lang"`
}
type UserPrint struct {
	ID   uuid.UUID `json:"id"`
	Role string    `json:"role"`
}
type UserContact struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

const (
	UserCredentialsPassword = "passkey"
)

func GenerateRandomToken() (string, error) {
	b := make([]byte, 48)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func HashString(token string) string {
	hashed := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hashed[:])
}

const (
	argonPasses  = 3
	argonThreads = 2
	argonMemory  = 1024 * 64 // 64 MB
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, argonPasses, argonMemory, argonThreads, 32)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonPasses, argonThreads, b64Salt, b64Hash), nil
}
func VerifyPassword(password, encodedHash string) (bool, error) {
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
		[]byte(password),
		salt,
		uint32(passes),
		uint32(memory),
		uint8(threads),
		uint32(len(decodedHash)),
	)

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
			Issuer:    "panels",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.accessSecret)
	return token, expiry, err
}

func (s *accessSecretWrap) ValidateAccess(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.accessSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*UserClaims), nil
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, r *http.Request, user *UserPrint) error {
	var refresh string

	access, exp, err := h.secret.GenerateAccess(user)
	if err != nil {
		return err
	}

	refresh, err = GenerateRandomToken()
	if err != nil {
		return err
	}

	refreshHash := HashString(refresh)
	ip := util.GetClientIP(r)
	rawUA := r.UserAgent()
	ua := useragent.Parse(rawUA)
	deviceName := fmt.Sprintf("%s on %s", ua.Name, ua.OS)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var usrDetail email.UserDetail
		const q = `
        SELECT email, username, preferred_lang FROM users 
        WHERE id = $1 
		AND NOT EXISTS (
			SELECT 1 FROM refresh_sessions
			WHERE user_id = $1 AND ip_address = $2 AND device_name = $3
		)`

		err := h.db.Pool.QueryRow(ctx, q, user.ID, ip, deviceName).Scan(
			&usrDetail.Email,
			&usrDetail.Username,
			&usrDetail.Lang,
		)

		if err == nil {
			h.EmailManager.SendNewLoginEmail(&email.NewLoginInfo{
				Device:     deviceName,
				IP:         ip,
				Time:       time.Now().String(),
				SecureLink: "not implemented yet",
			}, usrDetail)
		}
	}()

	ctx, cancelCtx := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelCtx()

	_, err = h.db.Pool.Exec(ctx,
		`INSERT INTO refresh_sessions 
        (user_id, token_hash, expires_at, ip_address, user_agent, device_name) 
        VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, refreshHash, time.Now().Add(30*24*time.Hour), ip, rawUA, deviceName)

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
		HttpOnly: true,
		Secure:   true,
		Path:     "/api/auth/",
		SameSite: http.SameSiteLaxMode,
	})

	return nil
}
