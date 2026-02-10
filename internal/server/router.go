package server

import (
	"io/fs"
	"net/http"
)

func (srv *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	getRecover := []Middleware{
		recoveryMiddleware,
		NewAllowMethodMiddleware("GET"),
	}

	guestOnlySite := []Middleware{
		recoveryMiddleware,
		NewAllowMethodMiddleware("GET"),
		srv.authHandler.MiddlewareGuestOnly,
	}

	api := []Middleware{
		recoveryMiddleware,
		NewAllowMethodMiddleware("GET"),
		srv.authHandler.Middleware,
		srv.rateLimited,
		srv.analyticsService.Middleware,
	}

	guestOnlyApi := []Middleware{
		recoveryMiddleware,
		NewAllowMethodMiddleware("POST"),
		srv.authHandler.MiddlewareGuestOnly,
	}

	mux.Handle("/", Chain(srv.authHandler.MiddlewareSoft(http.HandlerFunc(srv.HandleRoot)), getRecover...))

	mux.Handle("/login", Chain(srv.serveHtml("login.html", nil), guestOnlySite...))
	mux.Handle("/register", Chain(srv.serveHtml("register.html", nil), guestOnlySite...))

	mux.Handle("/api/health", Chain(http.HandlerFunc(srv.HandleHealth), getRecover...))

	mux.Handle("/api/weather", Chain(http.HandlerFunc(srv.HandleWeather), api...))

	mux.Handle("/api/location", Chain(http.HandlerFunc(srv.HandleLocation), api...))

	mux.Handle("/api/auth/register", Chain(http.HandlerFunc(srv.authHandler.Register), guestOnlyApi...))
	mux.Handle("/api/auth/login", Chain(http.HandlerFunc(srv.authHandler.Login), guestOnlyApi...))

	mux.Handle("/api/auth/logout", Chain(http.HandlerFunc(srv.authHandler.Logout),
		recoveryMiddleware,
		NewAllowMethodMiddleware("POST"),
		srv.authHandler.Middleware,
	))
	mux.Handle("/api/auth/refresh", Chain(http.HandlerFunc(srv.authHandler.Refresh), api...))

	staticFiles, _ := fs.Sub(srv.siteEmbed, "site/static")
	mux.Handle("/static/", http.StripPrefix("/static/", CacheMiddleware(http.FileServer(http.FS(staticFiles)))))

	return mux
}
