package auth

import (
	"net/http"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/bytedance/sonic"
)

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

	if strings.Contains(req.Username, "@") || !strings.Contains(req.Email, "@") {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	hashed, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	token, err := GenerateRandomToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	lang := i18n.GetLangFromReq(r)

	const registerQuery = `
	WITH new_user AS (
		INSERT INTO users (username, email, password_hash, preferred_lang)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	)
	INSERT INTO user_verifications (user_id, token, expires_at)
	SELECT id, $5, NOW() + INTERVAL '24 hours' FROM new_user
	ON CONFLICT (user_id) DO UPDATE SET token = EXCLUDED.token, expires_at = EXCLUDED.expires_at
	`
	_, err = h.db.Pool.Exec(ctx, registerQuery,
		req.Username,
		strings.ToLower(req.Email),
		hashed,
		lang,
		HashToken(token),
	)
	if err != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	go h.sendVerificationEmail(req.Email, token, lang)

	w.WriteHeader(http.StatusCreated)
	sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{"message": "Please check your email to verify your account"})
}
