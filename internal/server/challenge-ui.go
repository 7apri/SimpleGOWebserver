package server

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

type challengeUIConfig struct {
	tokenName     string
	codeName      string
	codeMaxAge    int
	tokenMaxAge   int
	pageKey       templates.TemplateKey
	challengeType email.ChallengeType
}

func setChallengeCookie(w http.ResponseWriter, cookieName, val string, cookieMaxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:  cookieName,
		Value: val,
		Path:  "/", MaxAge: cookieMaxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}
func cookieExists(r *http.Request, cookieName string) (bool, *http.Cookie) {
	if c, err := r.Cookie(cookieName); err == nil {
		return true, c
	}
	return false, nil
}

func (rw *RouteWrapper) handleChallengeUI(cfg challengeUIConfig) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		q := r.URL.Query()
		t, c := q.Get("t"), q.Get("c")

		if t != "" || c != "" {
			if t != "" {
				setChallengeCookie(w, cfg.tokenName, t, cfg.tokenMaxAge)
			}
			if c != "" {
				if _, err := rw.authHandler.VerifyChallenge(r, cfg.challengeType, t, c); err == nil {
					slog.Error("ad", "err", err)
					setChallengeCookie(w, cfg.codeName, c, cfg.codeMaxAge)
				}
			}

			q.Del("t")
			q.Del("c")
			target := r.URL.Path
			if qs := q.Encode(); qs != "" {
				target += "?" + qs
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return nil
		}

		hasToken, _ := cookieExists(r, cfg.tokenName)
		hasCode, _ := cookieExists(r, cfg.codeName)

		state := "email"
		if hasToken {
			if hasCode {
				state = "success"
			} else {
				state = "code"
			}
		}

		_, isLoggedIn := auth.GetUser(r.Context())

		return rw.templateMgr.WriteTemplateETag(w, r, cfg.pageKey, map[string]any{
			"State":    state,
			"LoggedIn": isLoggedIn,
		}, state, strconv.FormatBool(isLoggedIn))
	}
}
