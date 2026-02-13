package auth

import "net/http"

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
	err = h.db.Pool.QueryRow(r.Context(), q, cookie.Value).Scan(
		&user.ID,
		&user.Role,
	)

	if err != nil {
		http.Error(w, "Session expired", http.StatusUnauthorized)
		return
	}

	h.issueTokens(w, &user)
}
