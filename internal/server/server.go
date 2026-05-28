package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/social"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	"github.com/7apri/SimpleGOWebserver/internal/websocket"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RouteWrapper struct {
	locationService  *location.LocationService
	weatherService   *weather.WeatherService
	analyticsService *analytics.Service
	authHandler      *auth.AuthHandler
	database         *database.Database
	redis            *redis.Client
	userLimiters     sync.Map
	templateMgr      *templates.TemplateManager
	i18Mgr           *i18n.I18nManager
	websocketHub     *websocket.WebsocketHub
	socialWrapper    *social.SocialWrapper
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
	websocketHub *websocket.WebsocketHub,
	socialWrapper *social.SocialWrapper,
) *http.Server {
	w := &RouteWrapper{
		locationService:  locationSr,
		weatherService:   weatherSr,
		analyticsService: analyticsSr,
		authHandler:      authHl,
		database:         db,
		templateMgr:      templateMgr,
		i18Mgr:           i18nMgr,
		redis:            rdb,
		websocketHub:     websocketHub,
		socialWrapper:    socialWrapper,
	}
	s := &http.Server{
		Addr:         ":8080",
		Handler:      w.Routes(),
		IdleTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		for {
			select {
			case action, ok := <-templateMgr.RefreshChan:
				if !ok {
					return
				}
				switch action {
				case templates.SignalReload:
					websocketHub.Broadcast(websocket.TopicGlobal, uuid.Nil, []byte(`{"act":"h"}`))
				case templates.SignalCSS:
					websocketHub.Broadcast(websocket.TopicGlobal, uuid.Nil, []byte(`{"act":"s"}`))
				}
			}
		}
	}()
	w.cleanupLimiters()
	return s
}
