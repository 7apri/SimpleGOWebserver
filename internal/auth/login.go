package auth

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
)

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
	var hash sql.NullString

	input := strings.TrimSpace(req.Identifier)

	const q = `
        SELECT id, role,password_hash 
        FROM users 
        WHERE email = $1 OR username = $1 
        LIMIT 1
    `

	err := h.db.Pool.QueryRow(r.Context(), q, input).Scan(&user.ID, &user.Role, &hash)

	if err != nil || !hash.Valid {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if match, _ := VerifyPassword(req.Password, hash.String); !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, r, &user)
}
