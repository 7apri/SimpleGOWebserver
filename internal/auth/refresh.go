package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func redirectToLogin(w http.ResponseWriter, r *http.Request, next string) {
	target := "/sign-in"
	if next != "" {
		target = fmt.Sprintf("/sign-in?next=%s", url.QueryEscape(next))
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

	const q = `
        DELETE FROM refresh_sessions rs
        USING users u
        WHERE rs.user_id = u.id 
          AND rs.token_hash = $1 
          AND rs.expires_at > NOW()
        RETURNING u.id, u.role`

	var user UserPrint
	err = h.db.Pool.QueryRow(r.Context(), q, HashToken(cookie.Value)).Scan(
		&user.ID,
		&user.Role,
	)

	if err != nil {
		redirectToLogin(w, r, next)
		return
	}

	h.issueTokens(w, r, &user)

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
}
