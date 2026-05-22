package server

import (
	"net/http"
	"strconv"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) *web.WebError {
	if r.URL.Path != "/" {
		return web.NewError(http.StatusNotFound, "err_not_found", nil, nil)
	}
	user, loggedIn := auth.GetUser(r.Context())

	if !loggedIn {
		http.Redirect(w, r, "/api/auth/refresh", http.StatusTemporaryRedirect)
		return nil
	}

	return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "main"}, "", user)
}

func (server *Server) HandleSignUp(w http.ResponseWriter, r *http.Request) *web.WebError {
	cookie, err := r.Cookie("oauth_pending")

	if err == nil && cookie.Value != "" {
		claims, err := server.authHandler.GetPendingAuthProviderClaims(cookie.Value)
		if err == nil && claims != nil {
			return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "auth/finish-external"}, claims.AvatarURL, claims)
		}
	}

	return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "auth/register"}, "", nil)
}

func (server *Server) serveHtml(name string) http.Handler {
	return server.handlerHtml(func(w http.ResponseWriter, r *http.Request) *web.WebError {
		return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: name}, "", nil)
	})
}
func (server *Server) serveHtmlUser(name string) http.Handler {
	return server.handlerHtml(func(w http.ResponseWriter, r *http.Request) *web.WebError {
		user, _ := auth.GetUser(r.Context())
		return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: name}, "", user)
	})
}

type challengeUIConfig struct {
	tokenName   string
	codeName    string
	codeMaxAge  int
	tokenMaxAge int
	pageKey     templates.TemplateKey
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

func (server *Server) handleChallengeUI(cfg challengeUIConfig) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		q := r.URL.Query()
		t, c := q.Get("t"), q.Get("c")

		if t != "" || c != "" {
			if t != "" {
				setChallengeCookie(w, cfg.tokenName, t, cfg.tokenMaxAge)
			}
			if c != "" {
				setChallengeCookie(w, cfg.codeName, c, cfg.codeMaxAge)
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

		return server.templateMgr.WriteTemplateETag(w, r, cfg.pageKey, state+strconv.FormatBool(isLoggedIn), map[string]any{
			"State":    state,
			"LoggedIn": isLoggedIn,
		})
	}
}
