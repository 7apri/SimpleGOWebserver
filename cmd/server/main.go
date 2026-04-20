package main

import (
	"context"
	_ "embed"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/7apri/SimpleGOWebserver/internal/analytics"
	"github.com/7apri/SimpleGOWebserver/internal/auth"
	"github.com/7apri/SimpleGOWebserver/internal/consts"
	"github.com/7apri/SimpleGOWebserver/internal/database"
	"github.com/7apri/SimpleGOWebserver/internal/email"
	exApi "github.com/7apri/SimpleGOWebserver/internal/ex-api"
	"github.com/7apri/SimpleGOWebserver/internal/i18n"
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/redis"
	"github.com/7apri/SimpleGOWebserver/internal/server"
	"github.com/7apri/SimpleGOWebserver/internal/templates"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
)

// //go:embed all:site/templates
// var templatesRaw embed.FS

// //go:embed all:site/i18n
// var i18nRaw embed.FS

var (
// templatesEmbed, _ = fs.Sub(templatesRaw, "site/templates")
// i18nEmbed, _      = fs.ReadDir(i18nRaw, "site/i18n")
)

func tryGetEnvFatal(k string) string {
	v := os.Getenv(k)
	if v == "" {
		slog.Error("please check the .env", "missing key", k)
		os.Exit(1)
	}
	return v
}
func tryGetEnvDefault(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseUser := tryGetEnvFatal("DB_USER")
	databasepassword := tryGetEnvFatal("DB_PASSWORD")
	databaseName := tryGetEnvFatal("DB_NAME")
	db := database.Init(ctx, databaseUser, databasepassword, databaseName)

	redisAddr := tryGetEnvFatal("REDIS_ADDRESS")
	redisPassword := tryGetEnvFatal("REDIS_PASSWORD")
	rdb := redis.Init(ctx, redisAddr, redisPassword)

	as := analytics.NewService(db, rdb)

	slog.SetDefault(slog.New(as.LogHandler))

	weatherApiKey := tryGetEnvFatal("WEATHER_API_KEY")
	accessSecret := tryGetEnvFatal("ACCESS_SECRET_AUTH")
	twoFactorSecret := tryGetEnvFatal("TWO_FACTOR_SECRET_AUTH")
	mFAPepper := tryGetEnvFatal("TWO_FACTOR_PEPPER_AUTH")
	providerSecret := tryGetEnvFatal("PROVIDER_SECRET_AUTH")
	challengeSecret := tryGetEnvFatal("CHALLENGE_SECRET_AUTH")

	githubClientID := tryGetEnvFatal("GITHUB_CLIENT_ID_AUTH")
	githubRedirectUrl := tryGetEnvFatal("GITHUB_REDIRECT_URL_AUTH")
	githubClientSecret := tryGetEnvFatal("GITHUB_CLIENT_SECRET_AUTH")

	googleClientID := tryGetEnvFatal("GOOGLE_CLIENT_ID_AUTH")
	googleRedirectUrl := tryGetEnvFatal("GOOGLE_REDIRECT_URL_AUTH")
	googleClientSecret := tryGetEnvFatal("GOOGLE_CLIENT_SECRET_AUTH")

	i18nPath := tryGetEnvDefault("I18N_PATH", "./i18n")
	tmplPath := tryGetEnvDefault("TMPL_PATH", "./templates")
	statPath := tryGetEnvDefault("STAT_PATH", "./static")

	smtpUser := tryGetEnvDefault("SMTP_USER", "")
	smtpPass := tryGetEnvDefault("SMTP_PASSWORD", "")

	i18nFS := os.DirFS(i18nPath)
	tmplFS := os.DirFS(tmplPath)
	statFS := os.DirFS(statPath)

	i18nMgr, err := i18n.NewManager(i18nFS)
	if err != nil {
		log.Fatalf("Error creating the i18n manager: %s", err)
	}

	tmplMgr, err := templates.NewManager(tmplFS, statFS, statPath, i18nMgr)
	if err != nil {
		log.Fatalf("Error creating the tmpl manager: %s", err)
	}

	owClient := exApi.NewOwClient(weatherApiKey, 0) //time.Second

	ls, err := location.NewService(db,
		consts.LocationColdCacheSize,
		consts.LocationPromoteThreshold,
		consts.LocationPromoteBufferSize,
		consts.LocationJanitorInterval,
		consts.LocationSaveChanBufferSize,
		owClient,
		exApi.NewIpClient(time.Minute/40),
	)
	if err != nil {
		slog.Error("There was an error creating the location service", "error", err)
		os.Exit(1)
	}
	ws, err := weather.NewService(
		db,
		consts.LocationColdCacheSize,
		consts.WeatherPromoteThreshold,
		consts.WeatherPromoteBufferSize,
		consts.WeatherJanitorInterval,
		consts.WeatherSaveChanBufferSize,
		i18nMgr,
		owClient,
		ls,
	)
	if err != nil {
		slog.Error("There was an error creating the weather service", "error", err)
		os.Exit(1)
	}

	em := email.NewEmailManager(smtpPass, smtpUser, tmplMgr)
	ah, err := auth.NewAuthHandler(db, rdb, em, i18nMgr, accessSecret, twoFactorSecret, providerSecret, challengeSecret, mFAPepper)
	if err != nil {
		slog.Error("There was an error creating the auth handeler", "error", err)
		os.Exit(1)
	}

	githubProv := auth.NewGithubOAuth(githubClientID, githubClientSecret, githubRedirectUrl)
	googleProv := auth.NewGoogleOAuth(googleClientID, googleClientSecret, googleRedirectUrl)

	ah.RegisterProviders(githubProv, googleProv)

	srv := server.NewServer(ls, ws, ah, db, rdb, i18nMgr, tmplMgr, as)

	httpSrv := &http.Server{
		Addr:         ":8080",
		Handler:      srv.Routes(),
		IdleTimeout:  10 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
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

	if err := httpSrv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}
	srv.Down(shutdownCtx)

	db.Pool.Close()
	rdb.Close()

	slog.Info("Server exited gracefully")
}
