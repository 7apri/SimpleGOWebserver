package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthHandler struct {
	db     *pgxpool.Pool
	secret *secretWrap
}

func NewAuthHandler(db *pgxpool.Pool) *AuthHandler {
	return &AuthHandler{
		db: db,
		secret: &secretWrap{
			accessSecret:  []byte{},
			refreshSecret: []byte{},
		},
	}
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

		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
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

	var userID int64
	err := h.db.QueryRow(r.Context(),
		"INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id",
		req.Username, strings.ToLower(req.Email), hashed).Scan(&userID)

	if err != nil {
		http.Error(w, "Username or Email already taken", http.StatusConflict)
		return
	}

	h.issueTokens(w, userID)
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

	var userID int64
	var hash string

	input := strings.ToLower(strings.TrimSpace(req.Identifier))

	const q = `
        SELECT id, password_hash 
        FROM users 
        WHERE email = $1 OR username = $1 
        LIMIT 1
    `

	err := h.db.QueryRow(r.Context(), q, input).Scan(&userID, &hash)

	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if match, _ := VerifyPassword(req.Password, hash); !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, userID)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "No session", http.StatusUnauthorized)
		return
	}

	var userID int64
	err = h.db.QueryRow(context.Background(),
		"DELETE FROM refresh_sessions WHERE token_hash = $1 AND expires_at > NOW() RETURNING user_id",
		cookie.Value).Scan(&userID)

	if err != nil {
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, userID)
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, userID int64) {
	access, exp, _ := h.secret.GenerateAccess(userID)
	refresh := fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())

	h.db.Exec(context.Background(),
		"INSERT INTO refresh_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		userID, refresh, time.Now().Add(30*24*time.Hour))

	http.SetCookie(w, &http.Cookie{
		Name: "access_token", Value: access, Expires: exp, HttpOnly: true, Secure: true, Path: "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name: "refresh_token", Value: refresh, Expires: time.Now().Add(30 * 24 * time.Hour), HttpOnly: true, Secure: true, Path: "/auth/refresh",
	})

	w.Write([]byte(`{"status":"success"}`))
}
