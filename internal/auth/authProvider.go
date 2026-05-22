package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/bytedance/sonic"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

const callbackPath = "/api/auth/e/callback"

type OAuthProvider interface {
	Name() string
	getAuthURL(state, challenge string) string
	exchangeCode(ctx context.Context, code, verifier string) (string, error)
	fetchUser(ctx context.Context, token string) (*ExternalUser, error)
}
type ExternalUser struct {
	ID        string
	Username  string
	Email     string
	AvatarURL string
}

type PendingAuthProviderClaims struct {
	Email      string `json:"email"`
	Username   string `json:"username,omitempty"`
	ExternalID string `json:"ext_id"`
	Provider   string `json:"provider"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	Action     string `json:"action"`
	UserID     string `json:"user_id,omitempty"`
	jwt.RegisteredClaims
}

var ErrExtUserNoEmail = errors.New("external user has no email")
var SocialAccountTaken = errors.New("social account taken")

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

func (h *AuthHandler) OAuthLogin(w http.ResponseWriter, r *http.Request) *web.WebError {
	q := r.URL.Query()
	pQ := q.Get("provider")

	p, ok := h.providers[pQ]
	if !ok {
		return web.NewError(http.StatusBadRequest, "unsupported_provider", nil, map[string]string{"provider": pQ})
	}

	state, err := GenerateRandomString(32)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	next := q.Get("next")
	if next != "" && next != "/" {
		if err == nil {
			http.SetCookie(w, &http.Cookie{
				Name:     "oauth_next",
				Value:    next,
				Path:     "/",
				MaxAge:   300,
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
			})
		}
	}

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
	return nil
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
func (h *AuthHandler) handleOAuthLogin(w http.ResponseWriter, r *http.Request, user *UserPrint, has2FA bool) *web.WebError {
	var next string
	if cookie, err := r.Cookie("oauth_next"); err == nil {
		next = cookie.Value
		clearCookies(w, "/", "oauth_next")
	}

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	if has2FA {
		if next != "/" {
			next = "/2fa?next=" + url.QueryEscape(next)
		} else {
			next = "/2fa"
		}

		if err := h.issueAccessToken(w, user, AccessTokenOptions{
			IsPending: true,
			Remember:  true,
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
	} else {
		if err := h.issueTokens(w, r, user, TokenOptions{
			RotateCSRF: true,
			SendEmail:  true,
			AccessTokenOptions: AccessTokenOptions{
				Remember: true,
			},
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) *web.WebError {
	q := r.URL.Query()
	pQ := q.Get("provider")

	p, ok := h.providers[pQ]
	if !ok {
		return web.NewError(http.StatusBadRequest, "unsupported_provider", nil, map[string]string{"provider": pQ})
	}

	verifierCookie, err := r.Cookie("oauth_verifier")
	if err != nil {
		return web.NewError(http.StatusBadRequest, "no_cookie", err, nil)

	}
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || q.Get("state") != stateCookie.Value {
		return web.NewError(http.StatusBadRequest, "invalid_state", err, nil)
	}

	ctx := r.Context()
	code := q.Get("code")
	token, err := p.exchangeCode(ctx, code, verifierCookie.Value)
	if err != nil {
		return web.NewError(http.StatusBadGateway, "oauth_failed", err, nil)
	}

	extUser, err := p.fetchUser(ctx, token)
	if err != nil {
		return web.NewError(http.StatusBadGateway, "oauth_failed", err, nil)
	}

	clearCookies(w, callbackPath, "oauth_state", "oauth_verifier")

	var (
		user            UserPrint
		has2FA, isLogin bool
	)

	const findQ = `
    WITH oauth_match AS (
        SELECT u.id, u.role, u.username,
               EXISTS(SELECT 1 FROM user_credentials WHERE user_id = u.id AND kind = $3) as has_2fa,
               true as is_login
        FROM users u
        JOIN user_credentials uc ON uc.user_id = u.id
        WHERE uc.kind = $1 AND uc.secret = $2
		LIMIT 1
    ),
    email_match AS (
        SELECT id, role, username,
				false as has_2fa,
                false as is_login
        FROM users
        WHERE email = $4 AND NOT EXISTS (SELECT 1 FROM oauth_match)
		LIMIT 1
    )
    SELECT id, role, username, has_2fa, is_login 
    FROM oauth_match
    UNION ALL
    SELECT id, role, username, has_2fa, is_login
    FROM email_match;`

	err = h.db.Pool.QueryRow(ctx, findQ,
		p.Name(), extUser.ID, UserCredentials2FA, extUser.Email,
	).Scan(&user.ID, &user.Role, &user.Username, &has2FA, &isLogin)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return web.NewError(http.StatusInternalServerError, "database_error", err, nil)
	}

	if err == nil && isLogin {
		return h.handleOAuthLogin(w, r, &user, has2FA)
	}

	action := "link"
	userIDStr := user.ID.String()

	if errors.Is(err, pgx.ErrNoRows) {
		action = "register"
		userIDStr = ""
	}

	expiry := time.Now().Add(15 * time.Minute)

	claims := PendingAuthProviderClaims{
		Email:      extUser.Email,
		Username:   user.Username,
		ExternalID: extUser.ID,
		Provider:   p.Name(),
		AvatarURL:  extUser.AvatarURL,
		Action:     action,
		UserID:     userIDStr,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
		},
	}

	pending, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.secret.provider)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "jwt_error", err, nil)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_pending",
		Value:    pending,
		Expires:  expiry,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	if cookie, err := r.Cookie("oauth_next"); err == nil {
		clearCookies(w, "/", "oauth_next")
		http.Redirect(w, r, "/sign-up?next="+url.QueryEscape(cookie.Value), http.StatusSeeOther)
		return nil
	}

	http.Redirect(w, r, "/sign-up", http.StatusSeeOther)
	return nil
}

func (h *AuthHandler) GetPendingAuthProviderClaims(tokenString string) (*PendingAuthProviderClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &PendingAuthProviderClaims{}, func(t *jwt.Token) (any, error) {
		return h.secret.provider, nil
	})

	if err == nil && token.Valid {
		return token.Claims.(*PendingAuthProviderClaims), nil
	}

	return nil, err
}
func (h *AuthHandler) CancelPendingAuth(w http.ResponseWriter, r *http.Request) *web.WebError {
	next := "/"
	if cookie, err := r.Cookie("oauth_next"); err == nil {
		next = cookie.Value
	}

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	clearCookies(w, "/", "oauth_pending", "oauth_next")
	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

func (h *AuthHandler) FinalizeExternal(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookie, err := r.Cookie("oauth_pending")
	if err != nil {
		return web.NewError(http.StatusUnauthorized, "session_expired", err, nil)
	}

	claims, err := h.GetPendingAuthProviderClaims(cookie.Value)
	if err != nil || claims == nil {
		return web.NewError(http.StatusUnauthorized, "invalid_token", err, nil)
	}

	ctx := r.Context()

	var req struct {
		Username    *string `json:"username"`
		DisplayName *string `json:"display_name"`
		RememberMe  bool    `json:"remember"`
	}

	if err := sonic.ConfigDefault.NewDecoder(r.Body).Decode(&req); err != nil {
		return web.NewError(http.StatusBadRequest, "invalid_json", err, nil)
	}

	var user UserPrint
	var has2FA bool

	if claims.Action == "link" {
		const qCheck2FA = `SELECT EXISTS(SELECT 1 FROM user_credentials WHERE user_id = $1 AND kind = $2)`
		err = h.db.Pool.QueryRow(ctx, qCheck2FA, claims.UserID, UserCredentials2FA).Scan(&has2FA)
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "database_error", err, nil)
		}

		const qLink = `
            WITH linked AS (
                INSERT INTO user_credentials (user_id, kind, secret)
                VALUES ($1, $2, $3)
                RETURNING user_id
            )
            SELECT u.id, u.role, u.username
            FROM users u
            JOIN linked l ON u.id = l.user_id;`

		err = h.db.Pool.QueryRow(ctx, qLink, claims.UserID, claims.Provider, claims.ExternalID).Scan(
			&user.ID, &user.Role, &user.Username,
		)
	} else {
		if req.Username == nil || *req.Username == "" {
			return web.NewError(http.StatusBadRequest, "username_required", nil, nil)
		}
		/*
			if req.DisplayName == nil || *req.DisplayName == "" {
				return web.NewError(http.StatusBadRequest, "display_name_required", nil, nil)
			}
		*/

		lang, ok := i18n.GetLangFromContext(ctx)
		if !ok {
			lang = "en"
		}
		settings, err := sonic.Marshal(map[string]string{
			"lang": lang,
		})
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "marshal", err, nil)
		}

		const qReg = `
            WITH new_user AS (
                INSERT INTO users (email, username, display_name, avatar_url, settings, is_verified)
                VALUES ($1, $2, $3, $4, $5, true)
                RETURNING id, role, username
            ),
            new_cred AS (
                INSERT INTO user_credentials (user_id, kind, secret)
                SELECT id, $6, $7 FROM new_user
            )
            SELECT id, role, username FROM new_user;`

		err = h.db.Pool.QueryRow(ctx, qReg,
			claims.Email, req.Username, req.Username, claims.AvatarURL, settings, claims.Provider, claims.ExternalID).Scan(
			&user.ID, &user.Role, &user.Username,
		)
	}

	if err != nil {
		if strings.Contains(err.Error(), "users_username_key") {
			return web.NewError(http.StatusConflict, "username_taken", nil, nil)
		}
		return web.NewError(http.StatusInternalServerError, "database_error", err, nil)
	}

	clearCookies(w, "/", "oauth_pending")

	var status string
	if has2FA {
		err = h.issueAccessToken(w, &user, AccessTokenOptions{
			Remember:  req.RememberMe,
			IsPending: true,
		})
		if err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "pending"
	} else {
		if err := h.issueTokens(w, r, &user, TokenOptions{
			RotateCSRF: true,
			SendEmail:  true,
			AccessTokenOptions: AccessTokenOptions{
				Remember: req.RememberMe,
			},
		}); err != nil {
			return web.NewError(http.StatusInternalServerError, "internal", err, nil)
		}
		status = "success"
	}

	w.Header().Set("Content-Type", "application/json")
	err = sonic.ConfigDefault.NewEncoder(w).Encode(map[string]string{
		"status": status,
	})
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}

	return nil
}
