package main

import (
	"context"
	"embed"
	_ "embed"
	"fmt"
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
	"github.com/7apri/SimpleGOWebserver/internal/location"
	"github.com/7apri/SimpleGOWebserver/internal/redis"
	"github.com/7apri/SimpleGOWebserver/internal/server"
	"github.com/7apri/SimpleGOWebserver/internal/weather"
	util "github.com/7apri/SimpleGOWebserver/pkg"
)

//go:embed all:site
var siteEmbed embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	weatherApiKey := util.TryGetEnvFatal("WEATHER_API_KEY")
	accessSecret := util.TryGetEnvFatal("ACCESS_SECRET_AUTH")

	db := database.Init(ctx)
	rdb := redis.Init(ctx)

	owClient := exApi.NewOwClient(weatherApiKey, 0) //time.Second

	ls, err := location.NewService(ctx, db, 500, owClient, exApi.NewIpClient(time.Minute/40))
	if err != nil {
		slog.Error("There was an error creating the location service", "error", err)
		os.Exit(1)
	}
	ws, err := weather.NewService(ctx, db, 500, owClient, ls)
	if err != nil {
		slog.Error("There was an error creating the weather service", "error", err)
		os.Exit(1)
	}

	au := auth.NewAuthHandler(db, accessSecret)
	as := analytics.NewService(ctx, db, rdb)

	srv := server.NewServer(ls, ws, au, db, siteEmbed, rdb, as)

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
