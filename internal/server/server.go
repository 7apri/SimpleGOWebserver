package server

import (
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
	templateMgr      *templates.TemplateManager
	i18Mgr           *i18n.I18nManager
}

func NewServer(
	locationSr *location.LocationService,
	weatherSr *weather.WeatherService,
	authHl *auth.AuthHandler,
	db *database.Database,
	rdb *redis.Client,
	i18nMgr *i18n.I18nManager,
	templateMgr *templates.TemplateManager,
	analyticsSr *analytics.Service,
) *Server {
	srv := &Server{
		locationService:  locationSr,
		weatherService:   weatherSr,
		analyticsService: analyticsSr,
		authHandler:      authHl,
		database:         db,
		templateMgr:      templateMgr,
		i18Mgr:           i18nMgr,
		redis:            rdb,
	}

	srv.cleanupLimiters()
	return srv
}
