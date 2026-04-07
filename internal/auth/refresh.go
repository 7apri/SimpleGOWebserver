package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/7apri/SimpleGOWebserver/internal/templates"
)

func redirectToLogin(w http.ResponseWriter, r *http.Request, next string) {
	target := "/sign-in"
	if next != "" {
		target = target + "?next=" + url.QueryEscape(next)
	}
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	next := r.URL.Query().Get("next")

	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		redirectToLogin(w, r, next)
		return
	}

	remember := false

	const q = `
		DELETE FROM refresh_sessions rs
		USING users u
			WHERE rs.user_id = u.id 
			AND rs.token_hash = $1 
			AND rs.expires_at > NOW()
			AND u.is_verified = true
		RETURNING u.id, u.role, u.username, u.avatar_url, u.updated_at, rs.remember_me`

	var user UserPrintTimestamp
	err = h.db.Pool.QueryRow(r.Context(), q, HashString(cookie.Value)).Scan(
		&user.ID,
		&user.Role,
		&user.Username,
		&user.AvatarURL,
		&user.UpdatedAt,
		&remember,
	)

	if err != nil {
		redirectToLogin(w, r, next)
		return
	}

	err = h.issueTokens(w, r, &user, TokenOptions{
		AccessTokenOptions: AccessTokenOptions{
			Remember: remember,
		},
	})
	if err != nil {
		redirectToLogin(w, r, next)
		return
	}

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	templates.SetETag(w, r, "refresh")

	http.Redirect(w, r, next, http.StatusSeeOther)
}
