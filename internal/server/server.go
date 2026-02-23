package server

import (
	"io/fs"
	"sync"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
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
	userLimiters     sync.Map
	templateMgr      *templates.Manager
}

func NewServer(locationSr *location.LocationService, weatherSr *weather.WeatherService, authHl *auth.AuthHandler, db *database.Database, templateFs fs.FS, rdb *redis.Client, i18nMgr *i18n.Manager, templateMgr *templates.Manager, analyticsSr *analytics.Service) *Server {
	srv := &Server{
		locationService:  locationSr,
		weatherService:   weatherSr,
		analyticsService: analyticsSr,
		authHandler:      authHl,
		database:         db,
		templateMgr:      templateMgr,
		redis:            rdb,
	}

	srv.cleanupLimiters()
	return srv
}
