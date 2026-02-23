package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
)

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
	err := h.db.Pool.QueryRow(r.Context(), q, HashToken(token)).Scan(&user.ID, &user.Role)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, r, &user)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type newLoginInfo struct {
	Device     string
	IP         string
	Time       string
	SecureLink string
}

func sendNewLoginEmail(targetEmail, ip string, deviceName string) {
	msg := fmt.Sprintf("Subject: New login\r\n"+
		"To: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"A new login on %s %s", targetEmail, ip, deviceName)

	err := smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{targetEmail}, []byte(msg))
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", targetEmail, "host", smtpHost+":"+smtpPort)
	}
}

type userInfo struct {
	username string
	email    string
	lang     string
}

func sendWelcomeEmail(user *userInfo) {
	msg := fmt.Sprintf("Subject: Confirm your Email\r\n"+
		"To: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"Welsome %s", user.email, user.username)

	err := smtp.SendMail(smtpHost+":"+smtpPort, nil, from, []string{user.email}, []byte(msg))
	if err != nil {
		slog.Error("SMTP Error", "err", err, "to", user.email, "host", smtpHost+":"+smtpPort)
	}
}
