package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type contextKey string

const ClaimsContextKey contextKey = "auth_claims"

func SetAuthContext(ctx context.Context, claims *UserClaims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}
func GetClaims(ctx context.Context) (*UserClaims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*UserClaims)
	return claims, ok
}
func GetUser(ctx context.Context) (*UserPrint, bool) {
	claims, ok := GetClaims(ctx)
	if !ok {
		return nil, false
	}
	return claims.User, true
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
			http.Redirect(w, r, fmt.Sprintf("/api/auth/refresh?next=%s", url.QueryEscape(r.URL.RequestURI())), http.StatusTemporaryRedirect)
			return
		}

		if claims.Pending2FA {
			http.Redirect(w, r, fmt.Sprintf("/2fa?next=%s", url.QueryEscape(r.URL.RequestURI())), http.StatusTemporaryRedirect)
			return
		}

		ctx := SetAuthContext(r.Context(), claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (h *AuthHandler) MiddlewareTwoFA(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("access_token")
		if err != nil {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}

		claims, err := h.secret.ValidateAccess(cookie.Value)
		if err != nil {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}
		if !claims.Pending2FA {
			http.Error(w, "Not in 2FA state", http.StatusBadRequest)
			return
		}

		ctx := SetAuthContext(r.Context(), claims)

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
		if err != nil || claims.Pending2FA {
			next.ServeHTTP(w, r)
			return
		}

		ctx := SetAuthContext(r.Context(), claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/*
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
*/
