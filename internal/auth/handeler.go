package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/bytedance/sonic"
)

type AuthHandler struct {
	db     *database.Database
	secret *secretWrap
}

func NewAuthHandler(db *database.Database, accessSecret string) *AuthHandler {

	return &AuthHandler{
		db: db,
		secret: &secretWrap{
			accessSecret: []byte(accessSecret),
		},
	}
}

type contextKey string

const (
	userKey contextKey = "user"
)

func SetUserContext(ctx context.Context, user *UserPrint) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func GetUserFromContext(ctx context.Context) (*UserPrint, bool) {
	uid, ok := ctx.Value(userKey).(*UserPrint)
	return uid, ok
}

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}

		ctx := SetUserContext(r.Context(), claims.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *AuthHandler) MiddlewareSoft(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := SetUserContext(r.Context(), claims.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *AuthHandler) MiddlewareGuestOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err == nil {
			if _, err := h.secret.ValidateAccess(cookie.Value); err == nil {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (h *AuthHandler) MiddlewareRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		ctx := SetUserContext(r.Context(), claims.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Username) < 3 || !strings.Contains(req.Email, "@") {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	hashed, _ := HashPassword(req.Password)

	var user UserPrint
	err := h.db.Pool.QueryRow(r.Context(),
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id,role",
		req.Username, strings.ToLower(req.Email), hashed).Scan(
		&user.ID,
		&user.Role,
	)

	if err != nil {
		http.Error(w, "Username or Email already taken", http.StatusConflict)
		return
	}

	h.issueTokens(w, &user)

	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/"
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var user UserPrint
	var hash string

	input := strings.ToLower(strings.TrimSpace(req.Identifier))

	const q = `
        SELECT id, role,password_hash 
        FROM users 
        WHERE email = $1 OR username = $1 
        LIMIT 1
    `

	err := h.db.Pool.QueryRow(r.Context(), q, input).Scan(&user.ID, &user.Role, &hash)

	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if match, _ := VerifyPassword(req.Password, hash); !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, &user)

	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/"
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		h.db.Pool.Exec(r.Context(), "DELETE FROM refresh_sessions WHERE token_hash = $1", cookie.Value)
	}

	expiredCookie := &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}
	http.SetCookie(w, expiredCookie)

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth/refresh",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	const q = `
	DELETE FROM refresh_sessions rs
			USING users u
			WHERE rs.user_id = u.id 
			AND rs.token_hash = $1 
			AND rs.expires_at > NOW()
			RETURNING u.id, u.role`

	var user UserPrint
	err = h.db.Pool.QueryRow(context.Background(),
		q,
		cookie.Value).Scan(
		&user.ID,
		&user.Role,
	)

	if err != nil {
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, &user)
}

func generateRandomRefreshToken() (string, error) {
	b := make([]byte, 48)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, user *UserPrint) {
	var refresh string

	access, exp, err := h.secret.GenerateAccess(user)
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	refresh, err = generateRandomRefreshToken()
	if err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}

	h.db.Pool.Exec(context.Background(),
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
