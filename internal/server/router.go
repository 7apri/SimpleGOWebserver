package server

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
	"golang.org/x/time/rate"
)

func (rw *RouteWrapper) onErrHtml(w http.ResponseWriter, r *http.Request, buffer *bytes.Buffer, appErr *web.WebError) *web.WebError {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	lang := i18n.GetLangFromReq(r)
	err := rw.templateMgr.WriteTemplate(buffer, lang, templates.TemplateKey{
		Kind: "page",
		Name: "err/" + strconv.Itoa(appErr.Status),
	}, appErr)
	if err != nil {
		return web.NewError(http.StatusInternalServerError, "internal", err, nil)
	}
	return nil
}

func (rw *RouteWrapper) handlerHtml(h func(w http.ResponseWriter, r *http.Request) *web.WebError, middleware ...web.Middleware) http.Handler {
	return web.Handler(h).With(rw.i18Mgr, rw.onErrHtml, middleware...)
}

func (rw *RouteWrapper) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	baseStack := []web.Middleware{web.RecoveryM, rw.analyticsService.Middleware, rw.i18Mgr.Middleware, rw.authHandler.Middleware}

	// guestStack := append([]web.Middleware{rw.authHandler.MiddlewareGuestOnly}, baseStack...)
	protectedStack := append(baseStack, rw.authHandler.MiddlewareBlock)

	// --- Static Sites ---
	mux.Handle("GET /", rw.handlerHtml(rw.HandleRoot, baseStack...))
	mux.Handle("GET /{username}", rw.handlerHtml(rw.HandleProfile, baseStack...))
	mux.Handle("POST /{username}/follow", rw.handlerHtml(rw.HandleFollow, protectedStack...))

	mux.Handle("GET  /posts", rw.handlerHtml(rw.HandleGetFeed, baseStack...))
	mux.Handle("POST /posts", rw.handlerHtml(rw.HandleCreatePost, baseStack...))

	mux.Handle("GET /explore", web.Chain(rw.serveHtmx("main", "explore"), baseStack...))

	mux.Handle("GET /sign-in", web.Chain(rw.serveHtml("auth/login"), baseStack...))
	mux.Handle("GET /sign-up", rw.handlerHtml(rw.HandleSignUp, baseStack...))

	mux.Handle("GET /2fa", web.Chain(rw.serveHtml("2fa/enter-code"), baseStack...))
	mux.Handle("GET /2fa/setup", web.Chain(rw.serveHtmlUser("2fa/setup"), protectedStack...))

	mux.Handle("GET /password-reset", rw.handlerHtml(rw.handleChallengeUI(
		challengeUIConfig{
			tokenName:     "reset_token",
			codeName:      "reset_code_tmp",
			codeMaxAge:    300,
			tokenMaxAge:   900,
			challengeType: email.ChallengeReset,
			pageKey: templates.TemplateKey{
				Kind: "page",
				Name: "auth/reset",
			},
		}), baseStack...))

	mux.Handle("GET /account-verify", rw.handlerHtml(rw.handleChallengeUI(
		challengeUIConfig{
			tokenName:     "verify_token",
			codeName:      "verify_code_tmp",
			codeMaxAge:    300,
			tokenMaxAge:   900,
			challengeType: email.ChallengeVerify,
			pageKey: templates.TemplateKey{
				Kind: "page",
				Name: "auth/verify",
			},
		}), baseStack...))

	// --- Public API ---
	mux.Handle("GET /api/health", http.HandlerFunc(rw.HandleHealth))

	baseRateLimit := rw.rateLimited("", rate.Every(time.Second), 5)
	protectedApiStack := append(baseStack, rw.authHandler.MiddlewareBlock, baseRateLimit)

	// --- Protected API ---
	mux.Handle("GET /api/weather", web.Chain(http.HandlerFunc(rw.HandleWeather), protectedApiStack...))
	mux.Handle("GET /api/location", web.Chain(http.HandlerFunc(rw.HandleLocation), protectedApiStack...))
	mux.Handle("GET /api/sendEmail", web.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw.authHandler.EmailManager.SendEmail(&email.EmailCtx{
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

	// --- Auth API ---
	mux.Handle("GET /api/auth/logout", web.Chain(
		http.HandlerFunc(rw.authHandler.Logout),
		web.RecoveryM,
		rw.authHandler.Middleware,
		rw.authHandler.MiddlewareBlock,
	))

	authRateLimit := rw.rateLimited("auth:", rate.Limit(1), 5)
	authApiStack := append(baseStack, authRateLimit, auth.CSRFMiddleware(rw.i18Mgr))

	authExtRateLimit := rw.rateLimited("authExt:", rate.Limit(2), 5)
	authExtApiStack := append(baseStack, authExtRateLimit)

	authApiStackQuantize := append([]web.Middleware{web.QuantizeDelay(300*time.Millisecond, 50)}, authApiStack...)

	mux.Handle("POST /api/auth/register", rw.handlerHtml(rw.authHandler.Register, authApiStackQuantize...))

	verifyUsernameStack := append(baseStack, auth.CSRFMiddleware(rw.i18Mgr), rw.rateLimited("user:", rate.Every(time.Second), 15))
	mux.Handle("POST /api/verify-username", rw.handlerHtml(rw.authHandler.VerifyUsername, verifyUsernameStack...))

	mux.Handle("POST /api/auth/login", rw.handlerHtml(rw.authHandler.Login, authApiStackQuantize...))

	authApiTwoFAStack := append(authApiStack, rw.authHandler.MiddlewareBlock)
	mux.Handle("POST /api/auth/2fa/init", rw.handlerHtml(rw.authHandler.HandleInit2FA, authApiTwoFAStack...))

	mux.Handle("POST /api/auth/2fa/enable", rw.handlerHtml(rw.authHandler.HandleVerifyAndEnable2FA, authApiTwoFAStack...))

	mux.Handle("POST /api/auth/2fa/recovery/regen", rw.handlerHtml(rw.authHandler.HandleRegenerateRecoveryCodes, authApiTwoFAStack...))

	authApiTwoFALoginStack := append(authApiStackQuantize, rw.authHandler.MiddlewareTwoFA)
	mux.Handle("POST /api/auth/2fa/recovery/verify", rw.handlerHtml(rw.authHandler.VerifyRecoveryCode, authApiTwoFALoginStack...))
	mux.Handle("POST /api/auth/2fa/login", rw.handlerHtml(rw.authHandler.HandleLoginVerify2FA, authApiTwoFALoginStack...))

	InitReset := rw.authHandler.InitEmailChallenge(
		email.ChallengeReset,
		time.Minute,
		15*time.Minute,
		"reset_token",
		false,
		rw.authHandler.EmailManager.SendPasswordResetEmail,
	)
	InitVerify := rw.authHandler.InitEmailChallenge(
		email.ChallengeVerify,
		time.Minute,
		15*time.Minute,
		"verify_token",
		true,
		rw.authHandler.EmailManager.SendVerificationEmail,
	)
	mux.Handle("POST /api/auth/reset/init", rw.handlerHtml(InitReset, authApiStackQuantize...))

	resetChallengeStack := append(authApiStackQuantize, rw.authHandler.Middleware)
	mux.Handle("POST /api/auth/reset/confirm", rw.handlerHtml(rw.authHandler.ConfirmReset, resetChallengeStack...))

	mux.Handle("POST /api/auth/reset/check", rw.handlerHtml(rw.authHandler.CheckCodeReset, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/check", rw.handlerHtml(rw.authHandler.CheckCodeVerify, authApiStackQuantize...))

	mux.Handle("POST /api/auth/verify/init", rw.handlerHtml(InitVerify, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/confirm", rw.handlerHtml(rw.authHandler.ConfirmVerify, authApiStackQuantize...))

	crsfStack := append(baseStack, baseRateLimit)
	mux.Handle("GET /api/csrf", rw.handlerHtml(auth.CSRFEndpoint, crsfStack...))

	mux.Handle("GET  /api/auth/e/login", rw.handlerHtml(rw.authHandler.OAuthLogin, authExtApiStack...))
	mux.Handle("GET  /api/auth/e/callback", rw.handlerHtml(rw.authHandler.OAuthCallback, authExtApiStack...))
	mux.Handle("POST /api/auth/e/finalize", rw.handlerHtml(rw.authHandler.FinalizeExternal, authExtApiStack...))
	mux.Handle("GET  /api/auth/e/cancel", rw.handlerHtml(rw.authHandler.CancelPendingAuth, authExtApiStack...))

	//web.RecoveryM
	mux.Handle("GET /api/ws", web.MakeHandler(rw.websocketHub.HandleWS, rw.i18Mgr, nil))
	return mux
}
