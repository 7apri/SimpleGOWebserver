package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
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
	token, err := generateRandomToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	lang := i18n.GetLangFromReq(r)

	ctx := r.Context()
	i18n.GetLangFromContext(ctx)

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
		token,
	)
	if err != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	go h.sendVerificationEmail(req.Email, token, lang)

	w.WriteHeader(http.StatusCreated)
	sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{"message": "Please check your email to verify your account"})
}

const (
	from     = "noreply@panels.com"
	password = ""
	smtpHost = "mailpit"
	smtpPort = "1025"
)

func (h *AuthHandler) sendVerificationEmail(targetEmail, token string, lang string) {
	verifyLink := fmt.Sprintf("https://local.7apri.cfd/api/auth/verify?token=%s", token)

	msg := fmt.Sprintf("Subject: Confirm your Email\r\n"+
		"To: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"Please click the link to verify your account: %s", targetEmail, verifyLink)

	err := smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{targetEmail}, []byte(msg))
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", targetEmail, "host", smtpHost+":"+smtpPort)
	}
}
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	const q = `
	WITH deleted AS (
		DELETE FROM user_verifications 
		WHERE token = $1 AND expires_at > NOW()
		RETURNING user_id
	)
	UPDATE users 
		SET is_verified = true 
		FROM deleted 
		WHERE users.id = deleted.user_id
	RETURNING users.id, users.role;`

	var user UserPrint
	err := h.db.Pool.QueryRow(r.Context(), q, token).Scan(&user.ID, &user.Role)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, &user)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
