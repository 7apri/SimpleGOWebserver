package server

import (
	"html/template"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"strings"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	"github.com/redis/go-redis/v9"
)

type templateWrapper struct {
	*template.Template
}

func (tw *templateWrapper) Execute(wr io.Writer, data any) error {
	return tw.ExecuteTemplate(wr, "base", data)
}

type Server struct {
	locationService  *location.LocationService
	weatherService   *weather.WeatherService
	analyticsService *analytics.Service
	authHandler      *auth.AuthHandler
	database         *database.Database
	redis            *redis.Client
	templates        map[string]map[string]*templateWrapper
	userLimiters     sync.Map
}

func NewServer(locationSr *location.LocationService, weatherSr *weather.WeatherService, authHl *auth.AuthHandler, db *database.Database, templateFs fs.FS, rdb *redis.Client, i18nMgr *i18n.Manager, analyticsSr *analytics.Service) *Server {
	srv := &Server{
		locationService:  locationSr,
		weatherService:   weatherSr,
		analyticsService: analyticsSr,
		authHandler:      authHl,
		database:         db,
		templates:        make(map[string]map[string]*templateWrapper),
		redis:            rdb,
	}

	srv.initTemplates(templateFs, i18nMgr)
	srv.cleanupLimiters()
	return srv
}

func (s *Server) initTemplates(tmplFS fs.FS, mgr *i18n.Manager) {
	pages, err := fs.ReadDir(tmplFS, ".")
	if err != nil {
		log.Fatalf("Error reading subdir templates: %s", err)
	}

	for _, lang := range mgr.GetAvailableLangs() {
		s.templates[lang] = make(map[string]*templateWrapper)
	}

	for _, page := range pages {
		name := page.Name()

		if name == "base.html" || page.IsDir() {
			continue
		}

		pageKey := strings.TrimSuffix(name, ".html")
		slog.Info("adding page", "key", pageKey)

		const brandName = "SimpleDash"
		const titleLimit = 15

		for _, lang := range mgr.GetAvailableLangs() {
			tmpl := template.New(name).Funcs(template.FuncMap{
				"tr": func(key string) string {
					return mgr.Get(lang, key)
				},
				"trJSON": func() template.JS {
					return mgr.GetJSON(lang, pageKey)
				},
				"fullTitle": func(key string) string {
					title := mgr.Get(lang, key)
					if strings.HasSuffix(title, "#!") || title == "" {
						slog.Warn("Missing title translation", "key", key, "lang", lang)
						return brandName
					}
					if len([]byte(title)) > titleLimit {
						return title
					}
					return title + " | " + brandName
				},
			})
			_, err = tmpl.ParseFS(tmplFS, "base.html", name)
			if err != nil {
				log.Fatalf("Error parsing %s: %s", name, err)
			}

			s.templates[lang][pageKey] = &templateWrapper{
				tmpl,
			}
		}

	}
}
