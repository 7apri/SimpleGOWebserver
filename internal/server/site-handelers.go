package server

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

func generateUserETag(user *auth.UserPrintTimestamp) string {
	h := md5.New()
	io.WriteString(h, user.ID.String())
	io.WriteString(h, strconv.FormatInt(user.UpdatedAt.UnixNano(), 10))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (server *Server) HandleRoot(w http.ResponseWriter, r *http.Request) *web.WebError {
	if r.URL.Path != "/" {
		return web.NewError(http.StatusNotFound, "err_not_found", nil, nil)
	}
	user, loggedIn := auth.GetUser(r.Context())

	if !loggedIn {
		http.Redirect(w, r, "/api/auth/refresh", http.StatusTemporaryRedirect)
		return nil
	}

	return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: "main"}, generateUserETag(user), user)
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
		return server.templateMgr.WriteTemplateETag(w, r, templates.TemplateKey{Kind: "page", Name: name}, generateUserETag(user), user)
	})
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

type challengeUIConfig struct {
	actionName string
	pageKey    templates.TemplateKey
	cType      consts.UserChallengeType
}

func (server *Server) handleChallengeUI(cfg challengeUIConfig) func(w http.ResponseWriter, r *http.Request) *web.WebError {
	actionCookieName := cfg.actionName + "_claims"
	tokenCookieName := cfg.actionName + "_token"
	return func(w http.ResponseWriter, r *http.Request) *web.WebError {
		q := r.URL.Query()
		t, c := q.Get("t"), q.Get("c")

		if t != "" {
			setChallengeCookie(w, tokenCookieName, t, 900)

			if c != "" {
				_, err := server.authHandler.ProcessChallengeVerification(w, r, cfg.cType, t, cfg.actionName, c)
				if err != nil {
					return err
				}
			}

			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return nil
		}

		state := "email"

		if cookie, err := r.Cookie(actionCookieName); err == nil {
			claims, err := server.authHandler.GetChallengeClaims(cookie.Value)
			if err == nil && claims.Action == cfg.actionName {
				if claims.MfaPending {
					state = "2fa"
				} else {
					state = "success"
				}
			}
		} else if hasRaw, _ := cookieExists(r, tokenCookieName); hasRaw {
			state = "code"
		}

		return server.templateMgr.WriteTemplateETag(w, r, cfg.pageKey, state, map[string]any{
			"State": state,
		})
	}
}
