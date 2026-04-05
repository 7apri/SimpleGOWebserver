package auth

import (
	"net/http"
)

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		h.db.Pool.Exec(r.Context(), "DELETE FROM refresh_sessions WHERE token_hash = $1", HashString(cookie.Value))
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
		Path:     "/api/auth/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	resetCSRFCookie(w)

	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}
