package server

import (
	"net/http"
	"os"

	"github.com/7apri/SimpleGOWebserver/internal/i18n"
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

	mux.Handle("GET /", Chain(http.HandlerFunc(srv.HandleRoot),
		recoveryMiddleware,
		srv.authHandler.MiddlewareSoft,
		i18n.Middleware,
	))

	guestOnlySite := []Middleware{
		recoveryMiddleware,
		srv.authHandler.MiddlewareGuestOnly,
		i18n.Middleware,
	}

	mux.Handle("GET /login", Chain(srv.serveHtml("login"), guestOnlySite...))
	mux.Handle("GET /register", Chain(srv.serveHtml("register"), guestOnlySite...))

	mux.Handle("GET /api/health", Chain(http.HandlerFunc(srv.HandleHealth)))

	mux.Handle("GET /api/weather", Chain(http.HandlerFunc(srv.HandleWeather),
		recoveryMiddleware,
		srv.authHandler.Middleware,
		srv.rateLimited,
		srv.analyticsService.Middleware,
		i18n.Middleware,
	))
	mux.Handle("GET /api/location", Chain(http.HandlerFunc(srv.HandleLocation),
		recoveryMiddleware,
		srv.authHandler.Middleware,
		srv.rateLimited,
		srv.analyticsService.Middleware,
	))
	mux.Handle("GET /api/auth/refresh", Chain(http.HandlerFunc(srv.authHandler.Refresh),
		recoveryMiddleware,
		srv.authHandler.Middleware,
		srv.rateLimited,
		srv.analyticsService.Middleware,
	))

	guestOnlyApi := []Middleware{
		recoveryMiddleware,
		srv.authHandler.MiddlewareGuestOnly,
	}
	mux.Handle("POST /api/auth/register", Chain(http.HandlerFunc(srv.authHandler.Register), guestOnlyApi...))
	mux.Handle("POST /api/auth/login", Chain(http.HandlerFunc(srv.authHandler.Login), guestOnlyApi...))
	mux.Handle("GET  /api/auth/verify", Chain(http.HandlerFunc(srv.authHandler.VerifyEmail), guestOnlyApi...))
	mux.Handle("GET  /api/auth/e/login", Chain(http.HandlerFunc(srv.authHandler.OAuthLogin), guestOnlyApi...))
	mux.Handle("GET  /api/auth/e/callback", Chain(http.HandlerFunc(srv.authHandler.OAuthCallback), guestOnlyApi...))
	mux.Handle("GET  /api/auth/logout", Chain(http.HandlerFunc(srv.authHandler.Logout),
		recoveryMiddleware,
		srv.authHandler.Middleware,
	))

	mux.Handle("POST /api/setLang", Chain(http.HandlerFunc(i18n.HandleSetLang), recoveryMiddleware))

	return mux
}
