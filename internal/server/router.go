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

	baseStack := []Middleware{recoveryMiddleware, srv.analyticsService.Middleware, i18n.Middleware}

	protectedStack := append([]Middleware{srv.authHandler.Middleware, srv.rateLimited}, baseStack...)

	guestStack := append([]Middleware{srv.authHandler.MiddlewareGuestOnly}, baseStack...)

	// --- Static Sites ---
	rootStack := append([]Middleware{srv.authHandler.MiddlewareSoft}, baseStack...)
	mux.Handle("GET /", Chain(http.HandlerFunc(srv.HandleRoot), rootStack...))

	mux.Handle("GET /sign-in", Chain(srv.serveHtml("login"), guestStack...))
	mux.Handle("GET /sign-up", Chain(srv.serveHtml("register"), guestStack...))

	// --- Public API ---
	mux.Handle("GET /api/health", http.HandlerFunc(srv.HandleHealth))

	// --- Protected API ---
	mux.Handle("GET /api/weather", Chain(http.HandlerFunc(srv.HandleWeather), protectedStack...))
	mux.Handle("GET /api/location", Chain(http.HandlerFunc(srv.HandleLocation), protectedStack...))

	// --- Auth API ---
	mux.Handle("GET /api/auth/logout", Chain(http.HandlerFunc(srv.authHandler.Logout),
		recoveryMiddleware, srv.authHandler.Middleware))

	mux.Handle("POST /api/auth/register", Chain(http.HandlerFunc(srv.authHandler.Register), guestStack...))
	mux.Handle("POST /api/auth/login", Chain(http.HandlerFunc(srv.authHandler.Login), guestStack...))
	mux.Handle("GET  /api/auth/verify", Chain(http.HandlerFunc(srv.authHandler.VerifyEmail), guestStack...))

	refreshStack := append([]Middleware{srv.authHandler.MiddlewareSoft, srv.rateLimited}, baseStack...)
	mux.Handle("GET /api/auth/refresh", Chain(http.HandlerFunc(srv.authHandler.Refresh), refreshStack...))

	mux.Handle("GET /api/auth/e/login", Chain(http.HandlerFunc(srv.authHandler.OAuthLogin), guestStack...))
	mux.Handle("GET /api/auth/e/callback", Chain(http.HandlerFunc(srv.authHandler.OAuthCallback), guestStack...))

	mux.Handle("POST /api/setLang", Chain(http.HandlerFunc(i18n.HandleSetLang), recoveryMiddleware))

	return mux
}
