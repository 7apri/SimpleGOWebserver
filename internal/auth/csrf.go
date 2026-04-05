package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

const (
	saltLenghtCrsf   = 16
	secretLenghtCrsf = saltLenghtCrsf
	cookieNameCrsf   = "csrf_secret"
)

func MaskSecret(secretHex string) string {
	secret, err := hex.DecodeString(secretHex)
	if err != nil || len(secret) != secretLenghtCrsf {
		return ""
	}

	salt := make([]byte, saltLenghtCrsf)
	if _, err := rand.Read(salt); err != nil {
		return ""
	}

	masked := make([]byte, saltLenghtCrsf+secretLenghtCrsf)
	copy(masked[:saltLenghtCrsf], salt)

	for i := range secretLenghtCrsf {
		masked[saltLenghtCrsf+i] = secret[i] ^ salt[i]
	}

	return base64.RawURLEncoding.EncodeToString(masked)
}

func VerifyCSRFToken(maskedTokenBase64, secretHex string) bool {
	secret, err1 := hex.DecodeString(secretHex)
	decoded, err2 := base64.RawURLEncoding.DecodeString(maskedTokenBase64)

	if err1 != nil || err2 != nil ||
		len(secret) != secretLenghtCrsf ||
		len(decoded) != (saltLenghtCrsf+secretLenghtCrsf) {
		return false
	}

	salt := decoded[:saltLenghtCrsf]
	xored := decoded[saltLenghtCrsf:]

	unmasked := make([]byte, secretLenghtCrsf)
	for i := range secretLenghtCrsf {
		unmasked[i] = xored[i] ^ salt[i]
	}

	return subtle.ConstantTimeCompare(unmasked, secret) == 1
}

const csrfKey contextKey = "csrf"

func GetCSRFSecretFromContext(ctx context.Context) (string, bool) {
	scrt, ok := ctx.Value(csrfKey).(string)
	return scrt, ok
}

func CSRFMiddleware(i18nMgr *i18n.I18nManager) web.Middleware {
	return func(next http.Handler) http.Handler {
		return web.MakeHandler(func(w http.ResponseWriter, r *http.Request) *web.WebError {
			cookie, err := r.Cookie(cookieNameCrsf)
			if err != nil || cookie.Value == "" {
				_, err := setCSRFCookie(w)
				if err != nil {
					return web.NewError(http.StatusInternalServerError, "internal", nil, nil)
				}
				return web.NewError(http.StatusForbidden, "forbidden", nil, nil)
			}

			isMutating := false
			switch r.Method {
			case "POST", "PUT", "DELETE", "PATCH":
				isMutating = true
			}

			if isMutating {
				clientToken := r.Header.Get("X-CSRF-Token")
				if clientToken == "" {
					clientToken = r.FormValue("csrf_token")
				}

				if clientToken == "" || !VerifyCSRFToken(clientToken, cookie.Value) {
					return web.NewError(http.StatusForbidden, "forbidden", nil, nil)
				}
			}

			ctx := context.WithValue(r.Context(), csrfKey, cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))

			return nil
		}, i18nMgr, nil)
	}
}

func CSRFEndpoint(w http.ResponseWriter, r *http.Request) *web.WebError {
	var clientSecret string
	cookie, err := r.Cookie(cookieNameCrsf)
	if err != nil {
		clientSecret, err = setCSRFCookie(w)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", nil, nil)
		}
	} else {
		clientSecret = cookie.Value
	}

	maskedToken := MaskSecret(clientSecret)
	if maskedToken == "" {
		return web.NewError(http.StatusForbidden, "invalid_token", nil, nil)
	}

	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write([]byte(`{"token":"` + maskedToken + `"}`))
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}

func setCSRFCookie(w http.ResponseWriter) (string, error) {
	secretStr, err := GenerateRandomString(secretLenghtCrsf)
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieNameCrsf,
		Value:    secretStr,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	return secretStr, nil
}

func resetCSRFCookie(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieNameCrsf,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	return nil
}
