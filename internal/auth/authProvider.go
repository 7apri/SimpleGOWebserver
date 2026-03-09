package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const callbackPath = "/api/auth/e/callback"

type OAuthProvider interface {
	Name() string
	getAuthURL(state, challenge string) string
	exchangeCode(ctx context.Context, code, verifier string) (string, error)
	fetchUser(ctx context.Context, token string) (*ExternalUser, error)
}
type ExternalUser struct {
	ID       string
	Username string
	Email    string
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
		return web.NewError(http.StatusBadRequest, "err:unsupported_provider", nil, map[string]string{"provider": pQ})
	}

	state, err := GenerateRandomToken()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
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
func (h *AuthHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) *web.WebError {
	q := r.URL.Query()
	pQ := q.Get("provider")

	p, ok := h.providers[pQ]
	if !ok {
		return web.NewError(http.StatusBadRequest, "err:unsupported_provider", nil, map[string]string{"provider": pQ})
	}

	verifierCookie, err := r.Cookie("oauth_verifier")
	if err != nil {
		return web.NewError(http.StatusBadRequest, "err:no_cookie", err, nil)

	}
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || q.Get("state") != stateCookie.Value {
		return web.NewError(http.StatusBadRequest, "err:invalid_state", err, nil)
	}

	ctx := r.Context()
	code := q.Get("code")
	token, err := p.exchangeCode(ctx, code, verifierCookie.Value)
	if err != nil {
		return web.NewError(http.StatusBadGateway, "err:oauth_failed", err, nil)
	}

	extUser, err := p.fetchUser(ctx, token)
	if err != nil {
		return web.NewError(http.StatusBadGateway, "err:oauth_failed", err, nil)
	}

	user, wErr := h.getUserPrint(ctx, extUser, i18n.GetLangFromReq(r), h.db.Pool, p)
	if wErr != nil {
		return wErr
	}

	clearCookies(w, callbackPath, "oauth_state", "oauth_verifier", "oauth_next")

	if err := h.issueTokens(w, r, user); err != nil {
		return web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	next := "/"
	if cookie, err := r.Cookie("oauth_next"); err == nil {
		next = cookie.Value
	}

	if next == "" || strings.HasPrefix(next, "http") || strings.HasPrefix(next, "//") {
		next = "/"
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

func (h *AuthHandler) getUserPrint(ctx context.Context, extUser *ExternalUser, lang string, dbPool *pgxpool.Pool, prv OAuthProvider) (*UserPrint, *web.WebError) {
	var user UserPrint

	const qFind = `
		SELECT 
			u.id, 
			u.role
		FROM users u
		JOIN user_credentials uc ON uc.user_id = u.id
		WHERE uc.kind = $1
			AND uc.secret = $2
			AND u.deleted_at IS NULL
		LIMIT 1;
	`

	err := dbPool.QueryRow(ctx, qFind, prv.Name(), extUser.ID).Scan(&user.ID, &user.Role)

	if err == nil {
		return &user, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	const qLink = `
		WITH existing_user AS (
			SELECT id, role, false as is_new FROM users WHERE email = $1 AND deleted_at IS NULL
		),
		target_user AS (
			INSERT INTO users (email, username, preferred_lang, is_verified)
			SELECT $1, $2, $3, TRUE
			WHERE NOT EXISTS (SELECT 1 FROM existing_user)
			ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
			RETURNING id, role, true as is_new
		),
		final_user AS (
			SELECT id, role, is_new FROM target_user
			UNION ALL
			SELECT id, role, is_new FROM existing_user
			LIMIT 1
		)
		INSERT INTO user_credentials (user_id, kind, secret)
		SELECT id, $4, $5 FROM final_user
		ON CONFLICT (user_id, kind) DO UPDATE SET 
			secret = EXCLUDED.secret,
			updated_at = NOW()
		RETURNING user_id, (SELECT role FROM final_user), (SELECT is_new FROM final_user);
	`
	isNewUser := false

	chosenUsername := extUser.Username

	const maxAttempts = 5

	for i := range maxAttempts {
		err = dbPool.QueryRow(ctx, qLink,
			extUser.Email,
			chosenUsername,
			lang,
			prv.Name(),
			extUser.ID,
		).Scan(&user.ID, &user.Role, &isNewUser)

		if err == nil {
			if isNewUser {
				go h.EmailManager.SendWelcomeEmail(email.UserDetail{
					Lang: lang,
					UserContact: email.UserContact{
						Username: extUser.Username,
						Email:    extUser.Email,
					},
				})
			}
			return &user, nil
		}

		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {

			if strings.Contains(pgErr.ConstraintName, "secret") {
				return nil, web.NewError(http.StatusConflict, "err:account_taken", SocialAccountTaken, nil)
			}

			if strings.Contains(pgErr.ConstraintName, "username") {
				switch i {
				case 0, 1:
					chosenUsername = fmt.Sprintf("%s%d", extUser.Username, i+1)
				case 2:
					chosenUsername = fmt.Sprintf("%s-%s", extUser.Username, util.RandomInt(99))
				default:
					chosenUsername = fmt.Sprintf("%s-%d", extUser.Username, time.Now().Unix()%1000)
				}
				continue
			}
		}

		return nil, web.NewError(http.StatusInternalServerError, "err:internal", err, nil)
	}

	return nil, web.NewError(http.StatusConflict, "err:username_unavailable", nil, nil)
}
