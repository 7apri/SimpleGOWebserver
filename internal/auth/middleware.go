package auth

import (
	"context"
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

func (h *AuthHandler) tryRefresh(ctx context.Context, w http.ResponseWriter, r *http.Request) (*UserClaims, bool) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return nil, false
	}

	claims, err := h.Refresh(ctx, w, r, cookie.Value)
	if err != nil {
		return nil, false
	}

	return claims, true
}

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx    = r.Context()
			claims *UserClaims
			err    error
		)

		cookie, err := r.Cookie("access_token")
		if err == nil {
			claims, err = h.secret.ValidateAccess(cookie.Value)
		}
		if err != nil || claims == nil {
			var ok bool
			claims, ok = h.tryRefresh(ctx, w, r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
		}

		if claims == nil || claims.Pending2FA {
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(SetAuthContext(ctx, claims)))
	})
}

func redirectToLogin(w http.ResponseWriter, r *http.Request, next string) {
	target := "/sign-in"
	if next != "" {
		target = target + "?next=" + url.QueryEscape(next)
	}
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) MiddlewareBlock(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, loggedIn := GetClaims(r.Context())
		if !loggedIn {
			redirectToLogin(w, r, url.QueryEscape(r.URL.RequestURI()))
			return
		}

		next.ServeHTTP(w, r)
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
