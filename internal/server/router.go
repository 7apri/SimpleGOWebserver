package server

import (
	"html/template"
	"net/http"

	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	util "github.com/7apri/SimpleGOWebserver/pkg"
)

type Server struct {
	LocationService *location.LocationService
	WeatherService  *weather.WeatherService
	AuthHandler     *auth.AuthHandler
	Database        *database.Database
	Templates       *template.Template
}

func (srv *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/", util.AllowMethods(http.HandlerFunc(srv.HandleRoot), "GET"))

	mux.Handle("/api/health", util.AllowMethods(http.HandlerFunc(srv.HandleHealth), "GET"))

	mux.Handle("/api/weather", srv.protected(srv.HandleWeather, "GET"))
	mux.Handle("/api/location", srv.protected(srv.HandleLocation, "GET"))

	mux.Handle("/api/auth/login", util.AllowMethods(http.HandlerFunc(srv.AuthHandler.Login), "POST"))
	mux.Handle("/api/auth/register", util.AllowMethods(http.HandlerFunc(srv.AuthHandler.Register), "POST"))
	mux.Handle("/api/auth/refresh", srv.protected(http.HandlerFunc(srv.AuthHandler.Refresh), "GET"))

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	return mux
}
func (s *Server) protected(h http.HandlerFunc, m string) http.Handler {
	return s.AuthHandler.Middleware((util.AllowMethods(h, m)))
}
