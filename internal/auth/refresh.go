package auth

import (
	"context"
	"net/http"
)

func (h *AuthHandler) Refresh(ctx context.Context, w http.ResponseWriter, r *http.Request, refreshToken string) (*UserClaims, error) {
	remember := false
	const q = `
		DELETE FROM refresh_sessions rs
		USING users u
			WHERE rs.user_id = u.id 
			AND rs.token_hash = $1 
			AND rs.expires_at > NOW()
			AND u.is_verified = true
		RETURNING u.id, u.role, u.username, rs.remember_me`
	var user UserPrint
	err := h.db.Pool.QueryRow(r.Context(), q, HashString(refreshToken)).Scan(
		&user.ID,
		&user.Role,
		&user.Username,
		&remember,
	)
	if err != nil {
		return nil, err
	}
	claims, err := h.issueTokens(w, r, &user, TokenOptions{
		AccessTokenOptions: AccessTokenOptions{
			Remember: remember,
		},
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
