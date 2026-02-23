package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/jackc/pgx/v5/pgxpool"
)

const callbackPath = "/api/auth/e/callback"

type OAuthProvider interface {
	Name() string
	getAuthURL(state, challenge string) string
	exchangeCode(ctx context.Context, code, verifier string) (string, error)
	fetchUser(ctx context.Context, token string) (*ExternalUser, error)
	getUserPrint(ctx context.Context, extUsr *ExternalUser, lang string, dbPool *pgxpool.Pool) (*UserPrint, error)
}
type ExternalUser struct {
	ID       string
	Username string
	Email    string
}

var ErrExtUserNoEmail = errors.New("external user has no email")

func GeneratePKCE() (verifier string, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}

func (h *AuthHandler) OAuthLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	p, ok := h.providers[q.Get("provider")]
	if !ok {
		http.Error(w, "Provider not supported", http.StatusBadRequest)
		return
	}

	state, err := GenerateRandomToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	next := q.Get("next")
	if next == "" {
		next = "/"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_next",
		Value:    next,
		Path:     callbackPath,
		MaxAge:   300,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     callbackPath,
		HttpOnly: true,
		MaxAge:   300,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_verifier",
		Value:    verifier,
		Path:     callbackPath,
		HttpOnly: true,
		MaxAge:   300,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, p.getAuthURL(state, challenge), http.StatusTemporaryRedirect)
}
func clearCookies(w http.ResponseWriter, path string, names ...string) {
	for _, name := range names {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     path,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})
	}
}
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := h.providers[r.URL.Query().Get("provider")]
	if !ok {
		http.Error(w, "Provider not supported", http.StatusBadRequest)
		return
	}

	verifierCookie, err := r.Cookie("oauth_verifier")
	if err != nil {
		http.Error(w, "No verifier cookie", http.StatusBadRequest)
		return
	}
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	code := r.URL.Query().Get("code")
	token, err := p.exchangeCode(ctx, code, verifierCookie.Value)
	if err != nil {
		http.Error(w, "Token exchange failed", http.StatusInternalServerError)
		return
	}

	extUser, err := p.fetchUser(ctx, token)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	user, err := p.getUserPrint(ctx, extUser, i18n.GetLangFromReq(r), h.db.Pool)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	h.issueTokens(w, r, user)

	next := "/"
	if cookie, err := r.Cookie("oauth_next"); err == nil {
		next = cookie.Value
	}

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	clearCookies(w, callbackPath, "oauth_state", "oauth_verifier", "oauth_next")
	http.Redirect(w, r, next, http.StatusSeeOther)
}
