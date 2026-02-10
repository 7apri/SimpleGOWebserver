package server

import (
	"html/template"
	"io/fs"
	"log"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	"github.com/redis/go-redis/v9"
)

type Server struct {
	locationService  *location.LocationService
	weatherService   *weather.WeatherService
	analyticsService *analytics.Service
	authHandler      *auth.AuthHandler
	database         *database.Database
	redis            *redis.Client
	templates        map[string]*template.Template
	userLimiters     sync.Map
	siteEmbed        fs.FS
}

func NewServer(locationSr *location.LocationService, weatherSr *weather.WeatherService, authHl *auth.AuthHandler, db *database.Database, siteEmbed fs.FS, rdb *redis.Client, analyticsSr *analytics.Service) *Server {
	srv := &Server{
		locationService:  locationSr,
		weatherService:   weatherSr,
		analyticsService: analyticsSr,
		authHandler:      authHl,
		database:         db,
		siteEmbed:        siteEmbed,
		templates:        make(map[string]*template.Template),
		redis:            rdb,
	}
	srv.cleanupLimiters()
	templateFiles, err := fs.Sub(siteEmbed, "site/templates")
	if err != nil {
		log.Fatalf("Error creating subdir templates: %s", err)
	}

	srv.initTemplates(templateFiles)
	return srv
}
func (s *Server) initTemplates(tmplFS fs.FS) {
	entries, err := fs.ReadDir(tmplFS, ".")
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		name := entry.Name()

		if name == "base.html" || entry.IsDir() {
			continue
		}

		tmpl, err := template.ParseFS(tmplFS, "base.html", name)
		if err != nil {
			log.Fatalf("Error parsing %s: %s", name, err)
		}

		s.templates[name] = tmpl
	}
}
