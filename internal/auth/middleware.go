package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type contextKey string

const userKey contextKey = "user"

func SetUserContext(ctx context.Context, user *UserPrint) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func GetUserFromContext(ctx context.Context) (*UserPrint, bool) {
	uid, ok := ctx.Value(userKey).(*UserPrint)
	return uid, ok
}

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/api/auth/refresh?next=%s", url.QueryEscape(r.URL.RequestURI())), http.StatusTemporaryRedirect)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}

		ctx := SetUserContext(r.Context(), claims.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *AuthHandler) MiddlewareSoft(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := SetUserContext(r.Context(), claims.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *AuthHandler) MiddlewareGuestOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err == nil {
			if _, err := h.secret.ValidateAccess(cookie.Value); err == nil {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
