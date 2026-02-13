package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/redis"
	"github.com/7apri/SimpleGOWebserver/internal/server"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	"github.com/7apri/SimpleGOWebserver/pkg/util"
)

//go:embed all:site/templates
var templatesRaw embed.FS

//go:embed all:site/i18n
var i18nRaw embed.FS

var (
	templatesEmbed, _ = fs.Sub(templatesRaw, "site/templates")
	i18nEmbed, _      = fs.Sub(i18nRaw, "site/i18n")
)

const (
	coldCacheSizeLocation = 500
	coldCacheSizeWeather  = 500

	promoteThresholdLocation = 20

	promoteBufferSizeLocation = 100
	promoteBufferSizeWeather  = 100

	janitorIntervalLocation = 10 * time.Minute
	janitorIntervalWeather  = 10 * time.Minute

	saveChanBufferSizeLocation = 100
	saveChanBufferSizeWeather  = 100
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db := database.Init(ctx)
	rdb := redis.Init(ctx)

	as := analytics.NewService(db, rdb)

	slog.SetDefault(slog.New(as.LogHandler))

	weatherApiKey := util.TryGetEnvFatal("WEATHER_API_KEY")
	accessSecret := util.TryGetEnvFatal("ACCESS_SECRET_AUTH")

	githubClientID := util.TryGetEnvFatal("GITHUB_CLIENT_ID_AUTH")
	githubRedirectUrl := util.TryGetEnvFatal("GITHUB_REDIRECT_URL_AUTH")
	githubClientSecret := util.TryGetEnvFatal("GITHUB_CLIENT_SECRET_AUTH")

	googleClientID := util.TryGetEnvFatal("GOOGLE_CLIENT_ID_AUTH")
	googleRedirectUrl := util.TryGetEnvFatal("GOOGLE_REDIRECT_URL_AUTH")
	googleClientSecret := util.TryGetEnvFatal("GOOGLE_CLIENT_SECRET_AUTH")

	mgr, err := i18n.NewManager(i18nEmbed)
	if err != nil {
		log.Fatalf("Error creating the i18n manager: %s", err)
	}

	owClient := exApi.NewOwClient(weatherApiKey, 0) //time.Second

	ls, err := location.NewService(db,
		coldCacheSizeLocation,
		promoteThresholdLocation,
		promoteBufferSizeLocation,
		janitorIntervalLocation,
		saveChanBufferSizeLocation,
		owClient,
		exApi.NewIpClient(time.Minute/40),
	)
	if err != nil {
		slog.Error("There was an error creating the location service", "error", err)
		os.Exit(1)
	}
	ws, err := weather.NewService(
		db,
		coldCacheSizeWeather,
		promoteBufferSizeWeather,
		promoteBufferSizeWeather,
		janitorIntervalWeather,
		saveChanBufferSizeWeather,
		mgr.GetWeatherT(),
		owClient,
		ls,
	)
	if err != nil {
		slog.Error("There was an error creating the weather service", "error", err)
		os.Exit(1)
	}

	githubProv := auth.NewGithubOAuth(githubClientID, githubClientSecret, githubRedirectUrl)
	googleProv := auth.NewGoogleOAuth(googleClientID, googleClientSecret, googleRedirectUrl)

	au := auth.NewAuthHandler(db, accessSecret, githubProv, googleProv)

	srv := server.NewServer(ls, ws, au, db, templatesEmbed, rdb, mgr, as)

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	slog.Info(fmt.Sprintf("Server starting on %s:8080 (external:inernal)", os.Getenv("SERVER_PORT")))

	httpSrv := &http.Server{
		Addr:         ":8080",
		Handler:      srv.Routes(),
		IdleTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	httpSrv.SetKeepAlivesEnabled(false)

	go func() {
		slog.Info("Starting server on :8080")
		if err = httpSrv.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				slog.Error("There was an error running the server", "error", err)
			}
			return
		}
	}()

	<-ctx.Done()
	slog.Info("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP shutdown failed", "error", err)
	}

	if err := ls.Down(shutdownCtx); err != nil {
		slog.Error("Location service shutdown failed", "error", err)
	}
	if err := ws.Down(shutdownCtx); err != nil {
		slog.Error("Weather service shutdown failed", "error", err)
	}

	db.Pool.Close()
	rdb.Close()

	slog.Info("Server exited gracefully")
}
