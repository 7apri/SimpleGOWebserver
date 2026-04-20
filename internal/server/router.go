package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"golang.org/x/time/rate"
)

func (s *Server) onErrHtml(w http.ResponseWriter, r *http.Request, buffer *bytes.Buffer, appErr *web.WebError) *web.WebError {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	lang := i18n.GetLangFromReq(r)
	err := s.templateMgr.WriteTemplate(buffer, lang, templates.TemplateKey{
		Kind: "page",
		Name: "err/" + strconv.Itoa(appErr.Status),
	}, appErr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}

func (s *Server) handlerHtml(h func(w http.ResponseWriter, r *http.Request) *web.WebError, middleware ...web.Middleware) http.Handler {
	return web.Handler(h).With(s.i18Mgr, s.onErrHtml, middleware...)
}

func (srv *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	baseStack := []web.Middleware{web.RecoveryM, srv.analyticsService.Middleware, i18n.Middleware}

	// guestStack := append([]web.Middleware{srv.authHandler.MiddlewareGuestOnly}, baseStack...)
	// protectedStack := append([]web.Middleware{srv.authHandler.Middleware}, baseStack...)

	rootStack := append([]web.Middleware{srv.authHandler.MiddlewareSoft}, baseStack...)
	mux.Handle("GET /", srv.handlerHtml(srv.HandleRoot, rootStack...))

	mux.Handle("GET /sign-in", web.Chain(srv.serveHtml("auth/login"), baseStack...))
	mux.Handle("GET /sign-up", srv.handlerHtml(srv.HandleSignUp, baseStack...))

	mux.Handle("GET /2fa", web.Chain(srv.serveHtml("2fa/enter-code"), baseStack...))

	mux.Handle("GET /password-reset", srv.handlerHtml(srv.handleChallengeUI(
		challengeUIConfig{
			actionName: "reset",
			pageKey: templates.TemplateKey{
				Kind: "page",
				Name: "auth/reset",
			},
			cType: consts.UserChallengeReset,
		}), rootStack...))

	mux.Handle("GET /account-verify", srv.handlerHtml(srv.handleChallengeUI(
		challengeUIConfig{
			actionName: "verify",
			pageKey: templates.TemplateKey{
				Kind: "page",
				Name: "auth/verify",
			},
			cType: consts.UserChallengeVerify,
		}), baseStack...))

	mux.Handle("GET /api/health", http.HandlerFunc(srv.HandleHealth))

	baseRateLimit := srv.rateLimited("", rate.Every(time.Second), 5)

	protectedApiStack := append([]web.Middleware{srv.authHandler.Middleware, baseRateLimit}, baseStack...)

	mux.Handle("GET /api/weather", web.Chain(http.HandlerFunc(srv.HandleWeather), protectedApiStack...))
	mux.Handle("GET /api/location", web.Chain(http.HandlerFunc(srv.HandleLocation), protectedApiStack...))
	mux.Handle("GET /api/sendEmail", web.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.authHandler.EmailManager.SendEmail(&email.EmailCtx{
			Reciever: "test@panels.com",
			EmailTemplateIdentifier: email.EmailTemplateIdentifier{
				Lang: i18n.GetLangFromReq(r),
				Name: "verify",
				Data: email.EmailDataSecurity{
					Username:   "dev",
					Email:      "test@panels.com",
					Code:       []string{"123", "456"},
					Token:      "test",
					SecureLink: "verify first",
				},
			},
		})
		next := r.URL.Query().Get("next")
		if next == "" {
			next = "/"
		}
		http.Redirect(w, r, next, http.StatusPermanentRedirect)
	}), protectedApiStack...))

	mux.Handle("GET /api/auth/logout", web.Chain(
		http.HandlerFunc(srv.authHandler.Logout),
		web.RecoveryM,
		srv.authHandler.Middleware,
	))

	authRateLimit := srv.rateLimited("auth:", rate.Limit(1), 5)

	authApiStack := append([]web.Middleware{auth.CSRFMiddleware(srv.i18Mgr), authRateLimit}, baseStack...)

	authExtRateLimit := srv.rateLimited("authExt:", rate.Limit(2), 5)
	authExtApiStack := append([]web.Middleware{authExtRateLimit}, baseStack...)

	authApiStackQuantize := append([]web.Middleware{web.QuantizeDelay(300*time.Millisecond, 50)}, authApiStack...)

	mux.Handle("POST /api/auth/register", srv.handlerHtml(srv.authHandler.Register, authApiStackQuantize...))

	verifyUsernameStack := append([]web.Middleware{auth.CSRFMiddleware(srv.i18Mgr), srv.rateLimited("user:", rate.Every(time.Second), 15)}, baseStack...)
	mux.Handle("POST /api/verify-username", srv.handlerHtml(srv.authHandler.VerifyUsername, verifyUsernameStack...))

	mux.Handle("POST /api/auth/login", srv.handlerHtml(srv.authHandler.Login, authApiStackQuantize...))

	authApiTwoFAStack := append([]web.Middleware{srv.authHandler.Middleware}, authApiStack...)
	mux.Handle("POST /api/auth/2fa/init", srv.handlerHtml(srv.authHandler.HandleInit2FA, authApiTwoFAStack...))

	mux.Handle("POST /api/auth/2fa/enable", srv.handlerHtml(srv.authHandler.HandleVerifyAndEnable2FA, authApiTwoFAStack...))

	mux.Handle("POST /api/auth/2fa/recovery/regen", srv.handlerHtml(srv.authHandler.HandleRegenerateRecoveryCodes, authApiTwoFAStack...))

	authApiTwoFALoginStack := append([]web.Middleware{srv.authHandler.MiddlewareTwoFA}, authApiStackQuantize...)
	mux.Handle("POST /api/auth/2fa/recovery/verify", srv.handlerHtml(srv.authHandler.HandleVerifyRecoveryCode, authApiTwoFALoginStack...))
	mux.Handle("POST /api/auth/2fa/login", srv.handlerHtml(srv.authHandler.HandleLoginVerify2FA, authApiTwoFALoginStack...))

	InitReset := srv.authHandler.InitEmailChallenge(
		consts.UserChallengeReset,
		time.Minute,
		15*time.Minute,
		"reset_token",
		false,
		srv.authHandler.EmailManager.SendPasswordResetEmail,
	)
	InitVerify := srv.authHandler.InitEmailChallenge(
		consts.UserChallengeVerify,
		time.Minute,
		15*time.Minute,
		"verify_token",
		true,
		srv.authHandler.EmailManager.SendVerificationEmail,
	)

	mux.Handle("POST /api/auth/reset/init", srv.handlerHtml(InitReset, authApiStackQuantize...))
	mux.Handle("POST /api/auth/reset/confirm", srv.handlerHtml(srv.authHandler.ConfirmReset, authApiStackQuantize...))
	mux.Handle("POST /api/auth/reset/2fa", srv.handlerHtml(srv.authHandler.HandleResetVerify2FA, authApiStackQuantize...))
	mux.Handle("POST /api/auth/reset/2fa/recovery", srv.handlerHtml(srv.authHandler.HandleResetVerifyRecoveryCode, authApiStackQuantize...))

	mux.Handle("POST /api/auth/reset/check", srv.handlerHtml(srv.authHandler.CheckCodeReset, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/check", srv.handlerHtml(srv.authHandler.CheckCodeVerify, authApiStackQuantize...))

	mux.Handle("POST /api/auth/verify/init", srv.handlerHtml(InitVerify, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/confirm", srv.handlerHtml(srv.authHandler.ConfirmVerify, authApiStackQuantize...))

	crsfStack := append([]web.Middleware{baseRateLimit}, baseStack...)
	mux.Handle("GET /api/csrf", srv.handlerHtml(auth.CSRFEndpoint, crsfStack...))

	refreshStack := append([]web.Middleware{srv.authHandler.MiddlewareSoft}, crsfStack...)
	mux.Handle("GET /api/auth/refresh", web.Chain(http.HandlerFunc(srv.authHandler.Refresh), refreshStack...))

	mux.Handle("GET  /api/auth/e/login", srv.handlerHtml(srv.authHandler.OAuthLogin, authExtApiStack...))
	mux.Handle("GET  /api/auth/e/callback", srv.handlerHtml(srv.authHandler.OAuthCallback, authExtApiStack...))
	mux.Handle("POST /api/auth/e/finalize", srv.handlerHtml(srv.authHandler.FinalizeExternal, authExtApiStack...))
	mux.Handle("GET  /api/auth/e/cancel", srv.handlerHtml(srv.authHandler.CancelPendingAuth, authExtApiStack...))

	mux.Handle("GET /ws-reload", srv.handlerHtml(refreshWebsocket(srv.templateMgr.RefreshChan, srv.shutdownChan), web.RecoveryM))
	return mux
}
