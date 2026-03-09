package server

import (
	"net/http"
	"os"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/email"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/web"
)

type noDirFS struct {
	fs http.FileSystem
}

func (n noDirFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, os.ErrNotExist
	}

	return f, nil
}

func (srv *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	baseStack := []web.Middleware{web.RecoveryM, srv.analyticsService.Middleware, i18n.Middleware}

	protectedStack := append([]web.Middleware{srv.authHandler.Middleware, srv.rateLimited}, baseStack...)

	guestStack := append([]web.Middleware{srv.authHandler.MiddlewareGuestOnly}, baseStack...)

	// --- Static Sites ---
	rootStack := append([]web.Middleware{srv.authHandler.MiddlewareSoft}, baseStack...)
	mux.Handle("GET /", web.Handler(srv.HandleRoot).With(srv.i18Mgr, rootStack...))

	mux.Handle("GET /sign-in", srv.serveHtml("auth/login"))
	mux.Handle("GET /sign-up", web.Chain(srv.serveHtml("auth/register"), guestStack...))

	mux.Handle("GET /password-reset", web.Handler(srv.handleChallengeUI(
		challengeUIConfig{
			tokenName:   "reset_token",
			codeName:    "reset_code_tmp",
			codeMaxAge:  300,
			tokenMaxAge: 900,
			basePage: pageTemplateData{
				data: nil,
				key: templates.TemplateKey{
					Kind: "page",
					Name: "reset/email-request",
				},
			},
		}, func(w http.ResponseWriter, r *http.Request) *web.WebError {
			return srv.templateMgr.RenderPage(w, r, templates.TemplateKey{
				Kind: "page",
				Name: "reset/new-password",
			}, nil)
		}, func(w http.ResponseWriter, r *http.Request) *web.WebError {
			return srv.templateMgr.RenderPage(w, r, templates.TemplateKey{
				Kind: "page",
				Name: "reset/enter-code",
			}, nil)
		},
	)).With(srv.i18Mgr, guestStack...))

	mux.Handle("GET /account-verify", web.Handler(srv.handleChallengeUI(
		challengeUIConfig{
			tokenName:   "verify_token",
			codeName:    "verify_code_tmp",
			codeMaxAge:  300,
			tokenMaxAge: 900,
			basePage: pageTemplateData{
				data: nil,
				key: templates.TemplateKey{
					Kind: "page",
					Name: "verify/email-request",
				},
			},
		}, func(w http.ResponseWriter, r *http.Request) *web.WebError {
			return srv.templateMgr.RenderPage(w, r, templates.TemplateKey{
				Kind: "page",
				Name: "verify/confirm-challenge",
			}, nil)
		},
		func(w http.ResponseWriter, r *http.Request) *web.WebError {
			return srv.templateMgr.RenderPage(w, r, templates.TemplateKey{
				Kind: "page",
				Name: "verify/enter-code",
			}, nil)
		},
	)).With(srv.i18Mgr, guestStack...))

	// --- Public API ---
	mux.Handle("GET /api/health", http.HandlerFunc(srv.HandleHealth))

	// --- Protected API ---
	mux.Handle("GET /api/weather", web.Chain(http.HandlerFunc(srv.HandleWeather), protectedStack...))
	mux.Handle("GET /api/location", web.Chain(http.HandlerFunc(srv.HandleLocation), protectedStack...))

	// --- Auth API ---
	mux.Handle("GET /api/auth/logout", web.Chain(
		http.HandlerFunc(srv.authHandler.Logout),
		web.RecoveryM,
		srv.authHandler.Middleware,
	))

	authApiStack := append([]web.Middleware{srv.authRateLimited}, guestStack...)
	authApiStackQuantize := append([]web.Middleware{web.QuantizeDelay(150*time.Millisecond, 20)}, authApiStack...)

	mux.Handle("POST /api/auth/register", web.Handler(srv.authHandler.Register).With(srv.i18Mgr, authApiStackQuantize...))
	mux.Handle("POST /api/auth/login", web.Handler(srv.authHandler.Login).With(srv.i18Mgr, authApiStackQuantize...))

	InitReset := srv.authHandler.InitEmailChallenge(
		email.ChallengeReset,
		time.Minute,
		15*time.Minute,
		"reset_token",
		false,
		srv.authHandler.EmailManager.SendPasswordResetEmail,
	)
	InitVerify := srv.authHandler.InitEmailChallenge(
		email.ChallengeVerify,
		time.Minute,
		15*time.Minute,
		"verify_token",
		true,
		srv.authHandler.EmailManager.SendVerificationEmail,
	)
	mux.Handle("POST /api/auth/reset/init", web.Handler(InitReset).With(srv.i18Mgr, authApiStackQuantize...))
	mux.Handle("POST /api/auth/reset/confirm", web.Handler(srv.authHandler.ConfirmReset).With(srv.i18Mgr, authApiStackQuantize...))

	mux.Handle("POST /api/auth/reset/check", web.Handler(srv.authHandler.CheckChallengeCode("reset_token", email.ChallengeReset)).With(srv.i18Mgr, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/check", web.Handler(srv.authHandler.CheckChallengeCode("verify_token", email.ChallengeVerify)).With(srv.i18Mgr, authApiStackQuantize...))

	mux.Handle("POST /api/auth/verify/init", web.Handler(InitVerify).With(srv.i18Mgr, authApiStackQuantize...))
	mux.Handle("POST /api/auth/verify/confirm", web.Handler(srv.authHandler.ConfirmVerify).With(srv.i18Mgr, authApiStackQuantize...))

	refreshStack := append([]web.Middleware{srv.authHandler.MiddlewareSoft, srv.rateLimited}, baseStack...)
	mux.Handle("GET /api/auth/refresh", web.Chain(http.HandlerFunc(srv.authHandler.Refresh), refreshStack...))

	mux.Handle("GET /api/auth/e/login", web.Handler(srv.authHandler.OAuthLogin).With(srv.i18Mgr, authApiStack...))
	mux.Handle("GET /api/auth/e/callback", web.Handler(srv.authHandler.OAuthCallback).With(srv.i18Mgr, authApiStack...))

	mux.Handle("POST /api/setLang", web.Chain(http.HandlerFunc(i18n.HandleSetLang), web.RecoveryM))

	mux.Handle("GET /ws-reload", web.Handler(refreshWebsocket(srv.templateMgr.RefreshChan)).With(srv.i18Mgr))
	return mux
}
